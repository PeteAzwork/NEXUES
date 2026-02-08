package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/org/nexus/internal/models"
)

// MockStore implements SnapshotStore for testing.
type MockStore struct {
	mu        sync.RWMutex
	snapshots map[string]*models.Snapshot
	healthy   bool
}

// NewMockStore creates a MockStore with health checks enabled.
func NewMockStore() *MockStore {
	return &MockStore{
		snapshots: make(map[string]*models.Snapshot),
		healthy:   true,
	}
}

func (m *MockStore) Upsert(_ context.Context, snapshot *models.Snapshot) (*models.UpsertResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.snapshots[snapshot.Hash]
	if ok {
		existing.Metadata.OccurrenceCount++
		return &models.UpsertResult{
			IsNew:           false,
			OccurrenceCount: existing.Metadata.OccurrenceCount,
		}, nil
	}

	snapshot.Metadata.OccurrenceCount = 1
	m.snapshots[snapshot.Hash] = snapshot
	return &models.UpsertResult{
		IsNew:           true,
		OccurrenceCount: 1,
	}, nil
}

func (m *MockStore) FindByHash(_ context.Context, hash string) (*models.Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.snapshots[hash]
	if !ok {
		return nil, ErrNotFound
	}
	return s, nil
}

func (m *MockStore) Find(_ context.Context, query models.SnapshotQuery) ([]models.Snapshot, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []models.Snapshot
	for _, s := range m.snapshots {
		if query.Path != "" && s.Request.Path != query.Path {
			continue
		}
		if query.StatusCode > 0 && s.Response.StatusCode != query.StatusCode {
			continue
		}
		if query.Method != "" && s.Request.Method != query.Method {
			continue
		}
		results = append(results, *s)
	}

	total := int64(len(results))

	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	if offset >= int64(len(results)) {
		return []models.Snapshot{}, total, nil
	}

	end := offset + limit
	if end > int64(len(results)) {
		end = int64(len(results))
	}

	return results[offset:end], total, nil
}

func (m *MockStore) FindByHashPrefix(_ context.Context, prefix string) ([]models.Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(prefix) < 6 {
		return nil, fmt.Errorf("hash prefix must be at least 6 characters, got %d", len(prefix))
	}

	var results []models.Snapshot
	for hash, s := range m.snapshots {
		if len(hash) >= len(prefix) && hash[:len(prefix)] == prefix {
			results = append(results, *s)
		}
	}
	if results == nil {
		results = []models.Snapshot{}
	}
	return results, nil
}

func (m *MockStore) FindByAttributes(_ context.Context, query models.AttributeQuery) ([]models.Snapshot, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []models.Snapshot
	for _, s := range m.snapshots {
		if matchesAttributes(s, query.Conditions) {
			results = append(results, *s)
		}
	}

	total := int64(len(results))
	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= int64(len(results)) {
		return []models.Snapshot{}, total, nil
	}
	end := offset + limit
	if end > int64(len(results)) {
		end = int64(len(results))
	}
	return results[offset:end], total, nil
}

func matchesAttributes(s *models.Snapshot, conditions []models.AttributeCondition) bool {
	for _, cond := range conditions {
		var attrs map[string]interface{}
		switch cond.Category {
		case "subject":
			attrs = s.AttributeState.Subject
		case "resource":
			attrs = s.AttributeState.Resource
		case "environment":
			attrs = s.AttributeState.Environment
		case "context":
			attrs = s.AttributeState.Context
		default:
			return false
		}
		if attrs == nil {
			return false
		}
		v, ok := attrs[cond.Key]
		if !ok || fmt.Sprintf("%v", v) != fmt.Sprintf("%v", cond.Value) {
			return false
		}
	}
	return true
}

func (m *MockStore) GetStats(_ context.Context, query models.StatsQuery) (*models.SnapshotStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &models.SnapshotStats{
		ByMethod:      make(map[string]int64),
		ByStatusRange: make(map[string]int64),
		TopEndpoints:  []models.EndpointStat{},
	}

	endpointCounts := make(map[string]int64)
	endpointUnique := make(map[string]int64)

	for _, s := range m.snapshots {
		if query.From != nil && s.Metadata.FirstSeen.Before(*query.From) {
			continue
		}
		if query.To != nil && s.Metadata.FirstSeen.After(*query.To) {
			continue
		}

		stats.TotalCount++
		stats.ByMethod[s.Request.Method]++

		sc := s.Response.StatusCode
		switch {
		case sc >= 200 && sc < 300:
			stats.ByStatusRange["2xx"]++
		case sc >= 300 && sc < 400:
			stats.ByStatusRange["3xx"]++
		case sc >= 400 && sc < 500:
			stats.ByStatusRange["4xx"]++
		case sc >= 500 && sc < 600:
			stats.ByStatusRange["5xx"]++
		}

		endpointCounts[s.Request.Path] += s.Metadata.OccurrenceCount
		endpointUnique[s.Request.Path]++

		if stats.TimeRange.Earliest.IsZero() || s.Metadata.FirstSeen.Before(stats.TimeRange.Earliest) {
			stats.TimeRange.Earliest = s.Metadata.FirstSeen
		}
		if s.Metadata.LastSeen.After(stats.TimeRange.Latest) {
			stats.TimeRange.Latest = s.Metadata.LastSeen
		}
	}

	topN := query.TopN
	if topN <= 0 {
		topN = 10
	}
	for path, count := range endpointCounts {
		stats.TopEndpoints = append(stats.TopEndpoints, models.EndpointStat{
			Path:             path,
			TotalOccurrences: count,
			UniqueSnapshots:  endpointUnique[path],
		})
	}
	for i := 0; i < len(stats.TopEndpoints); i++ {
		for j := i + 1; j < len(stats.TopEndpoints); j++ {
			if stats.TopEndpoints[j].TotalOccurrences > stats.TopEndpoints[i].TotalOccurrences {
				stats.TopEndpoints[i], stats.TopEndpoints[j] = stats.TopEndpoints[j], stats.TopEndpoints[i]
			}
		}
	}
	if len(stats.TopEndpoints) > topN {
		stats.TopEndpoints = stats.TopEndpoints[:topN]
	}

	return stats, nil
}

func (m *MockStore) GetDistinctAttributeKeys(_ context.Context) (*models.AttributeIndex, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subjectSet := make(map[string]bool)
	resourceSet := make(map[string]bool)
	envSet := make(map[string]bool)
	ctxSet := make(map[string]bool)

	for _, s := range m.snapshots {
		for k := range s.AttributeState.Subject {
			subjectSet[k] = true
		}
		for k := range s.AttributeState.Resource {
			resourceSet[k] = true
		}
		for k := range s.AttributeState.Environment {
			envSet[k] = true
		}
		for k := range s.AttributeState.Context {
			ctxSet[k] = true
		}
	}

	toSlice := func(m map[string]bool) []string {
		s := make([]string, 0, len(m))
		for k := range m {
			s = append(s, k)
		}
		return s
	}

	return &models.AttributeIndex{
		SubjectKeys:     toSlice(subjectSet),
		ResourceKeys:    toSlice(resourceSet),
		EnvironmentKeys: toSlice(envSet),
		ContextKeys:     toSlice(ctxSet),
	}, nil
}

func (m *MockStore) HealthCheck(_ context.Context) error {
	if !m.healthy {
		return ErrNotFound
	}
	return nil
}

func (m *MockStore) Close(_ context.Context) error {
	return nil
}

// SetHealthy sets the mock health status.
func (m *MockStore) SetHealthy(healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthy = healthy
}

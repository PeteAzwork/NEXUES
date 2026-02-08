package store

import (
	"context"
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

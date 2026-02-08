package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/org/nexus/internal/models"
)

func seedMockStore(t *testing.T) *MockStore {
	t.Helper()
	ms := NewMockStore()
	now := time.Now().UTC()

	snapshots := []*models.Snapshot{
		{
			Hash: "aabbcc112233445566778899aabbcc112233445566778899aabbcc1122334455",
			Request: models.Request{
				Method: "GET",
				Path:   "/api/users",
			},
			Response: models.Response{StatusCode: 200},
			AttributeState: models.AttributeState{
				Subject:     map[string]interface{}{"role": "admin"},
				Resource:    map[string]interface{}{"type": "user"},
				Environment: map[string]interface{}{"region": "us-east-1"},
				Context:     map[string]interface{}{"action": "list"},
			},
			Metadata: models.Metadata{
				FirstSeen:       now.Add(-24 * time.Hour),
				LastSeen:        now,
				OccurrenceCount: 10,
			},
		},
		{
			Hash: "aabbcc998877665544332211aabbcc998877665544332211aabbcc9988776655",
			Request: models.Request{
				Method: "POST",
				Path:   "/api/users",
			},
			Response: models.Response{StatusCode: 201},
			AttributeState: models.AttributeState{
				Subject:     map[string]interface{}{"role": "editor"},
				Resource:    map[string]interface{}{"type": "user"},
				Environment: map[string]interface{}{"region": "eu-west-1"},
				Context:     map[string]interface{}{"action": "create"},
			},
			Metadata: models.Metadata{
				FirstSeen:       now.Add(-12 * time.Hour),
				LastSeen:        now.Add(-1 * time.Hour),
				OccurrenceCount: 3,
			},
		},
		{
			Hash: "ddeeff112233445566778899ddeeff112233445566778899ddeeff1122334455",
			Request: models.Request{
				Method: "GET",
				Path:   "/api/orders",
			},
			Response: models.Response{StatusCode: 500},
			AttributeState: models.AttributeState{
				Subject:     map[string]interface{}{"role": "admin"},
				Resource:    map[string]interface{}{"type": "order"},
				Environment: map[string]interface{}{"region": "us-east-1"},
				Context:     map[string]interface{}{"action": "list"},
			},
			Metadata: models.Metadata{
				FirstSeen:       now.Add(-6 * time.Hour),
				LastSeen:        now.Add(-30 * time.Minute),
				OccurrenceCount: 1,
			},
		},
	}

	ctx := context.Background()
	for _, s := range snapshots {
		_, err := ms.Upsert(ctx, s)
		require.NoError(t, err)
	}

	return ms
}

func TestFindByHashPrefix_UniqueMatch(t *testing.T) {
	ms := seedMockStore(t)
	ctx := context.Background()

	results, err := ms.FindByHashPrefix(ctx, "ddeeff")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "/api/orders", results[0].Request.Path)
}

func TestFindByHashPrefix_MultipleMatches(t *testing.T) {
	ms := seedMockStore(t)
	ctx := context.Background()

	results, err := ms.FindByHashPrefix(ctx, "aabbcc")
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestFindByHashPrefix_NoMatch(t *testing.T) {
	ms := seedMockStore(t)
	ctx := context.Background()

	results, err := ms.FindByHashPrefix(ctx, "ffffff")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestFindByHashPrefix_TooShort(t *testing.T) {
	ms := seedMockStore(t)
	ctx := context.Background()

	_, err := ms.FindByHashPrefix(ctx, "aabb")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least 6 characters")
}

func TestFindByAttributes_SingleCondition(t *testing.T) {
	ms := seedMockStore(t)
	ctx := context.Background()

	results, total, err := ms.FindByAttributes(ctx, models.AttributeQuery{
		Conditions: []models.AttributeCondition{
			{Category: "subject", Key: "role", Value: "admin"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, results, 2)
}

func TestFindByAttributes_MultipleConditions(t *testing.T) {
	ms := seedMockStore(t)
	ctx := context.Background()

	results, total, err := ms.FindByAttributes(ctx, models.AttributeQuery{
		Conditions: []models.AttributeCondition{
			{Category: "subject", Key: "role", Value: "admin"},
			{Category: "resource", Key: "type", Value: "order"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, results, 1)
	assert.Equal(t, "/api/orders", results[0].Request.Path)
}

func TestFindByAttributes_NoResults(t *testing.T) {
	ms := seedMockStore(t)
	ctx := context.Background()

	results, total, err := ms.FindByAttributes(ctx, models.AttributeQuery{
		Conditions: []models.AttributeCondition{
			{Category: "subject", Key: "role", Value: "nonexistent"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, results)
}

func TestGetStats_All(t *testing.T) {
	ms := seedMockStore(t)
	ctx := context.Background()

	stats, err := ms.GetStats(ctx, models.StatsQuery{})
	require.NoError(t, err)

	assert.Equal(t, int64(3), stats.TotalCount)
	assert.Equal(t, int64(2), stats.ByMethod["GET"])
	assert.Equal(t, int64(1), stats.ByMethod["POST"])
	assert.Equal(t, int64(2), stats.ByStatusRange["2xx"])
	assert.Equal(t, int64(1), stats.ByStatusRange["5xx"])
	assert.NotEmpty(t, stats.TopEndpoints)
}

func TestGetStats_TopN(t *testing.T) {
	ms := seedMockStore(t)
	ctx := context.Background()

	stats, err := ms.GetStats(ctx, models.StatsQuery{TopN: 1})
	require.NoError(t, err)

	assert.Len(t, stats.TopEndpoints, 1)
}

func TestGetStats_TimeRange(t *testing.T) {
	ms := seedMockStore(t)
	ctx := context.Background()

	stats, err := ms.GetStats(ctx, models.StatsQuery{})
	require.NoError(t, err)

	assert.False(t, stats.TimeRange.Earliest.IsZero())
	assert.False(t, stats.TimeRange.Latest.IsZero())
	assert.True(t, stats.TimeRange.Earliest.Before(stats.TimeRange.Latest) || stats.TimeRange.Earliest.Equal(stats.TimeRange.Latest))
}

func TestGetStats_Empty(t *testing.T) {
	ms := NewMockStore()
	ctx := context.Background()

	stats, err := ms.GetStats(ctx, models.StatsQuery{})
	require.NoError(t, err)

	assert.Equal(t, int64(0), stats.TotalCount)
	assert.Empty(t, stats.ByMethod)
	assert.Empty(t, stats.TopEndpoints)
}

func TestGetDistinctAttributeKeys(t *testing.T) {
	ms := seedMockStore(t)
	ctx := context.Background()

	index, err := ms.GetDistinctAttributeKeys(ctx)
	require.NoError(t, err)

	assert.Contains(t, index.SubjectKeys, "role")
	assert.Contains(t, index.ResourceKeys, "type")
	assert.Contains(t, index.EnvironmentKeys, "region")
	assert.Contains(t, index.ContextKeys, "action")
}

func TestGetDistinctAttributeKeys_Empty(t *testing.T) {
	ms := NewMockStore()
	ctx := context.Background()

	index, err := ms.GetDistinctAttributeKeys(ctx)
	require.NoError(t, err)

	assert.Empty(t, index.SubjectKeys)
	assert.Empty(t, index.ResourceKeys)
	assert.Empty(t, index.EnvironmentKeys)
	assert.Empty(t, index.ContextKeys)
}

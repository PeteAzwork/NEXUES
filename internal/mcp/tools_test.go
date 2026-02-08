package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/org/nexus/internal/models"
	"github.com/org/nexus/internal/store"
)

func newTestServer(t *testing.T) (*Server, *store.MockStore) {
	t.Helper()
	ms := store.NewMockStore()
	logger := zap.NewNop()
	srv := New("test-nexus-mcp", "0.0.1", ms, logger)
	return srv, ms
}

func seedSnapshots(t *testing.T, ms *store.MockStore) {
	t.Helper()
	now := time.Now().UTC()

	snapshots := []*models.Snapshot{
		{
			Hash: "aabbccdd11223344556677889900aabbccddeeff11223344556677889900aabb",
			Request: models.Request{
				Method:    "GET",
				Path:      "/api/users",
				Headers:   map[string]string{"Accept": "application/json"},
				Timestamp: now,
			},
			Response: models.Response{
				StatusCode: 200,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       map[string]interface{}{"users": []interface{}{}},
				Timestamp:  now,
			},
			AttributeState: models.AttributeState{
				Subject:     map[string]interface{}{"role": "admin", "userId": "u1"},
				Resource:    map[string]interface{}{"type": "user"},
				Environment: map[string]interface{}{"region": "us-east-1"},
				Context:     map[string]interface{}{"action": "list"},
			},
			Metadata: models.Metadata{
				FirstSeen:       now.Add(-24 * time.Hour),
				LastSeen:        now,
				OccurrenceCount: 5,
			},
		},
		{
			Hash: "bbccddee22334455667788990011bbccddeeff22334455667788990011bbcc",
			Request: models.Request{
				Method:    "POST",
				Path:      "/api/users",
				Headers:   map[string]string{"Content-Type": "application/json"},
				Body:      map[string]interface{}{"name": "Alice"},
				Timestamp: now,
			},
			Response: models.Response{
				StatusCode: 201,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       map[string]interface{}{"id": "u2", "name": "Alice"},
				Timestamp:  now,
			},
			AttributeState: models.AttributeState{
				Subject:     map[string]interface{}{"role": "admin", "userId": "u1"},
				Resource:    map[string]interface{}{"type": "user"},
				Environment: map[string]interface{}{"region": "us-east-1"},
				Context:     map[string]interface{}{"action": "create"},
			},
			Metadata: models.Metadata{
				FirstSeen:       now.Add(-12 * time.Hour),
				LastSeen:        now.Add(-1 * time.Hour),
				OccurrenceCount: 2,
			},
		},
		{
			Hash: "ccddeeff33445566778899001122ccddeeff33445566778899001122ccddee",
			Request: models.Request{
				Method:    "GET",
				Path:      "/api/orders",
				Timestamp: now,
			},
			Response: models.Response{
				StatusCode: 500,
				Body:       map[string]interface{}{"error": "internal server error"},
				Timestamp:  now,
			},
			AttributeState: models.AttributeState{
				Subject:     map[string]interface{}{"role": "viewer", "userId": "u3"},
				Resource:    map[string]interface{}{"type": "order"},
				Environment: map[string]interface{}{"region": "eu-west-1"},
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
}

func callTool(t *testing.T, srv *Server, name string, args map[string]interface{}) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args

	tools := srv.mcp.ListTools()
	tool, ok := tools[name]
	require.True(t, ok, "tool %s not registered", name)

	result, err := tool.Handler(context.Background(), req)
	require.NoError(t, err)
	return result
}

func getResultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, result.Content, "expected at least one content block")
	tc, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected TextContent")
	return tc.Text
}

// --- query_snapshots tests ---

func TestQuerySnapshots_NoFilters(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "query_snapshots", map[string]interface{}{})
	text := getResultText(t, result)

	assert.False(t, result.IsError)
	assert.Contains(t, text, "Found 3 total snapshots")
	assert.Contains(t, text, "/api/users")
	assert.Contains(t, text, "/api/orders")
}

func TestQuerySnapshots_ByMethod(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "query_snapshots", map[string]interface{}{
		"method": "GET",
	})
	text := getResultText(t, result)

	assert.False(t, result.IsError)
	assert.Contains(t, text, "Found 2 total snapshots")
}

func TestQuerySnapshots_ByPath(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "query_snapshots", map[string]interface{}{
		"path": "/api/orders",
	})
	text := getResultText(t, result)

	assert.False(t, result.IsError)
	assert.Contains(t, text, "Found 1 total snapshots")
	assert.Contains(t, text, "/api/orders")
}

func TestQuerySnapshots_ByStatusCode(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "query_snapshots", map[string]interface{}{
		"statusCode": float64(500),
	})
	text := getResultText(t, result)

	assert.False(t, result.IsError)
	assert.Contains(t, text, "Found 1 total snapshots")
	assert.Contains(t, text, "/api/orders")
}

func TestQuerySnapshots_MinStatus(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "query_snapshots", map[string]interface{}{
		"minStatus": float64(400),
	})
	text := getResultText(t, result)

	assert.False(t, result.IsError)
	assert.Contains(t, text, "500")
}

func TestQuerySnapshots_NoResults(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "query_snapshots", map[string]interface{}{
		"path": "/nonexistent",
	})
	text := getResultText(t, result)

	assert.False(t, result.IsError)
	assert.Contains(t, text, "No snapshots found")
}

func TestQuerySnapshots_Pagination(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "query_snapshots", map[string]interface{}{
		"limit":  float64(1),
		"offset": float64(0),
	})
	text := getResultText(t, result)

	assert.False(t, result.IsError)
	assert.Contains(t, text, "showing 1-1")
	assert.Contains(t, text, "more results available")
}

// --- get_snapshot tests ---

func TestGetSnapshot_FullHash(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "get_snapshot", map[string]interface{}{
		"hash": "aabbccdd11223344556677889900aabbccddeeff11223344556677889900aabb",
	})
	text := getResultText(t, result)

	assert.False(t, result.IsError)
	assert.Contains(t, text, "GET")
	assert.Contains(t, text, "/api/users")
	assert.Contains(t, text, "200")
}

func TestGetSnapshot_PrefixMatch(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "get_snapshot", map[string]interface{}{
		"hash": "aabbccdd1122",
	})
	text := getResultText(t, result)

	assert.False(t, result.IsError)
	assert.Contains(t, text, "/api/users")
}

func TestGetSnapshot_PrefixTooShort(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "get_snapshot", map[string]interface{}{
		"hash": "aabb",
	})

	assert.True(t, result.IsError)
	text := getResultText(t, result)
	assert.Contains(t, text, "at least 6 characters")
}

func TestGetSnapshot_NotFound(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "get_snapshot", map[string]interface{}{
		"hash": "0000000000000000000000000000000000000000000000000000000000000000",
	})

	assert.True(t, result.IsError)
	text := getResultText(t, result)
	assert.Contains(t, text, "No snapshot found")
}

func TestGetSnapshot_RequestOnly(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "get_snapshot", map[string]interface{}{
		"hash":   "aabbccdd11223344556677889900aabbccddeeff11223344556677889900aabb",
		"format": "request_only",
	})
	text := getResultText(t, result)

	assert.False(t, result.IsError)
	assert.Contains(t, text, "Request")
	assert.Contains(t, text, "GET")
	assert.NotContains(t, text, "Metadata")
}

func TestGetSnapshot_MissingHash(t *testing.T) {
	srv, _ := newTestServer(t)

	result := callTool(t, srv, "get_snapshot", map[string]interface{}{})

	assert.True(t, result.IsError)
	text := getResultText(t, result)
	assert.Contains(t, text, "required")
}

// --- search_by_attributes tests ---

func TestSearchByAttributes_SubjectRole(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "search_by_attributes", map[string]interface{}{
		"subjectKey":   "role",
		"subjectValue": "admin",
	})
	text := getResultText(t, result)

	assert.False(t, result.IsError)
	assert.Contains(t, text, "Found 2 snapshots")
}

func TestSearchByAttributes_EnvironmentRegion(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "search_by_attributes", map[string]interface{}{
		"envKey":   "region",
		"envValue": "eu-west-1",
	})
	text := getResultText(t, result)

	assert.False(t, result.IsError)
	assert.Contains(t, text, "Found 1 snapshots")
	assert.Contains(t, text, "/api/orders")
}

func TestSearchByAttributes_NoConditions(t *testing.T) {
	srv, _ := newTestServer(t)

	result := callTool(t, srv, "search_by_attributes", map[string]interface{}{})

	assert.True(t, result.IsError)
	text := getResultText(t, result)
	assert.Contains(t, text, "At least one attribute key")
}

func TestSearchByAttributes_NoResults(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "search_by_attributes", map[string]interface{}{
		"subjectKey":   "role",
		"subjectValue": "nonexistent",
	})
	text := getResultText(t, result)

	assert.False(t, result.IsError)
	assert.Contains(t, text, "No snapshots found")
}

// --- get_snapshot_stats tests ---

func TestGetSnapshotStats_All(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "get_snapshot_stats", map[string]interface{}{})
	text := getResultText(t, result)

	assert.False(t, result.IsError)
	assert.Contains(t, text, "Total Snapshots: 3")
	assert.Contains(t, text, "GET")
	assert.Contains(t, text, "POST")
	assert.Contains(t, text, "2xx")
	assert.Contains(t, text, "5xx")
	assert.Contains(t, text, "Top Endpoints")
}

func TestGetSnapshotStats_Empty(t *testing.T) {
	srv, _ := newTestServer(t)

	result := callTool(t, srv, "get_snapshot_stats", map[string]interface{}{})
	text := getResultText(t, result)

	assert.False(t, result.IsError)
	assert.Contains(t, text, "Total Snapshots: 0")
}

func TestGetSnapshotStats_InvalidTimeFormat(t *testing.T) {
	srv, _ := newTestServer(t)

	result := callTool(t, srv, "get_snapshot_stats", map[string]interface{}{
		"from": "not-a-date",
	})

	assert.True(t, result.IsError)
	text := getResultText(t, result)
	assert.Contains(t, text, "Invalid")
}

// --- compare_snapshots tests ---

func TestCompareSnapshots_Different(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "compare_snapshots", map[string]interface{}{
		"hash1": "aabbccdd11223344556677889900aabbccddeeff11223344556677889900aabb",
		"hash2": "bbccddee22334455667788990011bbccddeeff22334455667788990011bbcc",
	})
	text := getResultText(t, result)

	assert.False(t, result.IsError)
	assert.Contains(t, text, "Diff")
	assert.Contains(t, text, "Request")
	assert.Contains(t, text, "Response")
}

func TestCompareSnapshots_NotFound(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "compare_snapshots", map[string]interface{}{
		"hash1": "0000000000000000000000000000000000000000000000000000000000000000",
		"hash2": "aabbccdd11223344556677889900aabbccddeeff11223344556677889900aabb",
	})

	assert.True(t, result.IsError)
	text := getResultText(t, result)
	assert.Contains(t, text, "not found")
}

func TestCompareSnapshots_SectionsFilter(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	result := callTool(t, srv, "compare_snapshots", map[string]interface{}{
		"hash1":    "aabbccdd11223344556677889900aabbccddeeff11223344556677889900aabb",
		"hash2":    "bbccddee22334455667788990011bbccddeeff22334455667788990011bbcc",
		"sections": "request",
	})
	text := getResultText(t, result)

	assert.False(t, result.IsError)
	assert.Contains(t, text, "Request")
	assert.NotContains(t, text, "--- Metadata ---")
}

func TestCompareSnapshots_MissingHash(t *testing.T) {
	srv, _ := newTestServer(t)

	result := callTool(t, srv, "compare_snapshots", map[string]interface{}{
		"hash1": "aabbccdd11223344556677889900aabbccddeeff11223344556677889900aabb",
	})

	assert.True(t, result.IsError)
	text := getResultText(t, result)
	assert.Contains(t, text, "required")
}

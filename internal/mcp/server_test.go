package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/org/nexus/internal/store"
)

func TestServerCreation(t *testing.T) {
	ms := store.NewMockStore()
	logger := zap.NewNop()
	srv := New("test-server", "1.0.0", ms, logger)

	require.NotNil(t, srv)
	require.NotNil(t, srv.MCPServer())
}

func TestToolsRegistered(t *testing.T) {
	ms := store.NewMockStore()
	logger := zap.NewNop()
	srv := New("test-server", "1.0.0", ms, logger)

	tools := srv.MCPServer().ListTools()

	expectedTools := []string{
		"query_snapshots",
		"get_snapshot",
		"search_by_attributes",
		"get_snapshot_stats",
		"compare_snapshots",
	}

	for _, name := range expectedTools {
		_, ok := tools[name]
		assert.True(t, ok, "tool %s should be registered", name)
	}

	assert.Len(t, tools, len(expectedTools))
}

// --- Resource tests ---

func TestSnapshotsResource(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "nexus://snapshots"

	contents, err := srv.handleSnapshotsResource(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, contents, 1)

	tc, ok := contents[0].(mcp.TextResourceContents)
	require.True(t, ok)
	assert.Equal(t, "nexus://snapshots", tc.URI)
	assert.Equal(t, "application/json", tc.MIMEType)

	var result map[string]interface{}
	err = json.Unmarshal([]byte(tc.Text), &result)
	require.NoError(t, err)
	assert.Equal(t, float64(3), result["total"])

	snapshots, ok := result["snapshots"].([]interface{})
	require.True(t, ok)
	assert.Len(t, snapshots, 3)
}

func TestSnapshotsResource_Empty(t *testing.T) {
	srv, _ := newTestServer(t)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "nexus://snapshots"

	contents, err := srv.handleSnapshotsResource(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, contents, 1)

	tc := contents[0].(mcp.TextResourceContents)
	var result map[string]interface{}
	err = json.Unmarshal([]byte(tc.Text), &result)
	require.NoError(t, err)
	assert.Equal(t, float64(0), result["total"])
}

func TestSnapshotDetailResource(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "nexus://snapshots/aabbccdd11223344556677889900aabbccddeeff11223344556677889900aabb"

	contents, err := srv.handleSnapshotDetailResource(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, contents, 1)

	tc := contents[0].(mcp.TextResourceContents)
	assert.Equal(t, "application/json", tc.MIMEType)

	var snap map[string]interface{}
	err = json.Unmarshal([]byte(tc.Text), &snap)
	require.NoError(t, err)
	assert.Equal(t, "aabbccdd11223344556677889900aabbccddeeff11223344556677889900aabb", snap["hash"])
}

func TestSnapshotDetailResource_NotFound(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "nexus://snapshots/0000000000000000000000000000000000000000000000000000000000000000"

	_, err := srv.handleSnapshotDetailResource(context.Background(), req)
	assert.Error(t, err)
}

func TestStatsResource(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "nexus://stats"

	contents, err := srv.handleStatsResource(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, contents, 1)

	tc := contents[0].(mcp.TextResourceContents)
	assert.Equal(t, "application/json", tc.MIMEType)

	var stats map[string]interface{}
	err = json.Unmarshal([]byte(tc.Text), &stats)
	require.NoError(t, err)
	assert.Equal(t, float64(3), stats["totalCount"])
}

func TestAttributesResource(t *testing.T) {
	srv, ms := newTestServer(t)
	seedSnapshots(t, ms)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "nexus://attributes"

	contents, err := srv.handleAttributesResource(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, contents, 1)

	tc := contents[0].(mcp.TextResourceContents)
	assert.Equal(t, "application/json", tc.MIMEType)

	var index map[string]interface{}
	err = json.Unmarshal([]byte(tc.Text), &index)
	require.NoError(t, err)

	subjectKeys, ok := index["subjectKeys"].([]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(subjectKeys), 1)
}

package mcp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/org/nexus/internal/models"
)

func testSnapshot() *models.Snapshot {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	return &models.Snapshot{
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
			Body:       map[string]interface{}{"count": float64(0)},
			Timestamp:  now,
		},
		AttributeState: models.AttributeState{
			Subject:     map[string]interface{}{"role": "admin"},
			Resource:    map[string]interface{}{"type": "user"},
			Environment: map[string]interface{}{"region": "us-east-1"},
			Context:     map[string]interface{}{"action": "list"},
		},
		Metadata: models.Metadata{
			FirstSeen:       now.Add(-24 * time.Hour),
			LastSeen:        now,
			OccurrenceCount: 5,
		},
	}
}

func TestSnapshotSummary(t *testing.T) {
	s := testSnapshot()
	summary := snapshotSummary(*s)

	assert.Contains(t, summary, "[aabbccdd]")
	assert.Contains(t, summary, "GET")
	assert.Contains(t, summary, "/api/users")
	assert.Contains(t, summary, "200")
	assert.Contains(t, summary, "seen: 5 times")
}

func TestSnapshotToText_Full(t *testing.T) {
	s := testSnapshot()
	text := snapshotToText(s, "full")

	assert.Contains(t, text, "=== Snapshot")
	assert.Contains(t, text, "--- Request ---")
	assert.Contains(t, text, "--- Response ---")
	assert.Contains(t, text, "--- Attributes ---")
	assert.Contains(t, text, "--- Metadata ---")
	assert.Contains(t, text, "GET")
	assert.Contains(t, text, "/api/users")
	assert.Contains(t, text, "200")
	assert.Contains(t, text, "admin")
}

func TestSnapshotToText_RequestOnly(t *testing.T) {
	s := testSnapshot()
	text := snapshotToText(s, "request_only")

	assert.Contains(t, text, "--- Request ---")
	assert.Contains(t, text, "GET")
	assert.NotContains(t, text, "--- Response ---")
	assert.NotContains(t, text, "--- Metadata ---")
}

func TestSnapshotToText_ResponseOnly(t *testing.T) {
	s := testSnapshot()
	text := snapshotToText(s, "response_only")

	assert.Contains(t, text, "--- Response ---")
	assert.Contains(t, text, "200")
	assert.NotContains(t, text, "--- Request ---")
}

func TestSnapshotToText_MetadataOnly(t *testing.T) {
	s := testSnapshot()
	text := snapshotToText(s, "metadata_only")

	assert.Contains(t, text, "--- Metadata ---")
	assert.Contains(t, text, "Occurrences: 5")
	assert.NotContains(t, text, "--- Request ---")
}

func TestSnapshotToJSON(t *testing.T) {
	s := testSnapshot()
	j := snapshotToJSON(s)

	assert.Contains(t, j, `"hash"`)
	assert.Contains(t, j, `"request"`)
	assert.Contains(t, j, `"response"`)
	assert.Contains(t, j, `"attributeState"`)
}

func TestStatsToText(t *testing.T) {
	stats := &models.SnapshotStats{
		TotalCount:    10,
		ByMethod:      map[string]int64{"GET": 7, "POST": 3},
		ByStatusRange: map[string]int64{"2xx": 8, "5xx": 2},
		TopEndpoints: []models.EndpointStat{
			{Path: "/api/users", TotalOccurrences: 20, UniqueSnapshots: 5},
		},
		TimeRange: models.TimeRange{
			Earliest: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			Latest:   time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	text := statsToText(stats)

	assert.Contains(t, text, "Total Snapshots: 10")
	assert.Contains(t, text, "GET")
	assert.Contains(t, text, "POST")
	assert.Contains(t, text, "2xx")
	assert.Contains(t, text, "5xx")
	assert.Contains(t, text, "/api/users")
	assert.Contains(t, text, "Time Range")
}

func TestDiffSnapshots(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	a := &models.Snapshot{
		Hash: "aaaa000000000000000000000000000000000000000000000000000000000000",
		Request: models.Request{
			Method: "GET",
			Path:   "/api/users",
		},
		Response: models.Response{
			StatusCode: 200,
		},
		AttributeState: models.AttributeState{
			Subject: map[string]interface{}{"role": "admin"},
		},
		Metadata: models.Metadata{
			OccurrenceCount: 5,
			FirstSeen:       now,
			LastSeen:        now,
		},
	}

	b := &models.Snapshot{
		Hash: "bbbb000000000000000000000000000000000000000000000000000000000000",
		Request: models.Request{
			Method: "POST",
			Path:   "/api/users",
		},
		Response: models.Response{
			StatusCode: 201,
		},
		AttributeState: models.AttributeState{
			Subject: map[string]interface{}{"role": "editor"},
		},
		Metadata: models.Metadata{
			OccurrenceCount: 1,
			FirstSeen:       now,
			LastSeen:        now,
		},
	}

	text := diffSnapshots(a, b, "all")

	assert.Contains(t, text, "Diff")
	assert.Contains(t, text, "Method: GET")
	assert.Contains(t, text, "POST")
	assert.Contains(t, text, "Status: 200")
	assert.Contains(t, text, "201")
}

func TestDiffSnapshots_SameSection(t *testing.T) {
	s := testSnapshot()

	text := diffSnapshots(s, s, "request")
	assert.Contains(t, text, "Diff")
	// Same snapshot should not have method/path changes shown
	assert.NotContains(t, text, "Method: GET")
}

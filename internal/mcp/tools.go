package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"

	"github.com/org/nexus/internal/models"
	"github.com/org/nexus/internal/store"
)

// registerTools registers all MCP tools on the server.
func (s *Server) registerTools() {
	s.mcp.AddTool(querySnapshotsTool(), s.handleQuerySnapshots)
	s.mcp.AddTool(getSnapshotTool(), s.handleGetSnapshot)
	s.mcp.AddTool(searchByAttributesTool(), s.handleSearchByAttributes)
	s.mcp.AddTool(getSnapshotStatsTool(), s.handleGetSnapshotStats)
	s.mcp.AddTool(compareSnapshotsTool(), s.handleCompareSnapshots)
}

func querySnapshotsTool() mcp.Tool {
	return mcp.NewTool("query_snapshots",
		mcp.WithDescription("Search and filter previously captured HTTP request/response snapshots. Supports filtering by path, method, status code, and time range with pagination."),
		mcp.WithString("path", mcp.Description("Filter by exact request path (e.g. /api/users)")),
		mcp.WithString("method", mcp.Description("Filter by HTTP method"), mcp.Enum("GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS")),
		mcp.WithNumber("statusCode", mcp.Description("Filter by exact response status code")),
		mcp.WithNumber("minStatus", mcp.Description("Filter by minimum status code (e.g. 400 for errors)")),
		mcp.WithNumber("maxStatus", mcp.Description("Filter by maximum status code")),
		mcp.WithString("from", mcp.Description("Start of time range (RFC3339 format)")),
		mcp.WithString("to", mcp.Description("End of time range (RFC3339 format)")),
		mcp.WithNumber("limit", mcp.Description("Results per page (default 20, max 100)")),
		mcp.WithNumber("offset", mcp.Description("Pagination offset (default 0)")),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}

func (s *Server) handleQuerySnapshots(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := models.SnapshotQuery{
		Path:   req.GetString("path", ""),
		Method: req.GetString("method", ""),
		From:   req.GetString("from", ""),
		To:     req.GetString("to", ""),
		Limit:  int64(req.GetInt("limit", 20)),
		Offset: int64(req.GetInt("offset", 0)),
	}

	if sc := req.GetInt("statusCode", 0); sc > 0 {
		query.StatusCode = sc
	}

	snapshots, total, err := s.store.Find(ctx, query)
	if err != nil {
		s.logger.Error("query_snapshots failed", zap.Error(err))
		return toolError("Failed to query snapshots: " + err.Error()), nil
	}

	// Apply minStatus/maxStatus client-side filtering for the MCP layer
	minStatus := req.GetInt("minStatus", 0)
	maxStatus := req.GetInt("maxStatus", 0)
	if minStatus > 0 || maxStatus > 0 {
		var filtered []models.Snapshot
		for _, snap := range snapshots {
			if minStatus > 0 && snap.Response.StatusCode < minStatus {
				continue
			}
			if maxStatus > 0 && snap.Response.StatusCode > maxStatus {
				continue
			}
			filtered = append(filtered, snap)
		}
		snapshots = filtered
	}

	if len(snapshots) == 0 {
		return toolText("No snapshots found matching the query."), nil
	}

	var sb strings.Builder
	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	fmt.Fprintf(&sb, "Found %d total snapshots (showing %d-%d):\n\n",
		total, query.Offset+1, query.Offset+int64(len(snapshots)))

	for _, snap := range snapshots {
		sb.WriteString(snapshotSummary(snap))
		sb.WriteString("\n")
	}

	if query.Offset+limit < total {
		fmt.Fprintf(&sb, "\n... %d more results available (use offset=%d to see next page)",
			total-query.Offset-limit, query.Offset+limit)
	}

	return toolText(sb.String()), nil
}

func getSnapshotTool() mcp.Tool {
	return mcp.NewTool("get_snapshot",
		mcp.WithDescription("Retrieve the full details of a specific captured HTTP request/response snapshot by its SHA-256 content hash. Supports partial hash prefix lookup."),
		mcp.WithString("hash", mcp.Required(), mcp.Description("The SHA-256 hash (or prefix of at least 6 characters) identifying the snapshot")),
		mcp.WithString("format", mcp.Description("Output format: full, request_only, response_only, attributes_only, metadata_only"), mcp.Enum("full", "request_only", "response_only", "attributes_only", "metadata_only")),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}

func (s *Server) handleGetSnapshot(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	hash, err := req.RequireString("hash")
	if err != nil {
		return toolError("hash parameter is required"), nil
	}

	format := req.GetString("format", "full")

	// Full hash: direct lookup
	if len(hash) == 64 {
		snap, err := s.store.FindByHash(ctx, hash)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return toolError(fmt.Sprintf("No snapshot found with hash %s", hash)), nil
			}
			s.logger.Error("get_snapshot failed", zap.Error(err))
			return toolError("Failed to retrieve snapshot: " + err.Error()), nil
		}
		return toolText(snapshotToText(snap, format)), nil
	}

	// Prefix lookup
	if len(hash) < 6 {
		return toolError("Hash prefix must be at least 6 characters"), nil
	}

	matches, err := s.store.FindByHashPrefix(ctx, hash)
	if err != nil {
		s.logger.Error("get_snapshot prefix lookup failed", zap.Error(err))
		return toolError("Failed to search by hash prefix: " + err.Error()), nil
	}

	if len(matches) == 0 {
		return toolError(fmt.Sprintf("No snapshots found with hash prefix %s", hash)), nil
	}

	if len(matches) == 1 {
		return toolText(snapshotToText(&matches[0], format)), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Multiple snapshots match prefix '%s'. Please use a longer prefix:\n\n", hash)
	for _, m := range matches {
		sb.WriteString(snapshotSummary(m))
		sb.WriteString("\n")
	}
	return toolText(sb.String()), nil
}

func searchByAttributesTool() mcp.Tool {
	return mcp.NewTool("search_by_attributes",
		mcp.WithDescription("Search snapshots by their ABAC attribute state. Query by subject, resource, environment, or context attribute key-value pairs."),
		mcp.WithString("subjectKey", mcp.Description("Key in subject attributes to filter by")),
		mcp.WithString("subjectValue", mcp.Description("Expected value for the subject key")),
		mcp.WithString("resourceKey", mcp.Description("Key in resource attributes to filter by")),
		mcp.WithString("resourceValue", mcp.Description("Expected value for the resource key")),
		mcp.WithString("envKey", mcp.Description("Key in environment attributes to filter by")),
		mcp.WithString("envValue", mcp.Description("Expected value for the environment key")),
		mcp.WithString("contextKey", mcp.Description("Key in context attributes to filter by")),
		mcp.WithString("contextValue", mcp.Description("Expected value for the context key")),
		mcp.WithNumber("limit", mcp.Description("Results per page (default 20, max 100)")),
		mcp.WithNumber("offset", mcp.Description("Pagination offset (default 0)")),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}

func (s *Server) handleSearchByAttributes(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := models.AttributeQuery{
		Limit:  int64(req.GetInt("limit", 20)),
		Offset: int64(req.GetInt("offset", 0)),
	}

	pairs := []struct {
		category, keyParam, valParam string
	}{
		{"subject", "subjectKey", "subjectValue"},
		{"resource", "resourceKey", "resourceValue"},
		{"environment", "envKey", "envValue"},
		{"context", "contextKey", "contextValue"},
	}

	for _, p := range pairs {
		key := req.GetString(p.keyParam, "")
		val := req.GetString(p.valParam, "")
		if key != "" {
			query.Conditions = append(query.Conditions, models.AttributeCondition{
				Category: p.category,
				Key:      key,
				Value:    val,
			})
		}
	}

	if len(query.Conditions) == 0 {
		return toolError("At least one attribute key must be provided (subjectKey, resourceKey, envKey, or contextKey)"), nil
	}

	snapshots, total, err := s.store.FindByAttributes(ctx, query)
	if err != nil {
		s.logger.Error("search_by_attributes failed", zap.Error(err))
		return toolError("Failed to search by attributes: " + err.Error()), nil
	}

	if len(snapshots) == 0 {
		return toolText("No snapshots found matching the attribute criteria."), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d snapshots matching attribute criteria (showing %d):\n\n",
		total, len(snapshots))

	for _, snap := range snapshots {
		sb.WriteString(snapshotSummary(snap))
		sb.WriteString("\n")
	}

	return toolText(sb.String()), nil
}

func getSnapshotStatsTool() mcp.Tool {
	return mcp.NewTool("get_snapshot_stats",
		mcp.WithDescription("Get aggregate statistics about captured API traffic snapshots. Returns counts, distributions by method and status code, and top endpoints."),
		mcp.WithString("from", mcp.Description("Start of time range (RFC3339 format)")),
		mcp.WithString("to", mcp.Description("End of time range (RFC3339 format)")),
		mcp.WithNumber("topN", mcp.Description("Number of top endpoints to return (default 10)")),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}

func (s *Server) handleGetSnapshotStats(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := models.StatsQuery{
		TopN: req.GetInt("topN", 10),
	}

	if fromStr := req.GetString("from", ""); fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return toolError("Invalid 'from' time format. Use RFC3339 (e.g. 2024-01-01T00:00:00Z)"), nil
		}
		query.From = &t
	}

	if toStr := req.GetString("to", ""); toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			return toolError("Invalid 'to' time format. Use RFC3339 (e.g. 2024-01-01T00:00:00Z)"), nil
		}
		query.To = &t
	}

	stats, err := s.store.GetStats(ctx, query)
	if err != nil {
		s.logger.Error("get_snapshot_stats failed", zap.Error(err))
		return toolError("Failed to get stats: " + err.Error()), nil
	}

	return toolText(statsToText(stats)), nil
}

func compareSnapshotsTool() mcp.Tool {
	return mcp.NewTool("compare_snapshots",
		mcp.WithDescription("Compare two captured snapshots side-by-side to identify differences in their requests, responses, or attributes."),
		mcp.WithString("hash1", mcp.Required(), mcp.Description("SHA-256 hash of the first snapshot")),
		mcp.WithString("hash2", mcp.Required(), mcp.Description("SHA-256 hash of the second snapshot")),
		mcp.WithString("sections", mcp.Description("Sections to compare: all, request, response, attributes, metadata"), mcp.Enum("all", "request", "response", "attributes", "metadata")),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}

func (s *Server) handleCompareSnapshots(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	hash1, err := req.RequireString("hash1")
	if err != nil {
		return toolError("hash1 parameter is required"), nil
	}
	hash2, err := req.RequireString("hash2")
	if err != nil {
		return toolError("hash2 parameter is required"), nil
	}
	sections := req.GetString("sections", "all")

	snap1, err := s.store.FindByHash(ctx, hash1)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return toolError(fmt.Sprintf("Snapshot not found: %s", hash1)), nil
		}
		return toolError("Failed to retrieve first snapshot: " + err.Error()), nil
	}

	snap2, err := s.store.FindByHash(ctx, hash2)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return toolError(fmt.Sprintf("Snapshot not found: %s", hash2)), nil
		}
		return toolError("Failed to retrieve second snapshot: " + err.Error()), nil
	}

	return toolText(diffSnapshots(snap1, snap2, sections)), nil
}

// toolText creates a successful text result.
func toolText(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: text},
		},
	}
}

// toolError creates an error text result.
func toolError(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: msg},
		},
		IsError: true,
	}
}

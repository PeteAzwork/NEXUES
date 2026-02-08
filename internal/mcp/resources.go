package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"

	"github.com/org/nexus/internal/models"
)

// registerResources registers all MCP resources on the server.
func (s *Server) registerResources() {
	// Static resources
	s.mcp.AddResource(
		mcp.NewResource("nexus://snapshots", "Recent Snapshots",
			mcp.WithResourceDescription("List of recently captured API request/response snapshots"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleSnapshotsResource,
	)

	s.mcp.AddResource(
		mcp.NewResource("nexus://stats", "Snapshot Statistics",
			mcp.WithResourceDescription("Aggregate statistics about all captured snapshots"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleStatsResource,
	)

	s.mcp.AddResource(
		mcp.NewResource("nexus://attributes", "Attribute Index",
			mcp.WithResourceDescription("Index of all unique attribute keys found across snapshots"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleAttributesResource,
	)

	// Dynamic resource template
	s.mcp.AddResourceTemplate(
		mcp.NewResourceTemplate("nexus://snapshots/{hash}", "Snapshot Detail",
			mcp.WithTemplateDescription("Full detail of a specific captured snapshot by hash"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		s.handleSnapshotDetailResource,
	)
}

func (s *Server) handleSnapshotsResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	query := models.SnapshotQuery{
		Limit: 50,
	}

	snapshots, total, err := s.store.Find(ctx, query)
	if err != nil {
		s.logger.Error("snapshots resource failed", zap.Error(err))
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	type snapshotSummaryDTO struct {
		Hash            string `json:"hash"`
		Method          string `json:"method"`
		Path            string `json:"path"`
		StatusCode      int    `json:"statusCode"`
		LastSeen        string `json:"lastSeen"`
		OccurrenceCount int64  `json:"occurrenceCount"`
	}

	summaries := make([]snapshotSummaryDTO, 0, len(snapshots))
	for _, snap := range snapshots {
		summaries = append(summaries, snapshotSummaryDTO{
			Hash:            snap.Hash,
			Method:          snap.Request.Method,
			Path:            snap.Request.Path,
			StatusCode:      snap.Response.StatusCode,
			LastSeen:        snap.Metadata.LastSeen.Format("2006-01-02T15:04:05Z"),
			OccurrenceCount: snap.Metadata.OccurrenceCount,
		})
	}

	result := map[string]interface{}{
		"total":     total,
		"snapshots": summaries,
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "nexus://snapshots",
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (s *Server) handleSnapshotDetailResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	hash := strings.TrimPrefix(uri, "nexus://snapshots/")
	if hash == "" || hash == uri {
		return nil, fmt.Errorf("invalid snapshot URI: %s", uri)
	}

	snap, err := s.store.FindByHash(ctx, hash)
	if err != nil {
		s.logger.Error("snapshot detail resource failed", zap.Error(err), zap.String("hash", hash))
		return nil, fmt.Errorf("snapshot not found: %s", hash)
	}

	data, _ := json.MarshalIndent(snap, "", "  ")
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (s *Server) handleStatsResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	stats, err := s.store.GetStats(ctx, models.StatsQuery{TopN: 10})
	if err != nil {
		s.logger.Error("stats resource failed", zap.Error(err))
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	data, _ := json.MarshalIndent(stats, "", "  ")
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "nexus://stats",
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (s *Server) handleAttributesResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	index, err := s.store.GetDistinctAttributeKeys(ctx)
	if err != nil {
		s.logger.Error("attributes resource failed", zap.Error(err))
		return nil, fmt.Errorf("failed to get attribute keys: %w", err)
	}

	data, _ := json.MarshalIndent(index, "", "  ")
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "nexus://attributes",
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

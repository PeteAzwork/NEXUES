package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"github.com/org/nexus/internal/store"
)

// Server wraps the MCP server with Nexus dependencies.
type Server struct {
	mcp    *server.MCPServer
	store  store.SnapshotStore
	logger *zap.Logger
}

// New creates a new MCP server configured with all tools, resources, and prompts.
func New(name, version string, s store.SnapshotStore, logger *zap.Logger) *Server {
	mcpServer := server.NewMCPServer(name, version,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, false),
		server.WithPromptCapabilities(false),
		server.WithRecovery(),
		server.WithInstructions("Nexus MCP Server provides tools to query previously captured HTTP request/response snapshots. Use the available tools to search, retrieve, and analyze API traffic stored in the Nexus database."),
	)

	srv := &Server{
		mcp:    mcpServer,
		store:  s,
		logger: logger,
	}

	srv.registerTools()
	srv.registerResources()

	return srv
}

// ServeStdio starts the MCP server using stdio transport.
func (s *Server) ServeStdio() error {
	s.logger.Info("starting MCP server (stdio transport)")
	return server.ServeStdio(s.mcp)
}

// ServeSSE starts the MCP server using SSE transport on the given address.
func (s *Server) ServeSSE(ctx context.Context, addr string) error {
	s.logger.Info("starting MCP server (SSE transport)", zap.String("addr", addr))

	sseServer := server.NewSSEServer(s.mcp,
		server.WithBaseURL(fmt.Sprintf("http://localhost%s", addr)),
	)

	errCh := make(chan error, 1)
	go func() {
		errCh <- sseServer.Start(addr)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.logger.Info("shutting down SSE server")
		return sseServer.Shutdown(ctx)
	}
}

// MCPServer returns the underlying MCP server for testing.
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcp
}

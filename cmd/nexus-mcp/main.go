package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/org/nexus/internal/config"
	mcpserver "github.com/org/nexus/internal/mcp"
	"github.com/org/nexus/internal/store"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	mongoStore, err := store.NewMongoStore(ctx, cfg.MongoURI, cfg.DatabaseName, cfg.Collection, logger)
	if err != nil {
		logger.Fatal("failed to connect to MongoDB", zap.Error(err))
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		mongoStore.Close(shutdownCtx)
	}()

	// Create MCP server
	srv := mcpserver.New(cfg.MCPName, cfg.MCPVersion, mongoStore, logger)

	switch cfg.MCPTransport {
	case "stdio":
		if err := srv.ServeStdio(); err != nil {
			logger.Fatal("MCP stdio server error", zap.Error(err))
		}
	case "sse":
		sseCtx, sseCancel := context.WithCancel(context.Background())
		defer sseCancel()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		errCh := make(chan error, 1)
		go func() {
			errCh <- srv.ServeSSE(sseCtx, cfg.MCPSSEAddr)
		}()

		select {
		case sig := <-quit:
			logger.Info("received shutdown signal", zap.String("signal", sig.String()))
			sseCancel()
		case err := <-errCh:
			if err != nil {
				logger.Fatal("MCP SSE server error", zap.Error(err))
			}
		}
	default:
		logger.Fatal("unsupported MCP transport", zap.String("transport", cfg.MCPTransport))
	}

	logger.Info("nexus-mcp stopped")
}

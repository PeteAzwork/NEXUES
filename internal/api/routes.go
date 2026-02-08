package api

import (
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/org/nexus/internal/store"
)

// NewRouter builds and returns the chi router with all routes and middleware.
func NewRouter(s store.SnapshotStore, logger *zap.Logger) *chi.Mux {
	h := NewHandler(s, logger)

	r := chi.NewRouter()

	// Global middleware
	r.Use(RequestID)
	r.Use(Recoverer(logger))
	r.Use(RequestLogger(logger))

	// Health check
	r.Get("/health", h.HealthCheck)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/log", h.LogSnapshot)
		r.Get("/snapshots", h.ListSnapshots)
		r.Get("/snapshots/{hash}", h.GetSnapshot)
	})

	return r
}

package store

import (
	"context"
	"errors"

	"github.com/org/nexus/internal/models"
)

// Common errors returned by store operations.
var (
	ErrNotFound = errors.New("snapshot not found")
)

// SnapshotStore defines the persistence interface for snapshots.
type SnapshotStore interface {
	Upsert(ctx context.Context, snapshot *models.Snapshot) (*models.UpsertResult, error)
	FindByHash(ctx context.Context, hash string) (*models.Snapshot, error)
	Find(ctx context.Context, query models.SnapshotQuery) ([]models.Snapshot, int64, error)
	HealthCheck(ctx context.Context) error
	Close(ctx context.Context) error
}

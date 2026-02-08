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
	FindByHashPrefix(ctx context.Context, prefix string) ([]models.Snapshot, error)
	FindByAttributes(ctx context.Context, query models.AttributeQuery) ([]models.Snapshot, int64, error)
	GetStats(ctx context.Context, query models.StatsQuery) (*models.SnapshotStats, error)
	GetDistinctAttributeKeys(ctx context.Context) (*models.AttributeIndex, error)
	HealthCheck(ctx context.Context) error
	Close(ctx context.Context) error
}

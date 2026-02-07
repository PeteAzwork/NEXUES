package store

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.uber.org/zap"

	"github.com/org/nexus/internal/models"
)

// MongoStore implements SnapshotStore backed by MongoDB.
type MongoStore struct {
	client     *mongo.Client
	collection *mongo.Collection
	logger     *zap.Logger
}

// NewMongoStore connects to MongoDB and returns a configured MongoStore.
func NewMongoStore(ctx context.Context, uri, dbName, collName string, logger *zap.Logger) (*MongoStore, error) {
	clientOpts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("connecting to mongo: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("pinging mongo: %w", err)
	}

	collection := client.Database(dbName).Collection(collName)

	store := &MongoStore{
		client:     client,
		collection: collection,
		logger:     logger,
	}

	if err := store.ensureIndexes(ctx); err != nil {
		return nil, fmt.Errorf("creating indexes: %w", err)
	}

	logger.Info("connected to MongoDB",
		zap.String("database", dbName),
		zap.String("collection", collName),
	)

	return store, nil
}

func (s *MongoStore) ensureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "hash", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_hash_unique"),
		},
		{
			Keys:    bson.D{{Key: "request.path", Value: 1}},
			Options: options.Index().SetName("idx_request_path"),
		},
		{
			Keys:    bson.D{{Key: "metadata.firstSeen", Value: 1}},
			Options: options.Index().SetName("idx_metadata_firstSeen"),
		},
		{
			Keys:    bson.D{{Key: "response.statusCode", Value: 1}},
			Options: options.Index().SetName("idx_response_statusCode"),
		},
	}

	_, err := s.collection.Indexes().CreateMany(ctx, indexes)
	return err
}

// Upsert inserts a new snapshot or updates an existing one by hash.
func (s *MongoStore) Upsert(ctx context.Context, snapshot *models.Snapshot) (*models.UpsertResult, error) {
	now := time.Now().UTC()

	filter := bson.M{"hash": snapshot.Hash}
	update := bson.M{
		"$setOnInsert": bson.M{
			"hash":           snapshot.Hash,
			"request":        snapshot.Request,
			"response":       snapshot.Response,
			"attributeState": snapshot.AttributeState,
			"metadata": bson.M{
				"firstSeen":       now,
				"lastSeen":        now,
				"occurrenceCount": int64(1),
			},
		},
	}

	// First try to insert
	opts := options.UpdateOne().SetUpsert(true)
	result, err := s.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return nil, fmt.Errorf("upserting snapshot: %w", err)
	}

	// If it was an existing document, increment the count and update lastSeen
	if result.UpsertedCount == 0 {
		incUpdate := bson.M{
			"$set": bson.M{"metadata.lastSeen": now},
			"$inc": bson.M{"metadata.occurrenceCount": int64(1)},
		}
		_, err := s.collection.UpdateOne(ctx, filter, incUpdate)
		if err != nil {
			return nil, fmt.Errorf("incrementing occurrence count: %w", err)
		}

		// Fetch the updated count
		var doc models.Snapshot
		err = s.collection.FindOne(ctx, filter).Decode(&doc)
		if err != nil {
			return nil, fmt.Errorf("fetching updated snapshot: %w", err)
		}

		return &models.UpsertResult{
			IsNew:           false,
			OccurrenceCount: doc.Metadata.OccurrenceCount,
		}, nil
	}

	return &models.UpsertResult{
		IsNew:           true,
		OccurrenceCount: 1,
	}, nil
}

// FindByHash retrieves a single snapshot by its hash.
func (s *MongoStore) FindByHash(ctx context.Context, hash string) (*models.Snapshot, error) {
	var snapshot models.Snapshot
	err := s.collection.FindOne(ctx, bson.M{"hash": hash}).Decode(&snapshot)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("finding snapshot by hash: %w", err)
	}
	return &snapshot, nil
}

// Find queries snapshots with filtering and pagination.
func (s *MongoStore) Find(ctx context.Context, query models.SnapshotQuery) ([]models.Snapshot, int64, error) {
	filter := buildFilter(query)

	total, err := s.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("counting snapshots: %w", err)
	}

	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	findOpts := options.Find().
		SetLimit(limit).
		SetSkip(offset).
		SetSort(bson.D{{Key: "metadata.lastSeen", Value: -1}})

	cursor, err := s.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("finding snapshots: %w", err)
	}
	defer cursor.Close(ctx)

	var snapshots []models.Snapshot
	if err := cursor.All(ctx, &snapshots); err != nil {
		return nil, 0, fmt.Errorf("decoding snapshots: %w", err)
	}

	if snapshots == nil {
		snapshots = []models.Snapshot{}
	}

	return snapshots, total, nil
}

// HealthCheck pings the MongoDB server.
func (s *MongoStore) HealthCheck(ctx context.Context) error {
	return s.client.Ping(ctx, readpref.Primary())
}

// Close disconnects from MongoDB.
func (s *MongoStore) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}

func buildFilter(query models.SnapshotQuery) bson.M {
	filter := bson.M{}

	if query.Path != "" {
		filter["request.path"] = query.Path
	}
	if query.StatusCode > 0 {
		filter["response.statusCode"] = query.StatusCode
	}
	if query.Method != "" {
		filter["request.method"] = query.Method
	}

	if query.From != "" || query.To != "" {
		timeFilter := bson.M{}
		if query.From != "" {
			if t, err := time.Parse(time.RFC3339, query.From); err == nil {
				timeFilter["$gte"] = t
			}
		}
		if query.To != "" {
			if t, err := time.Parse(time.RFC3339, query.To); err == nil {
				timeFilter["$lte"] = t
			}
		}
		if len(timeFilter) > 0 {
			filter["metadata.firstSeen"] = timeFilter
		}
	}

	return filter
}

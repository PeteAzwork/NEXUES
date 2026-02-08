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

// FindByHashPrefix returns snapshots whose hash starts with the given prefix.
func (s *MongoStore) FindByHashPrefix(ctx context.Context, prefix string) ([]models.Snapshot, error) {
	if len(prefix) < 6 {
		return nil, fmt.Errorf("hash prefix must be at least 6 characters, got %d", len(prefix))
	}

	filter := bson.M{"hash": bson.M{"$regex": fmt.Sprintf("^%s", prefix)}}
	findOpts := options.Find().SetLimit(10)

	cursor, err := s.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("finding snapshots by hash prefix: %w", err)
	}
	defer cursor.Close(ctx)

	var snapshots []models.Snapshot
	if err := cursor.All(ctx, &snapshots); err != nil {
		return nil, fmt.Errorf("decoding snapshots: %w", err)
	}
	if snapshots == nil {
		snapshots = []models.Snapshot{}
	}
	return snapshots, nil
}

// FindByAttributes queries snapshots by their ABAC attribute state values.
func (s *MongoStore) FindByAttributes(ctx context.Context, query models.AttributeQuery) ([]models.Snapshot, int64, error) {
	filter := bson.M{}
	for _, cond := range query.Conditions {
		key := fmt.Sprintf("attributeState.%s.%s", cond.Category, cond.Key)
		filter[key] = cond.Value
	}

	total, err := s.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("counting attribute matches: %w", err)
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
		return nil, 0, fmt.Errorf("finding snapshots by attributes: %w", err)
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

// GetStats returns aggregate statistics about captured snapshots.
func (s *MongoStore) GetStats(ctx context.Context, query models.StatsQuery) (*models.SnapshotStats, error) {
	matchStage := bson.M{}
	if query.From != nil || query.To != nil {
		timeFilter := bson.M{}
		if query.From != nil {
			timeFilter["$gte"] = *query.From
		}
		if query.To != nil {
			timeFilter["$lte"] = *query.To
		}
		matchStage["metadata.firstSeen"] = timeFilter
	}

	topN := query.TopN
	if topN <= 0 {
		topN = 10
	}

	pipeline := bson.A{}
	if len(matchStage) > 0 {
		pipeline = append(pipeline, bson.M{"$match": matchStage})
	}

	pipeline = append(pipeline, bson.M{
		"$facet": bson.M{
			"total": bson.A{
				bson.M{"$count": "count"},
			},
			"byMethod": bson.A{
				bson.M{"$group": bson.M{
					"_id":   "$request.method",
					"count": bson.M{"$sum": int64(1)},
				}},
			},
			"byStatus": bson.A{
				bson.M{"$group": bson.M{
					"_id":   "$response.statusCode",
					"count": bson.M{"$sum": int64(1)},
				}},
			},
			"topEndpoints": bson.A{
				bson.M{"$group": bson.M{
					"_id":              "$request.path",
					"totalOccurrences": bson.M{"$sum": "$metadata.occurrenceCount"},
					"uniqueSnapshots":  bson.M{"$sum": int64(1)},
				}},
				bson.M{"$sort": bson.M{"totalOccurrences": -1}},
				bson.M{"$limit": topN},
			},
			"timeRange": bson.A{
				bson.M{"$group": bson.M{
					"_id":      nil,
					"earliest": bson.M{"$min": "$metadata.firstSeen"},
					"latest":   bson.M{"$max": "$metadata.lastSeen"},
				}},
			},
		},
	})

	cursor, err := s.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregating stats: %w", err)
	}
	defer cursor.Close(ctx)

	var results []struct {
		Total []struct {
			Count int64 `bson:"count"`
		} `bson:"total"`
		ByMethod []struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		} `bson:"byMethod"`
		ByStatus []struct {
			ID    int   `bson:"_id"`
			Count int64 `bson:"count"`
		} `bson:"byStatus"`
		TopEndpoints []models.EndpointStat `bson:"topEndpoints"`
		TimeRange    []struct {
			Earliest time.Time `bson:"earliest"`
			Latest   time.Time `bson:"latest"`
		} `bson:"timeRange"`
	}

	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decoding stats: %w", err)
	}

	stats := &models.SnapshotStats{
		ByMethod:      make(map[string]int64),
		ByStatusRange: make(map[string]int64),
		TopEndpoints:  []models.EndpointStat{},
	}

	if len(results) > 0 {
		r := results[0]

		if len(r.Total) > 0 {
			stats.TotalCount = r.Total[0].Count
		}

		for _, m := range r.ByMethod {
			stats.ByMethod[m.ID] = m.Count
		}

		for _, sc := range r.ByStatus {
			switch {
			case sc.ID >= 200 && sc.ID < 300:
				stats.ByStatusRange["2xx"] += sc.Count
			case sc.ID >= 300 && sc.ID < 400:
				stats.ByStatusRange["3xx"] += sc.Count
			case sc.ID >= 400 && sc.ID < 500:
				stats.ByStatusRange["4xx"] += sc.Count
			case sc.ID >= 500 && sc.ID < 600:
				stats.ByStatusRange["5xx"] += sc.Count
			}
		}

		stats.TopEndpoints = r.TopEndpoints
		if stats.TopEndpoints == nil {
			stats.TopEndpoints = []models.EndpointStat{}
		}

		if len(r.TimeRange) > 0 {
			stats.TimeRange = models.TimeRange{
				Earliest: r.TimeRange[0].Earliest,
				Latest:   r.TimeRange[0].Latest,
			}
		}
	}

	return stats, nil
}

// GetDistinctAttributeKeys returns the distinct attribute keys across all snapshots.
func (s *MongoStore) GetDistinctAttributeKeys(ctx context.Context) (*models.AttributeIndex, error) {
	index := &models.AttributeIndex{}

	categories := []struct {
		field string
		dest  *[]string
	}{
		{"attributeState.subject", &index.SubjectKeys},
		{"attributeState.resource", &index.ResourceKeys},
		{"attributeState.environment", &index.EnvironmentKeys},
		{"attributeState.context", &index.ContextKeys},
	}

	for _, cat := range categories {
		pipeline := bson.A{
			bson.M{"$project": bson.M{"keys": bson.M{"$objectToArray": "$" + cat.field}}},
			bson.M{"$unwind": "$keys"},
			bson.M{"$group": bson.M{"_id": "$keys.k"}},
			bson.M{"$sort": bson.M{"_id": 1}},
		}

		cursor, err := s.collection.Aggregate(ctx, pipeline)
		if err != nil {
			return nil, fmt.Errorf("aggregating keys for %s: %w", cat.field, err)
		}

		var keyResults []struct {
			ID string `bson:"_id"`
		}
		if err := cursor.All(ctx, &keyResults); err != nil {
			cursor.Close(ctx)
			return nil, fmt.Errorf("decoding keys for %s: %w", cat.field, err)
		}
		cursor.Close(ctx)

		keys := make([]string, 0, len(keyResults))
		for _, kr := range keyResults {
			keys = append(keys, kr.ID)
		}
		*cat.dest = keys
	}

	return index, nil
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

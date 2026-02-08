package models

import "time"

// StatsQuery holds parameters for aggregate statistics queries.
type StatsQuery struct {
	From    *time.Time
	To      *time.Time
	GroupBy string // "path", "method", "statusCode", "path+method"
	TopN    int
}

// EndpointStat represents aggregated stats for a single endpoint.
type EndpointStat struct {
	Path             string `json:"path" bson:"_id"`
	TotalOccurrences int64  `json:"totalOccurrences" bson:"totalOccurrences"`
	UniqueSnapshots  int64  `json:"uniqueSnapshots" bson:"uniqueSnapshots"`
}

// TimeRange represents the earliest and latest timestamps in a dataset.
type TimeRange struct {
	Earliest time.Time `json:"earliest" bson:"earliest"`
	Latest   time.Time `json:"latest" bson:"latest"`
}

// SnapshotStats holds aggregate statistics about captured snapshots.
type SnapshotStats struct {
	TotalCount    int64            `json:"totalCount"`
	ByMethod      map[string]int64 `json:"byMethod"`
	ByStatusRange map[string]int64 `json:"byStatusRange"`
	TopEndpoints  []EndpointStat   `json:"topEndpoints"`
	TimeRange     TimeRange        `json:"timeRange"`
}

// AttributeCondition defines a single attribute filter condition.
type AttributeCondition struct {
	Category string      // "subject", "resource", "environment", "context"
	Key      string      // dot-notation path within the category
	Value    interface{} // expected value
}

// AttributeQuery holds parameters for attribute-based snapshot queries.
type AttributeQuery struct {
	Conditions []AttributeCondition
	Limit      int64
	Offset     int64
}

// AttributeIndex lists distinct attribute keys found across all snapshots.
type AttributeIndex struct {
	SubjectKeys     []string `json:"subjectKeys"`
	ResourceKeys    []string `json:"resourceKeys"`
	EnvironmentKeys []string `json:"environmentKeys"`
	ContextKeys     []string `json:"contextKeys"`
}

package models

import "time"

// Request represents an HTTP request captured in a snapshot.
type Request struct {
	Method    string            `json:"method" bson:"method" validate:"required,oneof=GET POST PUT PATCH DELETE HEAD OPTIONS"`
	Path      string            `json:"path" bson:"path" validate:"required,startswith=/"`
	Headers   map[string]string `json:"headers" bson:"headers"`
	Body      interface{}       `json:"body" bson:"body"`
	Timestamp time.Time         `json:"timestamp" bson:"timestamp"`
}

// Response represents an HTTP response captured in a snapshot.
type Response struct {
	StatusCode int               `json:"statusCode" bson:"statusCode" validate:"required,gte=100,lte=599"`
	Headers    map[string]string `json:"headers" bson:"headers"`
	Body       interface{}       `json:"body" bson:"body"`
	Timestamp  time.Time         `json:"timestamp" bson:"timestamp"`
}

// AttributeState captures contextual attributes at the time of the request.
type AttributeState struct {
	Subject     map[string]interface{} `json:"subject" bson:"subject"`
	Resource    map[string]interface{} `json:"resource" bson:"resource"`
	Environment map[string]interface{} `json:"environment" bson:"environment"`
	Context     map[string]interface{} `json:"context" bson:"context"`
}

// Metadata tracks occurrence information for a snapshot.
type Metadata struct {
	FirstSeen       time.Time `json:"firstSeen" bson:"firstSeen"`
	LastSeen        time.Time `json:"lastSeen" bson:"lastSeen"`
	OccurrenceCount int64     `json:"occurrenceCount" bson:"occurrenceCount"`
}

// Snapshot is the top-level document stored in MongoDB.
type Snapshot struct {
	Hash           string         `json:"hash" bson:"hash"`
	Request        Request        `json:"request" bson:"request"`
	Response       Response       `json:"response" bson:"response"`
	AttributeState AttributeState `json:"attributeState" bson:"attributeState"`
	Metadata       Metadata       `json:"metadata" bson:"metadata"`
}

// LogRequest is the DTO for POST /api/v1/log.
type LogRequest struct {
	Request        Request        `json:"request" validate:"required"`
	Response       Response       `json:"response" validate:"required"`
	AttributeState AttributeState `json:"attributeState"`
}

// LogResponse is the DTO returned from POST /api/v1/log.
type LogResponse struct {
	Hash            string `json:"hash"`
	IsNew           bool   `json:"isNew"`
	OccurrenceCount int64  `json:"occurrenceCount"`
}

// SnapshotQuery holds query parameters for listing snapshots.
type SnapshotQuery struct {
	Path       string `json:"path"`
	StatusCode int    `json:"statusCode"`
	Method     string `json:"method"`
	From       string `json:"from"`
	To         string `json:"to"`
	Limit      int64  `json:"limit"`
	Offset     int64  `json:"offset"`
}

// SnapshotListResponse is returned for paginated snapshot queries.
type SnapshotListResponse struct {
	Data       []Snapshot `json:"data"`
	Total      int64      `json:"total"`
	Limit      int64      `json:"limit"`
	Offset     int64      `json:"offset"`
}

// UpsertResult contains the result of an upsert operation.
type UpsertResult struct {
	IsNew           bool
	OccurrenceCount int64
}

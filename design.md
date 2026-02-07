# Nexus API - Design Document

## Overview

Nexus is an API request logging service that captures HTTP request/response pairs, deduplicates them via SHA-256 content hashing, and stores them as snapshots in MongoDB. It provides endpoints to log new snapshots, query existing ones, and retrieve individual snapshots by hash.

## Architecture

```
┌─────────────┐     ┌──────────────────────────────────────────────┐
│   Client     │────▶│  Nexus API Server                            │
│  (HTTP)      │◀────│                                              │
└─────────────┘     │  ┌────────────┐  ┌──────────┐  ┌──────────┐ │
                    │  │ Middleware  │──│ Handlers  │──│  Store   │─┼──▶ MongoDB
                    │  │ - RequestID│  │ - Log     │  │ - Upsert │ │
                    │  │ - Logging  │  │ - List    │  │ - Find   │ │
                    │  │ - Recovery │  │ - Get     │  │ - Health │ │
                    │  └────────────┘  │ - Health  │  └──────────┘ │
                    │                  └──────────┘                │
                    │                       │                      │
                    │                  ┌──────────┐                │
                    │                  │  Hash     │                │
                    │                  │ Generator │                │
                    │                  └──────────┘                │
                    └──────────────────────────────────────────────┘
```

## Project Structure

```
nexus/
├── cmd/nexus/main.go           # Application entry point, server bootstrap
├── internal/
│   ├── api/
│   │   ├── handlers.go         # HTTP handler functions
│   │   ├── handlers_test.go    # Handler unit tests
│   │   ├── middleware.go       # Request ID, logging, panic recovery
│   │   ├── middleware_test.go  # Middleware unit tests
│   │   └── routes.go          # Router configuration
│   ├── config/
│   │   ├── config.go          # Environment-based configuration
│   │   └── config_test.go     # Config unit tests
│   ├── hash/
│   │   ├── canonicalize.go    # Deterministic JSON canonicalization
│   │   ├── canonicalize_test.go
│   │   ├── generator.go       # SHA-256 hash generation
│   │   └── generator_test.go
│   ├── models/
│   │   └── snapshot.go        # Core data types and DTOs
│   └── store/
│       ├── interface.go       # SnapshotStore interface
│       ├── mongo.go           # MongoDB implementation
│       └── mock.go            # Mock implementation for testing
├── pkg/client/
│   └── client.go              # Go HTTP client library
├── Dockerfile                 # Multi-stage Docker build
├── docker-compose.yml         # Local development stack
├── Makefile                   # Build automation
└── README.md                  # Usage documentation
```

## Core Components

### 1. Hash Generation

The hash system provides deterministic content-addressed identification of snapshots.

**Canonicalization** (`internal/hash/canonicalize.go`):
- Recursively sorts all map keys alphabetically
- Trims whitespace from string values
- Excludes configurable fields (e.g., `timestamp`, `requestId`)
- Handles nil values, nested objects, and arrays
- Case-insensitive field exclusion

**Hash Generation** (`internal/hash/generator.go`):
- Takes request, response, and attribute state as inputs
- Canonicalizes each component independently
- Concatenates with pipe (`|`) separator
- Produces SHA-256 hex-encoded hash (64 characters)

This ensures:
- Same logical request always produces the same hash
- Field ordering doesn't matter
- Volatile fields (timestamps) are excluded

### 2. Data Models

**Snapshot** - The primary document stored in MongoDB:
```
{
  hash: string (unique, content-addressed),
  request: {method, path, headers, body, timestamp},
  response: {statusCode, headers, body, timestamp},
  attributeState: {subject, resource, environment, context},
  metadata: {firstSeen, lastSeen, occurrenceCount}
}
```

**Key design decisions:**
- `hash` serves as the logical primary key (with a unique index)
- `metadata.occurrenceCount` tracks how many times identical requests are seen
- `attributeState` captures contextual ABAC attributes at request time

### 3. MongoDB Store

**Upsert Strategy:**
1. Attempt `UpdateOne` with `upsert: true` and `$setOnInsert` for all fields
2. If document already exists (`UpsertedCount == 0`), run a second update with `$inc` for `occurrenceCount` and `$set` for `lastSeen`
3. Return whether the document was new and the current occurrence count

**Indexes:**
| Index | Field | Type | Purpose |
|-------|-------|------|---------|
| idx_hash_unique | `hash` | Unique | Primary lookup, upsert |
| idx_request_path | `request.path` | Regular | Path-based queries |
| idx_metadata_firstSeen | `metadata.firstSeen` | Regular | Time-range queries |
| idx_response_statusCode | `response.statusCode` | Regular | Status code filtering |

### 4. API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/log` | Log a request/response snapshot |
| `GET` | `/api/v1/snapshots` | List snapshots with filtering |
| `GET` | `/api/v1/snapshots/{hash}` | Get a single snapshot by hash |
| `GET` | `/health` | Health check endpoint |

**POST /api/v1/log**
- Accepts: `{request, response, attributeState}`
- Validates required fields and enum values
- Generates SHA-256 hash and upserts into MongoDB
- Returns: `{hash, isNew, occurrenceCount}`

**GET /api/v1/snapshots**
- Query params: `path`, `method`, `statusCode`, `from`, `to`, `limit`, `offset`
- Returns paginated results sorted by `lastSeen` descending
- Default limit: 20, max: 100

### 5. Middleware Stack

Applied in order:
1. **RequestID** - Generates UUID or propagates `X-Request-ID` header
2. **Recoverer** - Catches panics, logs them, returns 500
3. **RequestLogger** - Structured logging of method, path, status, duration

### 6. Configuration

All config via environment variables with `NEXUS_` prefix:

| Variable | Default | Description |
|----------|---------|-------------|
| `NEXUS_PORT` | `8080` | HTTP server port |
| `NEXUS_MONGO_URI` | `mongodb://mongo:27017` | MongoDB connection string |
| `NEXUS_DATABASE_NAME` | `nexus` | MongoDB database name |
| `NEXUS_COLLECTION_NAME` | `snapshots` | MongoDB collection name |
| `NEXUS_LOG_LEVEL` | `info` | Log level |
| `NEXUS_READ_TIMEOUT` | `10` | HTTP read timeout (seconds) |
| `NEXUS_WRITE_TIMEOUT` | `10` | HTTP write timeout (seconds) |
| `NEXUS_IDLE_TIMEOUT` | `120` | HTTP idle timeout (seconds) |

## Testing Strategy

- **Unit tests** cover config, hash generation, canonicalization, API handlers, and middleware
- **Mock store** (`internal/store/mock.go`) provides an in-memory implementation for handler tests
- Tests use `httptest` for full HTTP round-trip testing without a running server
- 35 test cases covering success paths, error paths, validation, edge cases, and deduplication

## Graceful Shutdown

1. Server listens for `SIGINT` / `SIGTERM`
2. Stops accepting new HTTP connections
3. Waits up to 10 seconds for in-flight requests to complete
4. Closes MongoDB connection pool
5. Exits cleanly

# Nexus Implementation Task List (Go)

A step-by-step implementation plan for building the Nexus API request logging service in Go with MongoDB.

---

## Phase 1: Project Setup

### 1.1 Initialize Project Structure
- [ ] Create Go module: `go mod init github.com/org/nexus`
- [ ] Set up directory structure:
  ```
  nexus/
  ├── cmd/
  │   └── nexus/
  │       └── main.go
  ├── internal/
  │   ├── api/
  │   │   ├── handlers.go
  │   │   ├── middleware.go
  │   │   └── routes.go
  │   ├── config/
  │   │   └── config.go
  │   ├── hash/
  │   │   ├── canonicalize.go
  │   │   └── generator.go
  │   ├── models/
  │   │   └── snapshot.go
  │   └── store/
  │       ├── mongo.go
  │       └── interface.go
  ├── pkg/
  │   └── client/
  │       └── client.go
  ├── Dockerfile
  ├── docker-compose.yml
  ├── Makefile
  └── README.md
  ```

### 1.2 Add Dependencies
- [ ] Add HTTP router: `go get github.com/go-chi/chi/v5`
- [ ] Add MongoDB driver: `go get go.mongodb.org/mongo-driver/mongo`
- [ ] Add config management: `go get github.com/kelseyhightower/envconfig`
- [ ] Add structured logging: `go get go.uber.org/zap`
- [ ] Add validation: `go get github.com/go-playground/validator/v10`
- [ ] Add testing utilities: `go get github.com/stretchr/testify`

### 1.3 Configuration Setup
- [ ] Define config struct with environment variable bindings
- [ ] Implement config loading with defaults
- [ ] Add config validation on startup
- [ ] Document all configuration options

---

## Phase 2: Data Models

### 2.1 Define Core Types
- [ ] Create `Request` struct:
  ```go
  type Request struct {
      Method    string            `json:"method" bson:"method"`
      Path      string            `json:"path" bson:"path"`
      Headers   map[string]string `json:"headers" bson:"headers"`
      Body      interface{}       `json:"body" bson:"body"`
      Timestamp time.Time         `json:"timestamp" bson:"timestamp"`
  }
  ```
- [ ] Create `Response` struct:
  ```go
  type Response struct {
      StatusCode int               `json:"statusCode" bson:"statusCode"`
      Headers    map[string]string `json:"headers" bson:"headers"`
      Body       interface{}       `json:"body" bson:"body"`
      Timestamp  time.Time         `json:"timestamp" bson:"timestamp"`
  }
  ```
- [ ] Create `AttributeState` struct:
  ```go
  type AttributeState struct {
      Subject     map[string]interface{} `json:"subject" bson:"subject"`
      Resource    map[string]interface{} `json:"resource" bson:"resource"`
      Environment map[string]interface{} `json:"environment" bson:"environment"`
      Context     map[string]interface{} `json:"context" bson:"context"`
  }
  ```
- [ ] Create `Metadata` struct with firstSeen, lastSeen, occurrenceCount
- [ ] Create `Snapshot` document struct combining all elements

### 2.2 Define API DTOs
- [ ] Create `LogRequest` for POST /api/v1/log input
- [ ] Create `LogResponse` for POST /api/v1/log output
- [ ] Create `SnapshotQuery` for GET query parameters
- [ ] Create `SnapshotListResponse` for paginated results

### 2.3 Add Validation Tags
- [ ] Add struct validation tags (required fields, enum values)
- [ ] Create custom validators for path format, status codes
- [ ] Write validation error response formatter

---

## Phase 3: Hash Generation

### 3.1 Implement Canonicalization
- [ ] Create `Canonicalizer` interface:
  ```go
  type Canonicalizer interface {
      Canonicalize(v interface{}, excludeFields []string) ([]byte, error)
  }
  ```
- [ ] Implement recursive key sorting for maps
- [ ] Implement field exclusion logic (timestamps, requestIds)
- [ ] Handle nested objects and arrays
- [ ] Normalize string values (trim whitespace)
- [ ] Write unit tests for edge cases:
  - [ ] Empty objects
  - [ ] Nested arrays
  - [ ] Nil values
  - [ ] Special characters

### 3.2 Implement Hash Generator
- [ ] Create `HashGenerator` interface:
  ```go
  type HashGenerator interface {
      Generate(request, response, attributeState interface{}) (string, error)
  }
  ```
- [ ] Implement SHA-256 hash generation
- [ ] Combine canonicalized components deterministically
- [ ] Return hex-encoded hash string
- [ ] Write unit tests verifying:
  - [ ] Same input produces same hash
  - [ ] Different input produces different hash
  - [ ] Field order doesn't affect hash
  - [ ] Excluded fields don't affect hash

---

## Phase 4: MongoDB Store

### 4.1 Connection Management
- [ ] Implement connection factory with retry logic
- [ ] Configure connection pooling (min/max connections)
- [ ] Implement health check ping
- [ ] Handle graceful shutdown with context cancellation
- [ ] Add connection monitoring/logging

### 4.2 Index Creation
- [ ] Create unique index on `hash` field
- [ ] Create index on `request.path`
- [ ] Create index on `metadata.firstSeen`
- [ ] Create index on `response.statusCode`
- [ ] Make index creation idempotent (check before create)

### 4.3 Implement Store Interface
- [ ] Define `SnapshotStore` interface:
  ```go
  type SnapshotStore interface {
      Upsert(ctx context.Context, snapshot *Snapshot) (*UpsertResult, error)
      FindByHash(ctx context.Context, hash string) (*Snapshot, error)
      Find(ctx context.Context, query SnapshotQuery) ([]Snapshot, int64, error)
      HealthCheck(ctx context.Context) error
  }
  ```
- [ ] Implement `Upsert` with MongoDB upsert operation:
  - [ ] Use `$setOnInsert` for immutable fields
  - [ ] Use `$set` for lastSeen
  - [ ] Use `$inc` for occurrenceCount
  - [ ] Return whether document was new or existing
- [ ] Implement `FindByHash` with single document lookup
- [ ] Implement `Find` with filtering and pagination:
  - [ ] Build dynamic filter from query params
  - [ ] Implement cursor-based or offset pagination
  - [ ] Return total count for pagination
- [ ] Implement `HealthCheck` with ping

### 4.4 Error Handling
- [ ] Define custom error types (NotFound, DuplicateKey, Timeout)
- [ ] Wrap MongoDB errors with context
- [ ] Implement retry logic for transient errors

---

## Phase 5: API Handlers

### 5.1 Implement POST /api/v1/log
- [ ] Parse and validate request body
- [ ] Call hash generator with request components
- [ ] Call store upsert with generated hash and data
- [ ] Return hash, isNew status, occurrence count
- [ ] Handle validation errors (400)
- [ ] Handle store errors (500)

### 5.2 Implement GET /api/v1/snapshots
- [ ] Parse query parameters (path, statusCode, from, to, limit, offset)
- [ ] Validate query parameters
- [ ] Call store find with query
- [ ] Return paginated results with total count
- [ ] Handle empty results gracefully

### 5.3 Implement GET /api/v1/snapshots/:hash
- [ ] Extract hash from path parameter
- [ ] Call store findByHash
- [ ] Return snapshot document
- [ ] Handle not found (404)

### 5.4 Implement GET /health
- [ ] Check MongoDB connectivity
- [ ] Return health status and version info
- [ ] Include dependency health details

---

## Phase 6: Middleware & Infrastructure

### 6.1 Implement Middleware
- [ ] Request ID generation/propagation
- [ ] Structured request logging (method, path, duration, status)
- [ ] Panic recovery with error logging
- [ ] CORS configuration (if needed)
- [ ] Request timeout enforcement

### 6.2 Implement Graceful Shutdown
- [ ] Listen for SIGINT/SIGTERM signals
- [ ] Stop accepting new requests
- [ ] Wait for in-flight requests to complete
- [ ] Close MongoDB connection pool
- [ ] Exit cleanly with appropriate code

### 6.3 Metrics (Optional)
- [ ] Add Prometheus metrics endpoint
- [ ] Track request count by endpoint and status
- [ ] Track request duration histogram
- [ ] Track hash collisions (duplicate count)
- [ ] Track MongoDB operation latency

---

## Phase 7: Testing

### 7.1 Unit Tests
- [ ] Test canonicalization with various inputs
- [ ] Test hash generation determinism
- [ ] Test model validation
- [ ] Test query parameter parsing
- [ ] Achieve >80% coverage on business logic

### 7.2 Integration Tests
- [ ] Set up test MongoDB container (testcontainers-go)
- [ ] Test upsert creates new document
- [ ] Test upsert updates existing document
- [ ] Test find with various filters
- [ ] Test find pagination
- [ ] Test concurrent upserts with same hash

### 7.3 API Tests
- [ ] Test POST /api/v1/log success path
- [ ] Test POST /api/v1/log validation errors
- [ ] Test GET /api/v1/snapshots with filters
- [ ] Test GET /api/v1/snapshots/:hash found
- [ ] Test GET /api/v1/snapshots/:hash not found
- [ ] Test health endpoint

---

## Phase 8: Containerization & Deployment

### 8.1 Docker Setup
- [ ] Create multi-stage Dockerfile:
  ```dockerfile
  # Build stage
  FROM golang:1.22-alpine AS builder
  # ... build steps

  # Runtime stage
  FROM alpine:3.19
  # ... minimal runtime
  ```
- [ ] Create docker-compose.yml with MongoDB
- [ ] Add .dockerignore file
- [ ] Test local container build and run

### 8.2 Makefile Targets
- [ ] `make build` - Build binary
- [ ] `make test` - Run all tests
- [ ] `make test-unit` - Run unit tests only
- [ ] `make test-integration` - Run integration tests
- [ ] `make lint` - Run golangci-lint
- [ ] `make docker-build` - Build Docker image
- [ ] `make docker-run` - Run with docker-compose
- [ ] `make clean` - Clean build artifacts

### 8.3 CI/CD Pipeline
- [ ] Create GitHub Actions workflow
- [ ] Run linting on PR
- [ ] Run tests on PR
- [ ] Build and push Docker image on merge to main
- [ ] Tag images with git SHA and version

---

## Phase 9: Documentation

### 9.1 API Documentation
- [ ] Add OpenAPI/Swagger specification
- [ ] Document all endpoints with examples
- [ ] Document error responses
- [ ] Generate API docs from spec

### 9.2 Operational Documentation
- [ ] Document configuration options
- [ ] Document deployment steps
- [ ] Document monitoring and alerting
- [ ] Document troubleshooting guide

### 9.3 Developer Documentation
- [ ] Update README with quick start
- [ ] Document local development setup
- [ ] Document testing approach
- [ ] Add contributing guidelines

---

## Phase 10: Production Readiness

### 10.1 Security Review
- [ ] Validate no sensitive data in logs
- [ ] Review MongoDB authentication setup
- [ ] Implement request size limits
- [ ] Add rate limiting (if needed)
- [ ] Review CORS configuration

### 10.2 Performance Tuning
- [ ] Benchmark upsert operation
- [ ] Benchmark query operations
- [ ] Tune MongoDB connection pool size
- [ ] Tune HTTP server timeouts
- [ ] Profile memory usage under load

### 10.3 Observability
- [ ] Ensure structured logging throughout
- [ ] Add trace ID propagation
- [ ] Configure log levels by environment
- [ ] Set up alerting thresholds

---

## Estimated Timeline

| Phase | Description | Estimate |
|-------|-------------|----------|
| 1 | Project Setup | 0.5 day |
| 2 | Data Models | 0.5 day |
| 3 | Hash Generation | 1 day |
| 4 | MongoDB Store | 1.5 days |
| 5 | API Handlers | 1 day |
| 6 | Middleware & Infrastructure | 0.5 day |
| 7 | Testing | 1.5 days |
| 8 | Containerization | 0.5 day |
| 9 | Documentation | 0.5 day |
| 10 | Production Readiness | 1 day |
| **Total** | | **~8-9 days** |

---

## Dependencies & Decisions Needed

### Blocking Questions
1. Which Go version to target? (recommend 1.22+)
2. MongoDB version requirements? (recommend 6.0+)
3. Authentication mechanism for the API?
4. Maximum document size limit?
5. Retention policy / TTL requirements?

### Technical Decisions
1. HTTP router choice (chi vs gin vs stdlib)
2. Structured logging library (zap vs slog)
3. Error handling pattern (custom types vs pkg/errors)
4. Test database strategy (testcontainers vs embedded)

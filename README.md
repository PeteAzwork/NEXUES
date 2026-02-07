# Nexus - API Request Logging Service

Nexus captures HTTP request/response pairs, deduplicates them via content hashing, and stores them as queryable snapshots in MongoDB.

## Quick Start

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/)
- (Optional) Go 1.22+ for local development

### Start with Docker Compose

```bash
# Build and start the API + MongoDB
docker compose up -d

# Verify it's running
curl http://localhost:8080/health
```

This starts:
- **Nexus API** on port `8080`
- **MongoDB** on port `27017`

### Stop

```bash
docker compose down

# To also remove stored data:
docker compose down -v
```

## API Usage

### Log a Snapshot

```bash
curl -X POST http://localhost:8080/api/v1/log \
  -H "Content-Type: application/json" \
  -d '{
    "request": {
      "method": "GET",
      "path": "/api/users",
      "headers": {"Authorization": "Bearer token123"},
      "body": null
    },
    "response": {
      "statusCode": 200,
      "headers": {"Content-Type": "application/json"},
      "body": {"users": [{"id": 1, "name": "Alice"}]}
    },
    "attributeState": {
      "subject": {"role": "admin"},
      "resource": {"type": "user-list"},
      "environment": {"ip": "10.0.0.1"},
      "context": {}
    }
  }'
```

**Response:**
```json
{
  "hash": "a1b2c3d4e5f6...",
  "isNew": true,
  "occurrenceCount": 1
}
```

Sending the same request again returns `"isNew": false` with an incremented `occurrenceCount`.

### List Snapshots

```bash
# All snapshots (default limit: 20)
curl http://localhost:8080/api/v1/snapshots

# Filter by path
curl "http://localhost:8080/api/v1/snapshots?path=/api/users"

# Filter by method and status code
curl "http://localhost:8080/api/v1/snapshots?method=GET&statusCode=200"

# Pagination
curl "http://localhost:8080/api/v1/snapshots?limit=10&offset=20"

# Time range (RFC3339)
curl "http://localhost:8080/api/v1/snapshots?from=2024-01-01T00:00:00Z&to=2024-12-31T23:59:59Z"
```

**Response:**
```json
{
  "data": [ ... ],
  "total": 42,
  "limit": 20,
  "offset": 0
}
```

### Get Snapshot by Hash

```bash
curl http://localhost:8080/api/v1/snapshots/a1b2c3d4e5f6...
```

### Health Check

```bash
curl http://localhost:8080/health
```

**Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "dependencies": {
    "mongodb": "healthy"
  }
}
```

## Configuration

All settings are configured via environment variables with a `NEXUS_` prefix:

| Variable | Default | Description |
|----------|---------|-------------|
| `NEXUS_PORT` | `8080` | HTTP server port |
| `NEXUS_MONGO_URI` | `mongodb://mongo:27017` | MongoDB connection URI |
| `NEXUS_DATABASE_NAME` | `nexus` | MongoDB database name |
| `NEXUS_COLLECTION_NAME` | `snapshots` | MongoDB collection name |
| `NEXUS_LOG_LEVEL` | `info` | Logging level |
| `NEXUS_READ_TIMEOUT` | `10` | HTTP read timeout (seconds) |
| `NEXUS_WRITE_TIMEOUT` | `10` | HTTP write timeout (seconds) |
| `NEXUS_IDLE_TIMEOUT` | `120` | HTTP idle timeout (seconds) |

Override in docker-compose.yml or pass directly:

```bash
NEXUS_PORT=9090 NEXUS_MONGO_URI=mongodb://localhost:27017 ./nexus
```

## Local Development

### Build and Run Locally

```bash
# Install dependencies
go mod tidy

# Build
make build

# Run tests
make test

# Run with coverage
make test-coverage
```

### Run Tests

```bash
# All tests with race detection
make test

# Unit tests only
make test-unit
```

### Make Targets

| Target | Description |
|--------|-------------|
| `make build` | Build the binary to `bin/nexus` |
| `make test` | Run all tests with race detection |
| `make test-unit` | Run unit tests only |
| `make test-coverage` | Generate HTML coverage report |
| `make docker-build` | Build Docker image |
| `make docker-run` | Start services via Docker Compose |
| `make docker-down` | Stop services |
| `make docker-logs` | Tail Nexus container logs |
| `make clean` | Remove build artifacts and volumes |

## API Endpoints Summary

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/log` | Log a request/response snapshot |
| `GET` | `/api/v1/snapshots` | List snapshots with filtering and pagination |
| `GET` | `/api/v1/snapshots/{hash}` | Retrieve a single snapshot by its hash |
| `GET` | `/health` | Service health check |

## Design

See [design.md](design.md) for architecture details, data models, and technical decisions.

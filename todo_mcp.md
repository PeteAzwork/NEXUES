# Nexus MCP Server — Implementation Task List

A step-by-step implementation plan for adding a Model Context Protocol (MCP) server to the Nexus API logging service, enabling AI assistants to query previously captured request/response/attribute snapshots.

---

## Overview

The MCP server will expose the existing Nexus snapshot data (requests, responses, and ABAC attribute states stored in MongoDB) as queryable **tools** and browsable **resources** via the Model Context Protocol. This allows LLM-based clients (Claude, etc.) to search, retrieve, and analyze captured API traffic programmatically.

### Architecture

```
┌─────────────────┐       ┌──────────────────────────┐       ┌───────────┐
│  MCP Client      │◄─────►│  Nexus MCP Server         │◄─────►│  MongoDB  │
│  (Claude, etc.) │ stdio │  (Go, mcp-go SDK)        │       │ snapshots │
│                 │  or   │                          │       │           │
│                 │  SSE  │  Tools:                  │       │           │
│                 │       │   - query_snapshots      │       │           │
│                 │       │   - get_snapshot          │       │           │
│                 │       │   - search_by_attributes │       │           │
│                 │       │   - get_snapshot_stats   │       │           │
│                 │       │   - compare_snapshots    │       │           │
│                 │       │                          │       │           │
│                 │       │  Resources:              │       │           │
│                 │       │   - nexus://snapshots/   │       │           │
│                 │       │   - nexus://snapshot/{h} │       │           │
└─────────────────┘       └──────────────────────────┘       └───────────┘
```

---

## Phase 1: Project Setup & Dependencies

### 1.1 Add MCP SDK Dependency
- [ ] Research Go MCP SDK options (`github.com/mark3labs/mcp-go` or similar)
- [ ] Add chosen MCP SDK: `go get github.com/mark3labs/mcp-go`
- [ ] Verify SDK supports both stdio and SSE transports
- [ ] Verify SDK supports tools, resources, and prompts primitives

### 1.2 Extend Project Structure
- [ ] Create MCP server directory layout:
  ```
  internal/
  └── mcp/
      ├── server.go         # MCP server bootstrap and lifecycle
      ├── tools.go          # Tool definitions and handlers
      ├── resources.go      # Resource definitions and handlers
      ├── prompts.go        # Prompt template definitions
      └── convert.go        # Snapshot-to-MCP response converters
  cmd/
  └── nexus-mcp/
      └── main.go           # MCP server entry point (standalone)
  ```
- [ ] Decide whether MCP runs as a separate binary (`nexus-mcp`) or as a mode of the existing `nexus` binary (e.g., `nexus --mode mcp`)
- [ ] Add `mcp` build target to Makefile

### 1.3 Extend Configuration
- [ ] Add MCP-specific config fields to `internal/config/config.go`:
  ```go
  // MCP configuration
  MCPEnabled   bool   `envconfig:"MCP_ENABLED" default:"false"`
  MCPTransport string `envconfig:"MCP_TRANSPORT" default:"stdio"`  // "stdio" or "sse"
  MCPSSEPort   int    `envconfig:"MCP_SSE_PORT" default:"8081"`
  MCPName      string `envconfig:"MCP_SERVER_NAME" default:"nexus-mcp"`
  MCPVersion   string `envconfig:"MCP_SERVER_VERSION" default:"1.0.0"`
  ```
- [ ] Validate transport value is one of `stdio` or `sse`
- [ ] Document all new environment variables

---

## Phase 2: MCP Server Core

### 2.1 Server Initialization
- [ ] Implement `NewMCPServer()` factory in `internal/mcp/server.go`:
  - [ ] Accept `store.SnapshotStore` and `*zap.Logger` as dependencies
  - [ ] Initialize MCP server with name and version from config
  - [ ] Register all tools, resources, and prompts
  - [ ] Return configured server instance
- [ ] Implement `Run()` method that starts the selected transport:
  - [ ] stdio mode: read from stdin, write to stdout (default for CLI-based MCP clients)
  - [ ] SSE mode: start HTTP server on configured port for web-based MCP clients

### 2.2 Entry Point
- [ ] Create `cmd/nexus-mcp/main.go`:
  - [ ] Load configuration (reuse `internal/config`)
  - [ ] Initialize MongoDB connection (reuse `internal/store`)
  - [ ] Initialize logger (reuse existing zap setup)
  - [ ] Create and start MCP server
  - [ ] Handle graceful shutdown (SIGINT/SIGTERM)
- [ ] Alternative: Add `--mcp` flag to existing `cmd/nexus/main.go` to start in MCP mode instead of HTTP mode

### 2.3 Response Converters
- [ ] Implement `snapshotToText()` — human-readable text representation of a snapshot
- [ ] Implement `snapshotToJSON()` — JSON representation preserving full structure
- [ ] Implement `snapshotSummary()` — compact one-line summary (method, path, status, hash prefix, occurrence count)
- [ ] Implement `statsToText()` — formatted statistics output
- [ ] Implement `diffToText()` — formatted comparison output between two snapshots

---

## Phase 3: MCP Tools

### 3.1 Tool: `query_snapshots`
Query captured snapshots with flexible filtering and pagination.

- [ ] Define tool schema:
  ```
  Name:        "query_snapshots"
  Description: "Search and filter previously captured HTTP request/response
                snapshots. Supports filtering by path, method, status code,
                and time range with pagination."
  Parameters:
    - path        (string, optional): Filter by request path (exact match)
    - pathPattern (string, optional): Filter by request path (regex match)
    - method      (string, optional): Filter by HTTP method (GET, POST, etc.)
    - statusCode  (integer, optional): Filter by response status code
    - minStatus   (integer, optional): Filter by minimum status code (e.g., 400 for errors)
    - maxStatus   (integer, optional): Filter by maximum status code
    - from        (string, optional): Start of time range (RFC3339)
    - to          (string, optional): End of time range (RFC3339)
    - limit       (integer, optional, default=20, max=100): Results per page
    - offset      (integer, optional, default=0): Pagination offset
    - sortBy      (string, optional, default="lastSeen"): Sort field
    - sortOrder   (string, optional, default="desc"): "asc" or "desc"
  ```
- [ ] Implement handler that:
  - [ ] Validates and parses all parameters
  - [ ] Builds MongoDB query filter (extend `store.buildFilter` if needed)
  - [ ] Calls `store.Find()` with constructed query
  - [ ] Formats results as a text summary list with total count
  - [ ] Includes pagination info (showing X-Y of Z results)
- [ ] Add support for regex path matching in the store layer:
  - [ ] Extend `SnapshotQuery` with `PathPattern` field
  - [ ] Add `$regex` filter to `buildFilter` when pathPattern is provided
- [ ] Add support for status code range filtering:
  - [ ] Extend `SnapshotQuery` with `MinStatus` and `MaxStatus` fields
  - [ ] Add `$gte`/`$lte` filter on `response.statusCode`

### 3.2 Tool: `get_snapshot`
Retrieve a single snapshot by its content hash with full detail.

- [ ] Define tool schema:
  ```
  Name:        "get_snapshot"
  Description: "Retrieve the full details of a specific captured HTTP
                request/response snapshot by its SHA-256 content hash."
  Parameters:
    - hash   (string, required): The SHA-256 hash identifying the snapshot
    - format (string, optional, default="full"): "full", "request_only",
              "response_only", "attributes_only", "metadata_only"
  ```
- [ ] Implement handler that:
  - [ ] Validates hash format (64 hex characters)
  - [ ] Calls `store.FindByHash()`
  - [ ] Returns formatted snapshot based on requested format
  - [ ] Returns clear error if not found
- [ ] Support partial hash lookup (prefix match) as convenience:
  - [ ] If hash length < 64, treat as prefix and find matching snapshots
  - [ ] If exactly one match, return it; if multiple, list them

### 3.3 Tool: `search_by_attributes`
Query snapshots by their ABAC attribute state values.

- [ ] Define tool schema:
  ```
  Name:        "search_by_attributes"
  Description: "Search snapshots by their attribute state (ABAC context).
                Query by subject, resource, environment, or context attributes."
  Parameters:
    - subjectKey    (string, optional): Dot-notation key in subject attributes
    - subjectValue  (string, optional): Expected value for subject key
    - resourceKey   (string, optional): Dot-notation key in resource attributes
    - resourceValue (string, optional): Expected value for resource key
    - envKey        (string, optional): Dot-notation key in environment attributes
    - envValue      (string, optional): Expected value for environment key
    - contextKey    (string, optional): Dot-notation key in context attributes
    - contextValue  (string, optional): Expected value for context key
    - limit         (integer, optional, default=20): Results per page
    - offset        (integer, optional, default=0): Pagination offset
  ```
- [ ] Implement handler that:
  - [ ] Builds dot-notation MongoDB queries for nested attribute fields
  - [ ] Combines multiple attribute filters with `$and`
  - [ ] Returns matching snapshots with attribute highlights
- [ ] Add attribute-based query support to store layer:
  - [ ] New method `FindByAttributes(ctx, attrQuery)` or extend `Find()`
  - [ ] Create MongoDB index on commonly queried attribute paths (optional)

### 3.4 Tool: `get_snapshot_stats`
Retrieve aggregate statistics about captured snapshots.

- [ ] Define tool schema:
  ```
  Name:        "get_snapshot_stats"
  Description: "Get aggregate statistics about captured API traffic snapshots.
                Returns counts, distributions, and top endpoints."
  Parameters:
    - from       (string, optional): Start of time range (RFC3339)
    - to         (string, optional): End of time range (RFC3339)
    - groupBy    (string, optional): Group results by "path", "method",
                  "statusCode", or "path+method"
    - topN       (integer, optional, default=10): Number of top entries to return
  ```
- [ ] Implement handler that:
  - [ ] Builds MongoDB aggregation pipeline
  - [ ] Returns total snapshot count
  - [ ] Returns distribution by HTTP method
  - [ ] Returns distribution by status code ranges (2xx, 3xx, 4xx, 5xx)
  - [ ] Returns top N endpoints by occurrence count
  - [ ] Returns time range of captured data (earliest firstSeen, latest lastSeen)
- [ ] Add aggregation methods to store layer:
  - [ ] `GetStats(ctx, statsQuery) (*SnapshotStats, error)`
  - [ ] Build aggregation pipeline with `$match`, `$group`, `$sort`, `$limit`
  - [ ] Create `SnapshotStats` result struct

### 3.5 Tool: `compare_snapshots`
Compare two snapshots side-by-side to identify differences.

- [ ] Define tool schema:
  ```
  Name:        "compare_snapshots"
  Description: "Compare two captured snapshots to identify differences
                in their requests, responses, or attributes."
  Parameters:
    - hash1     (string, required): SHA-256 hash of first snapshot
    - hash2     (string, required): SHA-256 hash of second snapshot
    - sections  (string, optional, default="all"): "all", "request",
                 "response", "attributes", "metadata"
  ```
- [ ] Implement handler that:
  - [ ] Fetches both snapshots by hash
  - [ ] Computes structural diff between selected sections
  - [ ] Formats differences in a clear, readable text output
  - [ ] Highlights added, removed, and changed fields
- [ ] Implement diff utility in `internal/mcp/convert.go`:
  - [ ] Recursive map comparison
  - [ ] Array comparison with element matching
  - [ ] Formatted diff output (similar to unified diff format)

---

## Phase 4: MCP Resources

### 4.1 Resource: Snapshot List
- [ ] Register resource template: `nexus://snapshots`
  ```
  URI:         "nexus://snapshots"
  Name:        "Recent Snapshots"
  Description: "List of recently captured API request/response snapshots"
  MimeType:    "application/json"
  ```
- [ ] Implement resource handler:
  - [ ] Return the most recent 50 snapshots as a JSON summary list
  - [ ] Include: hash, method, path, statusCode, lastSeen, occurrenceCount

### 4.2 Resource: Individual Snapshot
- [ ] Register resource template: `nexus://snapshots/{hash}`
  ```
  URI Template: "nexus://snapshots/{hash}"
  Name:         "Snapshot Detail"
  Description:  "Full detail of a specific captured snapshot"
  MimeType:     "application/json"
  ```
- [ ] Implement resource handler:
  - [ ] Extract hash from URI
  - [ ] Return full snapshot document as JSON
  - [ ] Return appropriate error for unknown hashes

### 4.3 Resource: Snapshot Statistics
- [ ] Register resource: `nexus://stats`
  ```
  URI:         "nexus://stats"
  Name:        "Snapshot Statistics"
  Description: "Aggregate statistics about all captured snapshots"
  MimeType:    "application/json"
  ```
- [ ] Implement resource handler:
  - [ ] Return overall stats: total count, method distribution, status distribution
  - [ ] Include top 10 endpoints by occurrence

### 4.4 Resource: Attribute Index
- [ ] Register resource: `nexus://attributes`
  ```
  URI:         "nexus://attributes"
  Name:        "Attribute Index"
  Description: "Index of all unique attribute keys found across snapshots"
  MimeType:    "application/json"
  ```
- [ ] Implement resource handler:
  - [ ] Aggregate distinct attribute keys from subject, resource, environment, context
  - [ ] Return organized by attribute category
  - [ ] Add store method: `GetDistinctAttributeKeys(ctx) (*AttributeIndex, error)`

---

## Phase 5: MCP Prompts

### 5.1 Prompt: Analyze Endpoint Traffic
- [ ] Register prompt template:
  ```
  Name:        "analyze-endpoint"
  Description: "Analyze traffic patterns for a specific API endpoint"
  Arguments:
    - path (string, required): The API endpoint path to analyze
  ```
- [ ] Implement prompt handler:
  - [ ] Query all snapshots for the given path
  - [ ] Build a prompt that asks the LLM to analyze:
    - Request patterns and variations
    - Response status code distribution
    - Common attribute states
    - Anomalies or unusual patterns

### 5.2 Prompt: Error Investigation
- [ ] Register prompt template:
  ```
  Name:        "investigate-errors"
  Description: "Investigate error responses (4xx/5xx) in captured traffic"
  Arguments:
    - from (string, optional): Start of time range
    - to   (string, optional): End of time range
  ```
- [ ] Implement prompt handler:
  - [ ] Query snapshots with status >= 400
  - [ ] Build a prompt that asks the LLM to:
    - Categorize error types
    - Identify common failure patterns
    - Suggest potential root causes

### 5.3 Prompt: Traffic Summary
- [ ] Register prompt template:
  ```
  Name:        "traffic-summary"
  Description: "Generate a summary of all captured API traffic"
  Arguments:
    - from (string, optional): Start of time range
    - to   (string, optional): End of time range
  ```
- [ ] Implement prompt handler:
  - [ ] Fetch stats and recent snapshots
  - [ ] Build a prompt that asks the LLM to produce a narrative summary of the API traffic

---

## Phase 6: Store Layer Enhancements

### 6.1 Extend SnapshotStore Interface
- [ ] Add new methods to `store.SnapshotStore`:
  ```go
  // New methods for MCP support
  FindByHashPrefix(ctx context.Context, prefix string) ([]Snapshot, error)
  FindByAttributes(ctx context.Context, query AttributeQuery) ([]Snapshot, int64, error)
  GetStats(ctx context.Context, query StatsQuery) (*SnapshotStats, error)
  GetDistinctAttributeKeys(ctx context.Context) (*AttributeIndex, error)
  ```
- [ ] Update `store/interface.go` with new method signatures

### 6.2 Implement New Store Methods
- [ ] `FindByHashPrefix`:
  - [ ] Use `$regex` with `^prefix` pattern on hash field
  - [ ] Limit results to prevent excessive matches on short prefixes
  - [ ] Require minimum prefix length of 6 characters
- [ ] `FindByAttributes`:
  - [ ] Build dynamic dot-notation query paths: `attributeState.subject.{key}`
  - [ ] Support value matching with type coercion (string, number, boolean)
  - [ ] Combine multiple attribute conditions with `$and`
- [ ] `GetStats`:
  - [ ] Build MongoDB aggregation pipeline:
    ```
    $match (optional time range) →
    $facet {
      total: [{$count}],
      byMethod: [{$group: {_id: "$request.method", count: {$sum: 1}}}],
      byStatus: [{$group: {_id: "$response.statusCode", count: {$sum: 1}}}],
      topEndpoints: [{$group: {_id: "$request.path", count: {$sum: "$metadata.occurrenceCount"}}}, {$sort}, {$limit}],
      timeRange: [{$group: {min: {$min: "$metadata.firstSeen"}, max: {$max: "$metadata.lastSeen"}}}]
    }
    ```
- [ ] `GetDistinctAttributeKeys`:
  - [ ] Use aggregation to extract unique keys from each attribute category
  - [ ] Cache results with a short TTL (attribute keys change infrequently)

### 6.3 New Data Types
- [ ] Define `AttributeQuery` struct:
  ```go
  type AttributeQuery struct {
      Conditions []AttributeCondition
      Limit      int64
      Offset     int64
  }
  type AttributeCondition struct {
      Category string // "subject", "resource", "environment", "context"
      Key      string // dot-notation path within the category
      Value    interface{}
  }
  ```
- [ ] Define `StatsQuery` struct:
  ```go
  type StatsQuery struct {
      From    *time.Time
      To      *time.Time
      GroupBy string
      TopN    int
  }
  ```
- [ ] Define `SnapshotStats` result struct:
  ```go
  type SnapshotStats struct {
      TotalCount     int64
      ByMethod       map[string]int64
      ByStatusRange  map[string]int64 // "2xx", "3xx", "4xx", "5xx"
      ByStatusCode   map[int]int64
      TopEndpoints   []EndpointStat
      TimeRange      TimeRange
  }
  type EndpointStat struct {
      Path            string
      Method          string
      TotalOccurrences int64
      UniqueSnapshots  int64
  }
  type TimeRange struct {
      Earliest time.Time
      Latest   time.Time
  }
  ```
- [ ] Define `AttributeIndex` struct:
  ```go
  type AttributeIndex struct {
      SubjectKeys     []string
      ResourceKeys    []string
      EnvironmentKeys []string
      ContextKeys     []string
  }
  ```

### 6.4 New MongoDB Indexes
- [ ] Add index for attribute queries: `attributeState.subject` (sparse)
- [ ] Add index for attribute queries: `attributeState.resource` (sparse)
- [ ] Add text index on `request.path` for pattern searches (optional)
- [ ] Add compound index `{request.method: 1, response.statusCode: 1}` for stats aggregation

### 6.5 Update Mock Store
- [ ] Add implementations of all new methods to `internal/store/mock.go`
- [ ] Support configurable return values for testing

---

## Phase 7: Testing

### 7.1 Unit Tests — Tools
- [ ] Test `query_snapshots` tool handler:
  - [ ] With no filters (returns all, paginated)
  - [ ] With path filter
  - [ ] With method filter
  - [ ] With status code filter
  - [ ] With time range filter
  - [ ] With combined filters
  - [ ] With invalid parameters (error cases)
  - [ ] With pagination (limit + offset)
- [ ] Test `get_snapshot` tool handler:
  - [ ] With valid full hash
  - [ ] With valid hash prefix (unique match)
  - [ ] With ambiguous hash prefix (multiple matches)
  - [ ] With nonexistent hash (404)
  - [ ] With each format option
- [ ] Test `search_by_attributes` tool handler:
  - [ ] With single attribute condition
  - [ ] With multiple attribute conditions
  - [ ] With nested attribute keys (dot-notation)
  - [ ] With no results
- [ ] Test `get_snapshot_stats` tool handler:
  - [ ] With no time range (global stats)
  - [ ] With time range filter
  - [ ] With groupBy parameter
  - [ ] With empty database
- [ ] Test `compare_snapshots` tool handler:
  - [ ] With two different snapshots
  - [ ] With identical snapshots
  - [ ] With nonexistent hash
  - [ ] With each section filter

### 7.2 Unit Tests — Resources
- [ ] Test `nexus://snapshots` resource:
  - [ ] Returns recent snapshots as JSON
  - [ ] Handles empty database
- [ ] Test `nexus://snapshots/{hash}` resource:
  - [ ] Returns full snapshot for valid hash
  - [ ] Returns error for invalid hash
- [ ] Test `nexus://stats` resource:
  - [ ] Returns correct aggregate stats
- [ ] Test `nexus://attributes` resource:
  - [ ] Returns distinct attribute keys

### 7.3 Unit Tests — Store Layer
- [ ] Test `FindByHashPrefix`:
  - [ ] Unique prefix match
  - [ ] Multiple prefix matches
  - [ ] No matches
  - [ ] Minimum prefix length enforcement
- [ ] Test `FindByAttributes`:
  - [ ] Single condition match
  - [ ] Multi-condition AND match
  - [ ] Dot-notation nested key match
  - [ ] No results
- [ ] Test `GetStats`:
  - [ ] Correct aggregation counts
  - [ ] Time range filtering
  - [ ] TopN limiting
  - [ ] Empty collection handling
- [ ] Test `GetDistinctAttributeKeys`:
  - [ ] Returns all unique keys across categories
  - [ ] Handles empty attributes

### 7.4 Integration Tests
- [ ] Set up test MongoDB with fixture data (reuse existing test infrastructure)
- [ ] Test full MCP tool round-trip: initialize server → call tool → verify result
- [ ] Test full MCP resource round-trip: initialize server → read resource → verify content
- [ ] Test MCP server startup and shutdown lifecycle
- [ ] Test stdio transport end-to-end with piped input/output
- [ ] Test SSE transport end-to-end with HTTP client

### 7.5 MCP Protocol Compliance Tests
- [ ] Test `initialize` handshake returns correct capabilities
- [ ] Test `tools/list` returns all registered tools with valid schemas
- [ ] Test `resources/list` returns all registered resources
- [ ] Test `prompts/list` returns all registered prompts
- [ ] Test error responses conform to MCP JSON-RPC error format
- [ ] Test content types in tool responses are valid MCP content blocks

---

## Phase 8: Containerization & Deployment

### 8.1 Update Dockerfile
- [ ] Add build target for `nexus-mcp` binary (if separate binary):
  ```dockerfile
  # Build MCP server
  RUN go build -o /app/nexus-mcp ./cmd/nexus-mcp/
  ```
- [ ] Or: ensure existing binary supports `--mcp` mode
- [ ] Keep same multi-stage build pattern

### 8.2 Update Docker Compose
- [ ] Add `nexus-mcp` service to `docker-compose.yml`:
  ```yaml
  nexus-mcp:
    build: .
    command: ["/app/nexus-mcp"]
    environment:
      - NEXUS_MONGO_URI=mongodb://mongo:27017
      - NEXUS_MCP_TRANSPORT=sse
      - NEXUS_MCP_SSE_PORT=8081
    ports:
      - "8081:8081"
    depends_on:
      - mongo
  ```
- [ ] Add MCP-specific health check

### 8.3 MCP Client Configuration
- [ ] Create example `mcp_config.json` for Claude Desktop:
  ```json
  {
    "mcpServers": {
      "nexus": {
        "command": "./nexus-mcp",
        "args": [],
        "env": {
          "NEXUS_MONGO_URI": "mongodb://localhost:27017",
          "NEXUS_MCP_TRANSPORT": "stdio"
        }
      }
    }
  }
  ```
- [ ] Create example config for SSE transport mode
- [ ] Document both configuration approaches

### 8.4 Update Makefile
- [ ] Add `make build-mcp` target
- [ ] Add `make run-mcp` target (stdio mode)
- [ ] Add `make run-mcp-sse` target (SSE mode)
- [ ] Add `make test-mcp` target for MCP-specific tests
- [ ] Update `make build` to build both binaries
- [ ] Update `make docker-build` to include MCP server

---

## Phase 9: Documentation

### 9.1 MCP Server Documentation
- [ ] Document all available tools with parameter descriptions and examples:
  - [ ] `query_snapshots` with example queries
  - [ ] `get_snapshot` with example hash lookup
  - [ ] `search_by_attributes` with example attribute queries
  - [ ] `get_snapshot_stats` with example aggregations
  - [ ] `compare_snapshots` with example diff output
- [ ] Document all available resources with URI patterns
- [ ] Document all available prompts with argument descriptions

### 9.2 Setup & Configuration Guide
- [ ] Document stdio transport setup (for Claude Desktop / CLI clients)
- [ ] Document SSE transport setup (for web-based MCP clients)
- [ ] Document environment variable reference
- [ ] Document prerequisite: running MongoDB with Nexus snapshot data

### 9.3 Usage Examples
- [ ] Example: "Find all failed requests to /api/users"
- [ ] Example: "Show me the details of snapshot abc123..."
- [ ] Example: "What are the most frequently hit endpoints?"
- [ ] Example: "Compare two versions of a request to /api/orders"
- [ ] Example: "Find requests where the subject role was 'admin'"

### 9.4 Update Existing Docs
- [ ] Update project README to mention MCP server capability
- [ ] Update architecture diagram to include MCP server component

---

## Phase 10: Production Readiness

### 10.1 Error Handling
- [ ] All tool handlers return meaningful error messages (not stack traces)
- [ ] MongoDB connection failures are surfaced as MCP errors with retry guidance
- [ ] Invalid parameters return descriptive validation errors
- [ ] Large result sets are truncated with a clear message and suggestion to narrow query

### 10.2 Performance
- [ ] Add query timeout limits (prevent long-running aggregation queries)
- [ ] Set maximum result sizes for tool responses (MCP content size limits)
- [ ] Ensure aggregation pipelines use indexes effectively
- [ ] Profile memory usage of stats aggregation on large collections
- [ ] Consider adding an in-memory cache for frequently accessed stats

### 10.3 Security
- [ ] Sanitize all user-provided regex patterns (prevent ReDoS)
- [ ] Validate hash inputs to prevent injection
- [ ] Ensure no sensitive data (auth tokens, credentials) leaks through attribute queries
- [ ] Rate limit tool calls if using SSE transport (external exposure)
- [ ] Restrict SSE transport to localhost by default

### 10.4 Observability
- [ ] Log all MCP tool calls with parameters (at debug level)
- [ ] Log MCP tool errors (at error level)
- [ ] Track tool call counts and latency (reuse zap structured logging)
- [ ] Log MCP server startup/shutdown events

---

## Dependencies & Decisions Needed

### Blocking Questions
1. **Separate binary vs mode flag?** — Should the MCP server be `cmd/nexus-mcp/main.go` or a `--mcp` flag on the existing binary?
2. **MCP SDK choice** — Confirm `github.com/mark3labs/mcp-go` or evaluate alternatives
3. **Transport priority** — Implement stdio first (simplest, works with Claude Desktop), then SSE?
4. **Attribute query depth** — How deep should dot-notation attribute queries go? Limit nesting?
5. **Stats caching** — Cache aggregation results? If so, what TTL?

### Technical Decisions
1. MCP protocol version to target (latest stable)
2. Maximum content size for tool responses
3. Whether to support MCP sampling capability
4. Whether to support MCP resource subscriptions (live updates)
5. Regex engine safety limits for path pattern matching

---

## Implementation Order (Recommended)

| Step | Phase | Description | Depends On |
|------|-------|-------------|------------|
| 1 | 1 | Project setup, dependencies, structure | — |
| 2 | 2 | MCP server core (init, transport, lifecycle) | Step 1 |
| 3 | 6.1-6.3 | Store layer: new interfaces and data types | — |
| 4 | 6.2 | Store layer: implement new query methods | Step 3 |
| 5 | 6.4-6.5 | Store layer: indexes and mock updates | Step 4 |
| 6 | 3.1-3.2 | Tools: query_snapshots + get_snapshot | Steps 2, 4 |
| 7 | 4.1-4.2 | Resources: snapshot list + detail | Steps 2, 4 |
| 8 | 3.3 | Tool: search_by_attributes | Steps 2, 4 |
| 9 | 3.4 | Tool: get_snapshot_stats | Steps 2, 4 |
| 10 | 3.5 | Tool: compare_snapshots | Steps 2, 4 |
| 11 | 4.3-4.4 | Resources: stats + attribute index | Steps 9, 4 |
| 12 | 5 | Prompts | Steps 6-11 |
| 13 | 7.1-7.3 | Unit tests (tools, resources, store) | Steps 6-12 |
| 14 | 7.4-7.5 | Integration + compliance tests | Step 13 |
| 15 | 8 | Containerization + Makefile updates | Step 14 |
| 16 | 9 | Documentation | Step 15 |
| 17 | 10 | Production hardening | Step 16 |

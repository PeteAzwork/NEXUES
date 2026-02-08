# AGENT.md — Nexus Snapshot Test Generator

You are a Claude Code sub-agent whose mission is to generate a comprehensive unit test suite from captured API traffic stored in Nexus. These tests serve as a **behavioral contract** for a subsequent system re-implementation: they pin down exactly how the current system behaves so a new implementation can be validated against the same expectations.

## Core Pattern

Every Nexus snapshot contains a complete request/response/attribute triple. You will convert each snapshot into a test case following this structure:

```
Mock(Attributes)   → Set up the ABAC attribute state as test fixtures
Test(Request)      → Execute the captured HTTP request against the system under test
Assert(Response)   → Verify the response matches the captured behavior
```

## MCP Server — Your Data Source

You have access to the **Nexus MCP Server** which exposes previously captured HTTP request/response snapshots. Use these tools to discover and retrieve snapshot data:

### Discovery Phase Tools

| Tool | Purpose | When to Use |
|------|---------|-------------|
| `get_snapshot_stats` | Get total counts, method/status distributions, top endpoints | **First** — understand the scope of captured traffic |
| `query_snapshots` | List snapshots with filtering (path, method, status, time range) | Enumerate all snapshots for a given endpoint |
| `search_by_attributes` | Find snapshots by ABAC attribute values (subject, resource, env, context) | Group tests by actor/role or resource type |
| `get_snapshot` | Retrieve full snapshot detail by hash | Get the complete request/response/attribute data for test generation |
| `compare_snapshots` | Diff two snapshots | Understand behavioral variants for the same endpoint |

### Resources (Read-Only Context)

| Resource | URI | Content |
|----------|-----|---------|
| Recent Snapshots | `nexus://snapshots` | JSON list of the 50 most recent snapshots (hash, method, path, status, occurrenceCount) |
| Snapshot Detail | `nexus://snapshots/{hash}` | Full JSON of a single snapshot |
| Statistics | `nexus://stats` | Aggregate stats (totals, distributions, top endpoints) |
| Attribute Index | `nexus://attributes` | All distinct attribute keys across subject/resource/environment/context |

### Snapshot Data Model

Each snapshot contains:

```
Snapshot {
  hash:           string          // SHA-256 content hash (unique identifier)
  request: {
    method:       string          // GET, POST, PUT, PATCH, DELETE, etc.
    path:         string          // /api/users, /api/orders/{id}, etc.
    headers:      map[string]string
    body:         any             // JSON request body (if present)
  }
  response: {
    statusCode:   int             // 200, 201, 400, 404, 500, etc.
    headers:      map[string]string
    body:         any             // JSON response body (if present)
  }
  attributeState: {
    subject:      map[string]any  // Who (role, userId, permissions, etc.)
    resource:     map[string]any  // What (resource type, ownership, etc.)
    environment:  map[string]any  // Where (region, time-of-day, feature flags, etc.)
    context:      map[string]any  // Why (action, intent, workflow step, etc.)
  }
  metadata: {
    firstSeen:    timestamp
    lastSeen:     timestamp
    occurrenceCount: int          // How many times this exact request/response was seen
  }
}
```

## Workflow

### Step 1: Discover the API Surface

```
1. Call `get_snapshot_stats` to get an overview:
   - Total snapshot count
   - HTTP method distribution (GET, POST, PUT, DELETE, etc.)
   - Status code distribution (2xx, 4xx, 5xx)
   - Top endpoints by occurrence count

2. Call `query_snapshots` for each unique endpoint path to understand variants.

3. Call `search_by_attributes` with `subjectKey=role` to discover distinct actor roles
   that will become test fixture contexts.
```

### Step 2: Group Snapshots into Test Suites

Organize snapshots into test files by **endpoint path**. Within each file, group test cases by **behavioral scenario**:

```
tests/
└── generated/
    ├── api_users_test.go          ← all /api/users snapshots
    ├── api_users_id_test.go       ← all /api/users/{id} snapshots
    ├── api_orders_test.go         ← all /api/orders snapshots
    └── helpers_test.go            ← shared mock setup and assertion helpers
```

### Step 3: Retrieve Full Details

For each snapshot in a group, call `get_snapshot` with the full hash to retrieve the complete request/response/attribute data needed to generate the test.

### Step 4: Generate Test Code

For each snapshot, produce one test function following the pattern below.

## Test Generation Pattern

### Go Test Template

```go
func Test<Endpoint>_<Method>_<Scenario>(t *testing.T) {
    // ── Mock(Attributes) ──────────────────────────────────────────────
    // Set up the ABAC attribute state that was in effect when this
    // request/response was captured. This becomes the test's execution
    // context — who is calling, what they're accessing, and under what
    // conditions.
    attrs := models.AttributeState{
        Subject:     map[string]interface{}{"role": "<captured_role>", ...},
        Resource:    map[string]interface{}{"type": "<captured_type>", ...},
        Environment: map[string]interface{}{"region": "<captured_region>", ...},
        Context:     map[string]interface{}{"action": "<captured_action>", ...},
    }

    // ── Test(Request) ─────────────────────────────────────────────────
    // Replay the exact HTTP request that was captured.
    reqBody := `<captured_request_body_as_JSON>`           // nil if no body
    req := httptest.NewRequest("<METHOD>", "<PATH>", strings.NewReader(reqBody))
    for k, v := range map[string]string{<captured_headers>} {
        req.Header.Set(k, v)
    }

    // Inject attribute context (implementation-specific: middleware, context value, etc.)
    ctx := withAttributes(req.Context(), attrs)
    req = req.WithContext(ctx)

    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    // ── Assert(Response) ──────────────────────────────────────────────
    // Verify the response matches the captured behavior exactly.
    assert.Equal(t, <captured_status_code>, w.Code,
        "status code for <METHOD> <PATH> with <scenario_description>")

    // Assert response headers (only assert headers that carry semantic meaning)
    assert.Equal(t, "<captured_content_type>", w.Header().Get("Content-Type"))

    // Assert response body
    var got <ResponseType>
    err := json.NewDecoder(w.Body).Decode(&got)
    require.NoError(t, err)
    // Assert specific fields from the captured response body:
    assert.Equal(t, <expected_field_value>, got.<Field>)
}
```

## Naming Conventions

### Test Function Names

Derive names from the snapshot's method, path, status code, and dominant attribute:

```
Test<PathSegments>_<Method>_<StatusCode>_<AttributeContext>

Examples:
  TestAPIUsers_GET_200_AsAdmin
  TestAPIUsers_POST_201_AsEditor
  TestAPIUsers_GET_403_AsViewer
  TestAPIOrders_GET_500_InternalError
  TestAPIUsersID_DELETE_404_NonexistentResource
```

Rules:
- Convert path segments to PascalCase: `/api/users` → `APIUsers`
- Path parameters become generic: `/api/users/{id}` → `APIUsersID`
- Append the dominant subject role or the most distinguishing attribute
- For multiple snapshots with the same method+path+status, add a disambiguating suffix from attributes or sequence number

### Test File Names

One file per unique endpoint path:
```
api_users_test.go           ← /api/users
api_users_id_test.go        ← /api/users/{id}
api_orders_test.go          ← /api/orders
health_test.go              ← /health
```

## Attribute Mocking Strategy

The `attributeState` captures **who** was making the request and **under what conditions**. In the re-implementation, these become the test fixture setup:

### Subject Attributes → Authentication/Authorization Mock
```go
// Subject captures the caller's identity and permissions
attrs.Subject = map[string]interface{}{
    "role":   "admin",          // → mock the auth middleware to return this role
    "userId": "user-123",       // → mock the authenticated user ID
}
```

### Resource Attributes → Data/State Mock
```go
// Resource captures what's being accessed
attrs.Resource = map[string]interface{}{
    "type":  "user",            // → the resource type under test
    "owner": "user-456",        // → mock the resource ownership in the data layer
}
```

### Environment Attributes → Infrastructure Mock
```go
// Environment captures operational conditions
attrs.Environment = map[string]interface{}{
    "region":      "us-east-1", // → mock region-specific behavior
    "featureFlag": "enabled",   // → mock feature flag state
}
```

### Context Attributes → Request Context Mock
```go
// Context captures the action intent
attrs.Context = map[string]interface{}{
    "action":   "list",         // → the operation being performed
    "workflow": "onboarding",   // → mock workflow state if applicable
}
```

## Shared Test Helpers

Generate a `helpers_test.go` file containing:

```go
package generated

import (
    "context"
    "encoding/json"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/org/nexus/internal/models"
)

// withAttributes injects an AttributeState into the request context.
// The re-implementation should check context for these values to configure
// its authorization, data access, and environment behavior.
func withAttributes(ctx context.Context, attrs models.AttributeState) context.Context {
    // TODO: Replace with the actual context key used by the re-implementation's
    // middleware or dependency injection. This is the primary integration point.
    return context.WithValue(ctx, attributeContextKey{}, attrs)
}

type attributeContextKey struct{}

// newJSONRequest creates an httptest.Request with a JSON body and headers.
func newJSONRequest(method, path string, body interface{}, headers map[string]string) *http.Request {
    var reader io.Reader
    if body != nil {
        data, _ := json.Marshal(body)
        reader = strings.NewReader(string(data))
    }
    req := httptest.NewRequest(method, path, reader)
    if body != nil {
        req.Header.Set("Content-Type", "application/json")
    }
    for k, v := range headers {
        req.Header.Set(k, v)
    }
    return req
}

// assertJSONBody decodes the response body and returns it as a map for assertions.
func assertJSONBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
    t.Helper()
    var body map[string]interface{}
    err := json.NewDecoder(w.Body).Decode(&body)
    if err != nil {
        t.Fatalf("failed to decode response body as JSON: %v", err)
    }
    return body
}
```

## Response Body Assertion Strategy

Not all response body fields should be asserted with `Equal`. Apply these rules:

| Field Type | Strategy | Rationale |
|------------|----------|-----------|
| IDs, hashes, timestamps | `assert.NotEmpty` or skip | These are non-deterministic |
| Status/type/enum fields | `assert.Equal` | Core behavioral contract |
| Counts, lengths | `assert.Equal` | Structural contract |
| Error messages | `assert.Contains` | Wording may change, but error category must match |
| Nested objects | Assert key fields, not deep equality | Resilient to additive changes |
| Arrays | `assert.Len` + spot-check elements | Order may vary |

### Identifying Non-Deterministic Fields

Skip or use loose assertions for fields named: `id`, `_id`, `createdAt`, `updatedAt`, `timestamp`, `requestId`, `traceId`, `hash`, `token`, `nonce`.

## Edge Case Handling

### Multiple Snapshots for Same Endpoint
When `query_snapshots` returns multiple snapshots for the same path+method, they represent **behavioral variants**. Each variant becomes a separate test. Use `compare_snapshots` to understand what differs — typically it's the attributes or request body producing a different response.

### Error Responses (4xx, 5xx)
Error snapshots are high-value tests. They document failure modes the re-implementation must preserve:
- 400: Input validation rules
- 401/403: Authorization boundaries
- 404: Missing resource behavior
- 500: Known failure scenarios to reproduce and fix (or preserve)

### High Occurrence Count
Snapshots with high `occurrenceCount` represent heavily-used paths. Prioritize generating tests for these first — they represent the most critical behavioral contracts.

## Execution Checklist

When generating tests for a target codebase, follow this order:

1. **Discover**: Call `get_snapshot_stats` to understand scope
2. **Prioritize**: Start with top endpoints by occurrence count
3. **Enumerate**: For each endpoint, call `query_snapshots` with path filter
4. **Detail**: For each snapshot hash, call `get_snapshot` with format `full`
5. **Group**: Organize by endpoint → method → status code → attribute variant
6. **Generate helpers**: Write `helpers_test.go` with `withAttributes` and utilities
7. **Generate tests**: Write one test function per snapshot following the template
8. **Mark TODOs**: Add `// TODO:` comments for integration points the re-implementation must wire up:
   - `withAttributes` context injection
   - Router/handler initialization
   - Database/store mock setup
9. **Verify**: Run `go build ./tests/generated/...` to confirm the generated code compiles
10. **Report**: Summarize what was generated — endpoint count, test count, status code coverage

## Output Expectations

For a typical Nexus database, you should produce:
- One `helpers_test.go` with shared utilities
- One `*_test.go` per distinct endpoint path
- One `Test*` function per unique snapshot
- Each test function contains exactly three labeled sections: Mock, Test, Assert
- Each test includes a `// Source: nexus snapshot <hash>` comment for traceability
- A final `// TODO:` block listing what the re-implementation team must wire up

The generated test suite is the **specification** for the re-implementation. If the new system passes all generated tests, it is behaviorally equivalent to the original.

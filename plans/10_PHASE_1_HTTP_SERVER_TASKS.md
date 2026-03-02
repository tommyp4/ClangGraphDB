# Phase 1: Go HTTP Server and API Foundation - Detailed Tasks

This document expands on Phase 1 from `plans/10_CAMPAIGN_D3_VISUALIZER.md`, providing the strict technical specifications for the `internal/ui` HTTP server package. This implementation bridges the existing `query.GraphProvider` into a web-accessible JSON API, enabling the future D3 frontend.

## 📋 Task Breakdown

### 1. Define Server Structs & Interfaces
**File:** `internal/ui/server.go`

Define the foundational structures for the HTTP server. The server must act as a dependency injection container for the `GraphProvider` and `Embedder`.

*   **`Server` Struct:**
    ```go
    type Server struct {
        provider query.GraphProvider
        embedder embedding.Embedder
        mux      *http.ServeMux
    }
    ```
    *   **Constructor:** `func NewServer(p query.GraphProvider, e embedding.Embedder) *Server`
        *   Must initialize the `ServeMux`.
        *   Must call an internal `s.routes()` method to register HTTP handlers.

*   **Request/Response Models:**
    Create structures to parse incoming queries and format error responses.
    ```go
    // QueryRequest maps to the expected parameters for the /api/query endpoint.
    type QueryRequest struct {
        Type      string `json:"type"`
        Target    string `json:"target"`
        Target2   string `json:"target2,omitempty"`
        Depth     int    `json:"depth,omitempty"`
        Limit     int    `json:"limit,omitempty"`
        Module    string `json:"module,omitempty"`
        Layer     string `json:"layer,omitempty"`
        EdgeTypes string `json:"edge-types,omitempty"`
        Direction string `json:"direction,omitempty"`
    }

    // ErrorResponse normalizes API errors.
    type ErrorResponse struct {
        Error string `json:"error"`
    }
    ```

### 2. Implement the Routing Harness and Test
**File:** `internal/ui/server_test.go`

Before implementing the API logic, create the test harness using `net/http/httptest`.

*   **Test 1: `TestHealthCheck`**
    *   Initialize `Server` with `nil` dependencies.
    *   Issue `GET /api/health` via `httptest.NewRecorder()`.
    *   Assert status is `200 OK` and body contains `{"status":"ok"}`.
*   **Test 2: `TestQueryRouting` (Mocked)**
    *   To be completed alongside step 3. Will assert that a valid request maps to a mocked `GraphProvider` method.

### 3. Implement Handlers
**File:** `internal/ui/server.go`

Implement the `routes()` method and the handler logic.

*   **`handleHealth()`:**
    *   Returns `http.HandlerFunc` writing `{"status":"ok"}`.
*   **`handleQuery()`:**
    *   Returns `http.HandlerFunc`.
    *   **Input Parsing:** Must support both `GET` (query string parameters) and `POST` (JSON body) requests.
        *   If `GET`, manually map `r.URL.Query().Get("type")`, `target`, `depth` (parse to int, default 1), `limit` (parse to int, default 10) into a `QueryRequest`.
        *   If `POST`, use `json.NewDecoder(r.Body).Decode(&req)` into a `QueryRequest`.
    *   **Validation:** Require `Type` and typically `Target`. Return `400 Bad Request` with `ErrorResponse` if missing.
    *   **Switch Logic (The Bridge):**
        *   Switch on `req.Type` (e.g., `search-similar`, `hybrid-context`, `neighbors`, `what-if`).
        *   Translate the `QueryRequest` into the exact `query.GraphProvider` method call (mirroring the CLI logic in `cmd/graphdb/cmd_query.go`).
        *   *Semantic Embedding handling:* For `search-similar` or `hybrid-context`, if `req.Target` is provided and an `embedder` exists, call `s.embedder.EmbedBatch([]string{req.Target})` and pass the `[]float32` array to `s.provider.SearchSimilarFunctions(embedding[0], req.Limit)`. If no embedder exists, return a `500` error indicating semantic search is disabled.
        *   *Special parsing:* For `what-if`, split `req.Target` by `,` just as the CLI does.
    *   **Output Serialization:**
        *   On success, write the raw interface returned by `provider` to the response using `json.NewEncoder(w).Encode(result)`.
        *   On failure, write a `500 Internal Server Error` with `ErrorResponse`.
        *   Always set header `Content-Type: application/json`.

### 4. Implement Server Tests (Verification)
**File:** `internal/ui/server_test.go`

Write comprehensive unit tests ensuring JSON payload/query params bind correctly to `GraphProvider` method calls.

*   **Test 3: `TestQueryNeighbors`**
    *   Create a mock `GraphProvider` (or use a test double) where `GetNeighbors` returns a fake node structure.
    *   Issue `GET /api/query?type=neighbors&target=main&depth=2`.
    *   Assert status is `200 OK`. Decode response and verify the output maps to the fake node.
*   **Test 4: `TestQuerySearchSimilar`**
    *   Create a mock `Embedder` returning a dummy `[]float32` and mock `GraphProvider` returning a mocked `FeatureResult`.
    *   Issue `POST /api/query` with body `{"type":"search-similar", "target":"find the database", "limit":5}`.
    *   Assert the mock `Embedder` was called, and the response is serialized correctly.
*   **Test 5: `TestQueryWhatIf`**
    *   Issue `GET /api/query?type=what-if&target=funcA,funcB`.
    *   Assert the mocked `WhatIf` function receives a slice of `["funcA", "funcB"]`.

### 5. Add `ServeHTTP` interface
**File:** `internal/ui/server.go`

Ensure the `Server` struct implements `http.Handler` so it can be passed directly to `http.ListenAndServe`.

```go
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    s.mux.ServeHTTP(w, r)
}
```

## 🎯 Verification Criteria
- All tests in `go test ./internal/ui/...` pass.
- `internal/ui` has no circular dependencies with `cmd/graphdb`.
- Errors explicitly return JSON (`ErrorResponse`), avoiding raw text dumps in the API.
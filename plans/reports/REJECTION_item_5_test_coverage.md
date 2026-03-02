# Rejection Report: Item 5 (Test Coverage Integration)

## Status: REJECTED

### 1. Compilation Failures (Mocks not updated)
The implementation added `GetCoverage` and `LinkTests` to `query.GraphProvider` (`internal/query/interface.go`), but failed to update the corresponding mock structs. As a result, the project **does not compile** and `go test ./...` fails entirely.
*   **Missing from:** `internal/rpg/orchestrator_test.go` (`MockGraphProvider` struct)
*   **Missing from:** `cmd/graphdb/mocks.go` (`MockProvider` struct)

The Engineer **must** ensure that the build and tests pass across the entire codebase (`go build ./...`, `go test ./...`, and testing the CLI with `-tags mock`) before submitting.

### 2. Implementation Bug (Method Nodes Ignored)
In `internal/query/neo4j_coverage.go`, the Cypher queries in `LinkTests()` and `GetCoverage()` strictly look for nodes labeled `Function` (e.g., `MATCH (t:Function {is_test: true}) MATCH (p:Function)`).
*   **Issue:** The ingestion pipeline in `internal/ingest/worker.go` correctly identifies both `Function` and `Method` nodes for tests, but the queries entirely ignore `Method` labels. This breaks coverage tracking for object-oriented codebases (C#, Java, TypeScript) where tests are written as class methods.
*   **Action:** Update the Cypher queries to handle both `Function` and `Method` nodes (e.g., `MATCH (t) WHERE (t:Function OR t:Method) AND t.is_test = true` or `MATCH (t) WHERE labels(t)[0] IN ['Function', 'Method'] AND t.is_test = true`).

### 3. Missing/Insufficient Unit Tests
While `neo4j_coverage_test.go` exists, it currently only tests linking between `Function` nodes.
*   **Action:** Add tests to ensure `Method` nodes are successfully linked and retrieved by the coverage queries.

### Resolution Instructions
1. Implement the missing mock methods to fix the build errors.
2. Fix the Cypher queries in `internal/query/neo4j_coverage.go` to support `Method` nodes alongside `Function` nodes.
3. Add a test case for `Method` node linkage in `neo4j_coverage_test.go`.
4. Run `go test ./...` and `go build ./...` locally to confirm all tests pass and the system compiles cleanly before resubmitting.
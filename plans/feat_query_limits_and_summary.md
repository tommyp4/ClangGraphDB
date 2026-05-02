# Campaign 19: Query Limits & Summary Features

**Goal:** Handle large query outputs in the CLI to prevent LLM context exhaustion by enforcing depth/limits and providing summary outputs.
**Status:** Completed

## Overview
When the `graphdb` CLI is used on massive legacy classes, the resulting JSON arrays (e.g., in `neighbors` and `hybrid-context` queries) can easily exceed thousands of lines, triggering truncation and wasting LLM context. 

This campaign introduces two solutions:
1.  **Strict Limits:** Enforce the existing `-limit` flag in the `GetNeighbors` function down to the Cypher query level to prevent massive array allocation.
2.  **Summary Mode:** Add a `-summary` boolean flag to the CLI. When true, the CLI outputs structural counts/metrics instead of full data arrays, allowing agents to understand scale without needing the full payload.

## Tasks

### Phase 1: Enforce `-limit` in `GetNeighbors`
- [x] **Step 1:** Update `GraphProvider` interface in `internal/query/interface.go` to change `GetNeighbors(nodeID string, depth int)` to `GetNeighbors(nodeID string, depth int, limit int)`.
- [x] **Step 2:** Update `Neo4jProvider` implementation in `internal/query/neo4j.go`.
    - Change method signature.
    - Modify the cypher query: `RETURN n, (globals + funcs)[0..$limit] as dependencies` (or equivalent safe array slicing in Cypher).
    - Pass the `limit` parameter to the query execution mapping.
- [x] **Step 3:** Update Mock Providers.
    - Update `cmd/graphdb/mocks.go`.
    - Update `internal/ui/server_test.go` mock.
    - Update `internal/rpg/orchestrator_test.go` mock.
- [x] **Step 4:** Update Callers.
    - `cmd/graphdb/cmd_query.go`: Pass `*limitPtr` into `GetNeighbors`.
    - `internal/ui/server.go`: Pass `req.Limit` (or a default) into `GetNeighbors`.
- [x] **Step 5:** Run tests via `make test` to ensure compilation and expected behavior. Update `internal/query/neo4j_test.go` to explicitly test limit bounding.

### Phase 2: Introduce `-summary` CLI Flag
- [x] **Step 1:** Add `summaryPtr := fs.Bool("summary", false, "Output only structural metrics/counts instead of full arrays")` to `cmd/graphdb/cmd_query.go`.
- [x] **Step 2:** Define generic struct(s) or map transformations in `cmd/graphdb/cmd_query.go` for summarizing results.
    - For `NeighborResult`: Output `{ "node": result.Node, "total_dependencies_found": len(result.Dependencies) }`.
    - For `FeatureResult` slice (used in `hybrid-context`): Output counts or just top N results.
    - *Note:* The exact shape of the summary map can be determined during implementation, but it must significantly reduce JSON line count while preserving high-level context (e.g., node identity, risk scores, and array lengths).
- [x] **Step 3:** Modify the `result` serialization block in `cmd/graphdb/cmd_query.go`. If `*summaryPtr` is true, map `result` to its summary form before `json.MarshalIndent`.
- [x] **Step 4:** Verify behavior manually via `go run main.go query -type neighbors -target "SomeFunc" -summary`.

## Acceptance Criteria
- `graphdb query -type neighbors` no longer crashes or blows out stdout when hitting God Classes, cleanly slicing at `-limit` (default 10).
- `graphdb query -type neighbors -summary` prints only the node's properties and the count of its dependencies.
- `make test` passes.
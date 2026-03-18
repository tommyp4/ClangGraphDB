# Task Plan: Item 4 - Restore Git History Analysis & Incremental Ingestion ✅ Implemented

## Sub-tasks

### 1. Update Core Interfaces & Types ✅
- [x] Add `HotspotResult` struct in `internal/query/interface.go`.
- [x] Add `FileHistoryMetrics` struct.
- [x] Add `GetHotspots(modulePattern string) ([]*HotspotResult, error)` to `GraphProvider`.
- [x] Add `UpdateFileHistory(metrics map[string]FileHistoryMetrics) error` to `GraphProvider`.
- [x] Update `MockProvider` in `cmd/graphdb/mocks.go` and `internal/rpg/orchestrator_test.go`.

### 2. Implement History DB Logic ✅
- [x] Create `internal/query/neo4j_history.go`.
- [x] Implement `GetHotspots` (Cypher query joining `risk_score` and `change_frequency`).
- [x] Implement `UpdateFileHistory` (Batch update of File nodes).

### 3. Implement CLI Command: `enrich-history` ✅
- [x] Create `cmd/graphdb/cmd_enrich_history.go`.
- [x] Implement logic to run `git log --format="%H|%cI" --name-only` and parse the output.
- [x] Calculate `change_frequency`, `last_changed`, and `co_changes` (pairwise).
- [x] Call `UpdateFileHistory`.
- [x] Update `main.go` to register `enrich-history`.

### 4. Implement Hotspots Query CLI ✅
- [x] Update `cmd/graphdb/cmd_query.go` to support `-type hotspots`.

### 5. Implement Incremental Ingestion (`neo4j_emitter.go`) ✅
- [x] Create `internal/storage/neo4j_emitter.go`.
- [x] Implement `Emitter` interface writing directly to `neo4j.Neo4jLoader`.

### 6. Update `ingest` CLI for Incremental Mode ✅
- [x] Update `cmd/graphdb/cmd_ingest.go` to add `--since-commit` flag.
- [x] Implement logic to fetch changed files via `git diff --name-only <hash>..HEAD`.
- [x] Pass file list to ingestion logic.
- [x] If `--since-commit` is provided, use `Neo4jEmitter` instead of `JsonlEmitter`.

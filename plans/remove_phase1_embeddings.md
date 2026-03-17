# Feature Implementation Plan: Remove Phase 1 Embeddings

## 📋 Todo Checklist
- [x] ~~Remove `Embedder` interface and related logic from `internal/ingest`~~ ✅ Implemented
- [x] ~~Remove embedder setup from Phase 1 CLI (`cmd/graphdb/cmd_ingest.go`)~~ ✅ Implemented
- [x] ~~Verify `neo4j_batch.go` (`GetUnembeddedNodes`) correctly relies entirely on Phase 3~~ ✅ Implemented
- [x] ~~Verify `build-all` pipeline produces high-fidelity embeddings end-to-end~~ ✅ Implemented
- [x] ~~Final Review and Testing~~ ✅ Implemented

## 🔍 Analysis & Investigation
**Context & Rationale:**
Currently, the pipeline generates embeddings twice. Phase 1 (`graphdb ingest`) embeds raw function names to provide immediate low-fidelity search. Phase 3 (`graphdb enrich`) extracts high-fidelity `atomic_features` via an LLM and is intended to overwrite the Phase 1 embeddings. 

However, several critical issues arise from this approach:
1. **The Overwrite Bug:** Phase 3 currently queries for nodes where `n.embedding IS NULL`. Because Phase 1 already populates an embedding, these nodes are incorrectly skipped by Phase 3, preventing the high-fidelity overwrite from happening.
2. **File Bloat:** Phase 1 embeddings bloat the `nodes.jsonl` output (increasing file size by ~90%), significantly slowing down the Neo4j import process and consuming unnecessary disk space.
3. **Low Value:** Generating low-fidelity embeddings from raw function names provides little value to the primary RPG workflow, which requires the deep semantic understanding provided by Phase 3. Removing them from Phase 1 reduces complexity and speeds up ingestion.

**Goal:**
We will strip embedding responsibilities entirely from the ingestion phase (`graphdb ingest`), allowing Phase 3 (`graphdb enrich`) to act as the sole source of truth for generating embeddings.

## 📝 Implementation Plan

### Prerequisites
None. The required graph data model supports missing `embedding` fields.

### Step-by-Step Implementation

#### Phase 1: Clean Up `internal/ingest`
1.  **Step 1.A (The Harness):** Prepare the test suite in `internal/ingest` by removing embedding dependencies.
    *   *Action:* Modify `internal/ingest/worker_test.go`, `internal/ingest/walker_test.go`, and `internal/ingest/walker_recursive_test.go`.
    *   *Detail:* 
        - Remove the `MockFailingEmbedder` struct and its `EmbedBatch` method.
        - Update calls to `NewWorkerPool` and `NewWalker` to no longer pass the `embedder` parameter (e.g., change `NewWalker(1, &MockFailingEmbedder{}, &MockEmitter{})` to `NewWalker(1, &MockEmitter{})`).
    *   *Goal:* Ensure the test suite compiles after the struct updates.
    *   *Status:* [x] ~~Completed~~ ✅

2.  **Step 1.B (The Implementation):** Remove embedding logic from the worker.
    *   *Action:* Modify `internal/ingest/worker.go`.
    *   *Detail:* 
        - Remove the `embedder embedding.Embedder` field from the `WorkerPool` struct.
        - Remove `embedder` from the `NewWorkerPool` signature and instantiation.
        - In the `processFile` method, delete the entire block of code that filters functions for embedding, calls `wp.embedder.EmbedBatch(functionTexts)`, and assigns `node.Properties["embedding"] = embeddings[i]`.
        - Ensure the `embedding` package import is removed if unused.
    *   *Status:* [x] ~~Completed~~ ✅

3.  **Step 1.C (The Implementation):** Remove embedding dependencies from the walker.
    *   *Action:* Modify `internal/ingest/walker.go`.
    *   *Detail:*
        - Remove `embedder embedding.Embedder` from the `NewWalker` signature.
        - Remove `embedder` from the `Walker` struct and its instantiation.
        - Update the `wp := NewWorkerPool(...)` call inside `Walk` to reflect the updated signature.
    *   *Status:* [x] ~~Completed~~ ✅

4.  **Step 1.D (The Verification):** Verify `internal/ingest` tests.
    *   *Action:* Run `go test ./internal/ingest/...`.
    *   *Success:* All tests pass, confirming ingestion logic works identically without attempting to create embeddings.
    *   *Status:* [x] ~~Completed~~ ✅

#### Phase 2: Update CLI and Query Pipeline
1.  **Step 2.A (The Implementation):** Remove embedder setup from `graphdb ingest`.
    *   *Action:* Modify `cmd/graphdb/cmd_ingest.go`.
    *   *Detail:*
        - Remove the `loc := cfg.GoogleCloudLocation` block and its fallback (lines 37-40), as it exists solely to configure the embedder.
        - Remove the `model := cfg.GeminiEmbeddingModel` block and its fallback (lines 42-45).
        - Remove the `embedder := setupEmbedder(...)` line (line 125).
        - Update `ingest.NewWalker(*workersPtr, embedder, emitter)` to `ingest.NewWalker(*workersPtr, emitter)`.
        - Remove any unused imports resulting from these deletions.

2.  **Step 2.B (The Verification):** Validate query engine assumes zero embeddings from Phase 1.
    *   *Action:* Inspect `internal/query/neo4j_batch.go`.
    *   *Detail:* Confirm that `GetUnembeddedNodes` queries for `(n:Function OR n:Feature) AND n.embedding IS NULL`. Since Phase 1 no longer assigns embeddings, this query will correctly fetch all raw ingested nodes for Phase 3 enrichment. No code change is needed here, just validation.

3.  **Step 2.C (The Verification):** Build and test the CLI.
    *   *Action:* Run `go build ./cmd/graphdb` and `go test ./cmd/graphdb/...`.
    *   *Success:* The project compiles successfully, and any CLI unit tests pass without errors.

4.  **Step 2.D (The Verification):** Verify `build-all` pipeline integrity.
    *   *Action:* Inspect `cmd/graphdb/cmd_build_all.go`.
    *   *Detail:* Confirm that the `build-all` command calls `ingestCmd` (which no longer embeds) followed by `enrichCmd` (which generates embeddings via `RunEmbedding`). No code change is needed -- the fix propagates automatically through the function variable dispatch. Verify the full pipeline by running `build-all` on a small test directory and confirming that `enrich-features` successfully embeds all Function nodes (i.e., `GetUnembeddedNodes` returns results).

### Testing Strategy
- **Unit Testing:** Ensure `go test ./...` passes after removing all `embedder` references from `internal/ingest`.
- **Integration Testing:** Run `graphdb ingest` on a small directory (e.g., `./cmd`) and verify the `nodes.jsonl` output does NOT contain the `"embedding"` property for any `Function` or `Method` nodes.
- **Verification of Phase 3 Fix:** Run `graphdb import` followed by `graphdb enrich-features`. Verify that `enrich-features` successfully finds the unembedded nodes and generates high-fidelity embeddings for them, fully solving the overwrite bug.

### Data Migration (Existing Graphs)
For graphs built before this change (where Phase 1 embeddings are already persisted in Neo4j), the stale low-fidelity embeddings must be cleared before Phase 3 can generate replacements. Two options:

1.  **Clean rebuild (recommended):** Re-run `graphdb build-all -clean` to re-ingest, re-import (wiping the DB), and re-enrich from scratch.
2.  **In-place migration:** Clear stale embeddings manually, then re-run enrichment:
    ```cypher
    MATCH (n:Function) WHERE n.embedding IS NOT NULL AND n.atomic_features IS NOT NULL
    REMOVE n.embedding
    ```
    Then run: `graphdb enrich-features`

## 🎯 Success Criteria
- The `Embedder` interface and its invocations are entirely removed from `internal/ingest`.
- The `graphdb ingest` command executes faster and produces a significantly smaller `nodes.jsonl` file.
- `graphdb enrich-features` correctly identifies and processes nodes that were ingested without embeddings (`n.embedding IS NULL`).
- The project builds and passes all tests successfully without regressions.
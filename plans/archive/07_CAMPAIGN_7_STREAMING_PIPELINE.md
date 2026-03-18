# Feature Implementation Plan: Campaign 7 - Streaming Pipeline (GraphProvider-Centric OOM Resolution)

## 📋 Todo Checklist
- [x] Phase 1: GraphDB Interface/Provider Updates (Batch Read/Write)
- [x] Phase 2: Resumable Extraction Pipeline (Atomic Features)
- [x] Phase 3: Resumable Embedding Pipeline
- [x] Phase 4: Out-of-Core Clustering
- [x] Phase 5: Resumable Summarization
- [x] Final Review and OOM Stress Testing

## 🔍 Analysis & Investigation
The `graphdb` ingest pipeline currently utilizes an in-memory batch architecture that scales linearly with repository size, causing Out of Memory (OOM) crashes on massive (e.g., 31,000 file) codebases. The memory bloat originates from storing raw source code strings (`content`) for every function in memory, maintaining a massive map of precomputed embeddings (~10 GB), and rapid string allocations during LLM context preparation.

To resolve this, the system must adopt a "Pointer to Blob Storage" pattern where the active graph database (via a generic `GraphProvider` interface) acts as the active working memory for intermediate states. The local file system serves as immutable blob storage, accessed on-demand via `snippet.SliceFile()`. The monolithic `enrich-features` command must be split into a provider-agnostic orchestration loop that iterates over database records in discrete, resumable batches. This design ensures encapsulation, preventing tight coupling to Neo4j and allowing for future graph database backends (e.g., Google Cloud Spanner, NebulaGraph).

## 📝 Implementation Plan

### Prerequisites
- The underlying Graph Database is active and initialized with structural data (Phase 1 Ingestion already handles this).
- `internal/tools/snippet` package is available for on-demand disk reads.

### Step-by-Step Implementation

#### Phase 1: GraphDB Interface/Provider Updates
1. **Step 1.A (The Harness):** Define interface tests for batch operations.
   * *Action:* Add interface validation tests (e.g., `internal/query/provider_contract_test.go` or expand mock tests) to ensure any provider implementation correctly handles batch queries and state updates. Update `internal/query/neo4j_test.go` for the concrete Neo4j implementation.
2. **Step 1.B (The Implementation):** Extend `GraphProvider` in `internal/query/interface.go` to support state mutation and chunked reading.
   * *Action:* Add generic methods to query and update batches without exposing backend-specific constructs:
     - `GetUnextractedFunctions(limit int) ([]*graph.Node, error)`
     - `UpdateAtomicFeatures(id string, features []string) error`
     - `GetUnembeddedNodes(limit int) ([]*graph.Node, error)`
     - `UpdateEmbeddings(id string, embedding []float32) error`
     - `GetEmbeddingsOnly() (map[string][]float32, error)`
     - `GetUnnamedFeatures(limit int) ([]*graph.Node, error)`
     - `UpdateFeatureTopology(nodes []*graph.Node, edges []*graph.Edge) error`
     - `UpdateFeatureSummary(id string, name string, summary string) error`
   * *Detail:* Implement these methods in `internal/query/neo4j.go`.
3. **Step 1.C (The Verification):** Verify database interaction.
   * *Action:* Run `go test ./internal/query/...` to ensure all new database methods execute without syntax or logic errors across implementations.

#### Phase 2: Resumable Extraction Pipeline (Atomic Features)
1. **Step 2.A (The Harness):** Create integration test for resumable extraction.
   * *Action:* Update or create `cmd/graphdb/enrich_test.go` using a mock `GraphProvider` to assert that extraction processes limited batches and writes back using interface methods.
2. **Step 2.B (The Implementation):** Refactor Extraction in `cmd/graphdb/main.go` and `internal/rpg/extractor.go`.
   * *Action:* 
     - Remove the loading of all functions into memory via `loadFunctions`.
     - Implement an orchestration loop relying *only* on the interface: `for { nodes := db.GetUnextractedFunctions(batchSize); if len(nodes) == 0 break; ... }`.
     - Inside the loop, enforce the "Pointer to Blob Storage" pattern: use `snippet.SliceFile(node.File, node.Line, node.EndLine)` to read the `content` string lazily.
     - Pass the lazy-loaded code to `extractor.Extract`.
     - Write the resulting `atomic_features` back to the active provider via `db.UpdateAtomicFeatures`.
3. **Step 2.C (The Verification):** Verify memory usage stays flat.
   * *Action:* Run extraction on a mock database with a small batch limit. Run `go test ./internal/rpg/...`.

#### Phase 3: Resumable Embedding Pipeline
1. **Step 3.A (The Harness):** Write test for batched embedding processing.
   * *Action:* Create `internal/rpg/embed_test.go` confirming embeddings are fetched and stored in chunks using a mock `GraphProvider`.
2. **Step 3.B (The Implementation):** Refactor the pre-calculation step in `cmd/graphdb/main.go`.
   * *Action:* 
     - Remove the massive in-memory `precomputed` map allocation.
     - Implement a loop: `for { nodes := db.GetUnembeddedNodes(batchSize); if len(nodes) == 0 break; ... }`.
     - Generate the text representations using `rpg.NodeToText(node)` (which now uses the `atomic_features` fetched from the provider).
     - Fetch embeddings via the API.
     - Write the resulting embeddings back to the provider using `db.UpdateEmbeddings`.
3. **Step 3.C (The Verification):** 
   * *Action:* Run `go test ./cmd/graphdb/...` and verify embeddings are written accurately.

#### Phase 4: Out-of-Core Clustering
1. **Step 4.A (The Harness):** Test clustering from disk-backed vectors.
   * *Action:* Add tests in `internal/rpg/cluster_global_test.go` to read vectors from the `GraphProvider` interface instead of a direct struct property.
2. **Step 4.B (The Implementation):** Refactor Clustering execution.
   * *Action:* 
     - Modify `cmd/graphdb/main.go` to call `db.GetEmbeddingsOnly()`, which loads *only* the `id` and `embedding` floats into memory (avoiding graph map bloat).
     - Execute the K-Means clustering algorithm.
     - Write the resulting hierarchical Feature nodes and relationships (topology only) back to the provider via `db.UpdateFeatureTopology`, avoiding holding the full graph in RAM.
3. **Step 4.C (The Verification):**
   * *Action:* Run clustering unit tests `go test ./internal/rpg/...`.

#### Phase 5: Resumable Summarization
1. **Step 5.A (The Harness):** Test resumable feature enrichment.
   * *Action:* Update `internal/rpg/enrich_test.go`.
2. **Step 5.B (The Implementation):** Refactor `internal/rpg/enrich.go` to operate directly against the generic `GraphProvider`.
   * *Action:* 
     - Implement a loop in `cmd/graphdb/main.go` using `db.GetUnnamedFeatures(batchSize)`.
     - For each feature, query the provider for its child context (or traverse to functions).
     - Use `snippet.SliceFile()` to fetch file context from disk.
     - Generate LLM prompts, get names/summaries, and write them back to the database via `db.UpdateFeatureSummary`.
3. **Step 5.C (The Verification):** 
   * *Action:* Run `go test ./internal/rpg/...`.

### Testing Strategy
- Unit tests for all new `GraphProvider` operations ensuring queries and updates succeed.
- Use a mock implementation of `GraphProvider` in orchestration tests to guarantee the main pipeline is decoupled from Neo4j.
- Mock the file system to verify `snippet.SliceFile()` lazy loading is functioning correctly.
- Perform a simulated run (mock API calls) of the complete pipeline with a large generated graph to profile heap memory and confirm a flat memory usage pattern (O(1) footprint bounded by batch size).

## 🎯 Success Criteria
- The orchestration pipeline is fully provider-agnostic, interacting only via the `GraphProvider` interface.
- No Neo4j-specific queries (Cypher), logic, or terminology leak into the core orchestration loops in `cmd/graphdb/main.go` or `internal/rpg/`.
- The Go process memory footprint remains flat (bounded by batch sizes, e.g., < 2GB) throughout the entire enrichment pipeline regardless of repository size.
- The system correctly resumes operation from the last uncompleted batch if restarted or after an API failure.
- Zero raw source code string (`content`) retention in Go maps or intermediate JSONL files; all code context is loaded dynamically via `snippet.SliceFile()`.

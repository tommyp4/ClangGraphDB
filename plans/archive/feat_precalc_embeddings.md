# Feature Implementation Plan: Pre-calculated Embeddings & One-Shot Build

## 📋 Todo Checklist
- [x] **Phase 1: Refactoring & Preparation**
    - [x] Create `internal/rpg/text.go` with `NodeToText`.
    - [x] Add unit tests for `NodeToText`.
- [x] **Phase 2: Embedding Phase Implementation**
    - [x] Modify `EmbeddingClusterer` to accept pre-calculated embeddings.
    - [x] Update `cluster_semantic.go` to use the pre-calculated map.
    - [x] Add tests for `EmbeddingClusterer` with pre-calculated embeddings.
- [x] **Phase 3: Integration in `enrich-features`**
    - [x] Update `cmd/graphdb/main.go` (`handleEnrichFeatures`) to pre-calculate embeddings with a progress bar.
    - [x] Pass the map to the `Clusterer`.
- [x] **Phase 4: The "One-Shot" Command**
    - [x] Implement `handleBuildAll` in `cmd/graphdb/main.go`.
    - [x] Add `build-all` case to `main()`.
- [x] **Phase 5: Documentation & Polish**
    - [x] Update `.gemini/skills/graphdb/SKILL.md`.
    - [x] Verify all changes.

## 🔍 Analysis & Investigation
The `enrich-features` command currently "hangs" during the clustering phase because `EmbeddingClusterer` calls `EmbedBatch` on potentially large sets of functions without progress feedback. By moving the embedding process to a dedicated pre-calculation step with a progress bar, we improve UX and observability.

Additionally, the logic for converting a `graph.Node` (Function) to text for embedding is currently embedded inside `cluster_semantic.go` and potentially duplicated. Refactoring this adheres to DRY principles.

Finally, a `build-all` command will streamline the workflow for fresh deployments.

## 📝 Implementation Plan

### Prerequisites
- `internal/rpg` package exists.
- `cmd/graphdb/main.go` exists.

### Step-by-Step Implementation

#### Phase 1: Refactoring & Preparation
1.  **Step 1.A (The Harness):** Define behavior for `NodeToText`.
    *   *Action:* Create `internal/rpg/text_test.go`.
    *   *Goal:* Verify that `NodeToText` correctly formats atomic features or falls back to name/ID.
2.  **Step 1.B (The Implementation):** Implement `NodeToText`.
    *   *Action:* Create `internal/rpg/text.go`.
    *   *Detail:* Extract the logic from `cluster_semantic.go`.
3.  **Step 1.C (The Verification):** Run tests.
    *   *Action:* Run `go test ./internal/rpg/...`.

#### Phase 2: Embedding Phase Implementation
1.  **Step 2.A (The Harness):** Test `EmbeddingClusterer` with pre-computed embeddings.
    *   *Action:* Update `internal/rpg/cluster_semantic_test.go` to include a test case where `PrecomputedEmbeddings` is populated and `Embedder` is nil (or mocked to fail).
2.  **Step 2.B (The Implementation):** Update `EmbeddingClusterer`.
    *   *Action:* Modify `internal/rpg/cluster_semantic.go`.
    *   *Detail:* Add `PrecomputedEmbeddings map[string][]float32` field. Update `Cluster` method to check this map before calling `Embedder`.
3.  **Step 2.C (The Verification):** Run tests.
    *   *Action:* Run `go test ./internal/rpg/...`.

#### Phase 3: Integration in `enrich-features`
1.  **Step 3.A (The Implementation):** Update `handleEnrichFeatures`.
    *   *Action:* Modify `cmd/graphdb/main.go`.
    *   *Detail:*
        *   After extracting atomic features, iterate over all functions.
        *   Generate text for each using `rpg.NodeToText`.
        *   Batch these texts (e.g., 100 at a time) and send to `Embedder`.
        *   Update a new `ui.ProgressBar` during this process.
        *   Store results in a map.
        *   Pass this map to `rpg.EmbeddingClusterer`.

#### Phase 4: The "One-Shot" Command
1.  **Step 4.A (The Implementation):** Add `build-all` command.
    *   *Action:* Modify `cmd/graphdb/main.go`.
    *   *Detail:*
        *   Add `case "build-all": handleBuildAll(os.Args[2:])` in `main`.
        *   Implement `handleBuildAll`.
        *   It should parse common flags (or assume defaults) and sequentially call:
            1.  `handleIngest`
            2.  `handleEnrichFeatures`
            3.  `handleImport`
        *   *Note:* Ensure global flags (like config) are respected.
2.  **Step 4.B (The Verification):**
    *   *Action:* Run `go run cmd/graphdb/main.go build-all --help` (if help is implemented) or just verify it compiles.

#### Phase 5: Documentation & Polish
1.  **Step 5.A (Documentation):** Update SKILL.md.
    *   *Action:* Modify `.gemini/skills/graphdb/SKILL.md`.
    *   *Detail:* Update the "Workflow" section to include the explicit "Embedding" phase and the new `build-all` command.

### Testing Strategy
*   **Unit Tests:** Verify `NodeToText` and `EmbeddingClusterer` logic.
*   **Manual Verification:** Run `build-all` on a small test project (e.g., `test/fixtures`) to ensure the pipeline completes without hanging and progress bars appear.

## 🎯 Success Criteria
*   `enrich-features` shows a progress bar during embedding generation.
*   `build-all` command successfully runs the entire pipeline.
*   Code is refactored with no logic duplication for node text generation.
*   All tests pass.

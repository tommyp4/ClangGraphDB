# Refactor Plan: Remove FileClusterer and Enforce Semantic Clustering

## 📋 Todo Checklist
- [x] Phase 1: Verify Semantic Clustering Logic (Harness)
- [x] Phase 2: Remove Legacy FileClusterer
- [x] Phase 3: Enforce Semantic Clustering in CLI
- [x] Phase 4: Documentation Update
- [x] Final Review and Testing

## 🔍 Analysis & Investigation
**Goal:** Remove the `FileClusterer` implementation and the `--cluster-mode` flag, making `EmbeddingClusterer` the single, mandatory clustering strategy for the `enrich-features` command.

**Findings:**
*   `FileClusterer` is defined in `internal/rpg/cluster.go` and tested in `internal/rpg/cluster_test.go`. These files are to be deleted.
*   `EmbeddingClusterer` (the target) is fully implemented and tested in `internal/rpg/cluster_semantic_test.go` using a deterministic mock, allowing for safe offline verification.
*   `cmd/graphdb/main.go` currently switches between implementations based on a flag. This logic must be simplified.

**Risk Assessment:**
*   **Low Risk:** The semantic clusterer is already implemented and covered by tests.
*   **Dependency:** `EmbeddingClusterer` requires a valid `Embedder`. The `main.go` logic correctly sets this up for the semantic branch; we must ensure this setup code is preserved when removing the switch.

## 📝 Implementation Plan

### Prerequisites
*   Go 1.22+ environment (available).

### Step-by-Step Implementation

#### Phase 1: Verify Semantic Clustering Logic (Harness)
1.  **Step 1.A:** Run existing semantic clustering tests to ensure the target implementation is stable.
    *   *Action:* Run `go test -v internal/rpg/cluster_semantic.go internal/rpg/cluster_semantic_test.go internal/rpg/cluster.go internal/rpg/cluster_test.go internal/graph/schema.go` (or simply `go test -v ./internal/rpg/...`).
    *   *Goal:* Confirm Green state before changes.

#### Phase 2: Remove Legacy FileClusterer
1.  **Step 2.A:** Delete the legacy implementation.
    *   *Action:* Delete `internal/rpg/cluster.go`.
    *   *Action:* Delete `internal/rpg/cluster_test.go`.
2.  **Step 2.B:** Verify compilation (expect failure in `main.go`).
    *   *Action:* Run `go build ./cmd/graphdb` (Expect error: `rpg.FileClusterer undefined`).

#### Phase 3: Enforce Semantic Clustering in CLI
1.  **Step 3.A:** Refactor `cmd/graphdb/main.go`.
    *   *Action:* Remove the `cluster-mode` flag definition.
    *   *Action:* Remove the switch statement.
    *   *Action:* Unconditionally initialize `rpg.EmbeddingClusterer` with the configured embedder.
    *   *Detail:* Ensure `setupEmbedder` is called and passed to the clusterer.
2.  **Step 3.B:** Verify Build.
    *   *Action:* Run `go build ./cmd/graphdb`.
    *   *Success:* Build passes.

#### Phase 4: Documentation Update
1.  **Step 4.A:** Update Master Roadmap.
    *   *Action:* Modify `plans/00_MASTER_ROADMAP.md` under Campaign 3.6 to reflect the removal of the flag and the enforcement of semantic clustering.

### Testing Strategy
*   **Unit Tests:** Run `go test -v ./internal/rpg/...` to ensure the remaining `EmbeddingClusterer` tests pass and no regression was introduced by removing `FileClusterer` (though they should be independent).
*   **Build Verification:** `go build ./cmd/graphdb` ensures the main entry point is correctly wired.

## 🎯 Success Criteria
1.  `internal/rpg/cluster.go` and `internal/rpg/cluster_test.go` are removed.
2.  `cmd/graphdb` compiles without errors.
3.  The `-cluster-mode` flag is gone from `enrich-features`.
4.  Semantic clustering is the only path.

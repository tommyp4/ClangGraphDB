# Plan: Global Semantic Discovery (Latent Architecture Recovery)

**Status:** Active
**Context:** The current RPG implementation relies on directory-based domain discovery (`DirectoryDomainDiscoverer`), which violates the RPG-Encoder architecture. The paper specifies "Latent Architecture Recovery" via global clustering of embeddings, independent of file structure.
**Objective:** Implement `GlobalClusterer` to group all repository functions by semantic similarity (embeddings) into "Latent Domains," then ground them to physical directories using LCA (Lowest Common Ancestor).

## 1. Architecture Update

### New Component: `GlobalClusterer`
*   **Input:** All repository functions (Nodes).
*   **Mechanism:**
    1.  **Global Pooling:** Flatten the file tree into a single list of semantic units.
    2.  **Embedding:** Ensure all units have embeddings (use `Embedder`).
    3.  **Clustering:** Apply K-Means (or HDBSCAN) to partition the global embedding space into $K$ clusters.
    4.  **Labeling:** Generate a semantic name for each cluster (e.g., "DataProcessing") using the `Summarizer` or centroid terms.
*   **Output:** `map[string][]graph.Node` (Domain Name -> Functions).

### Modified Component: `Builder`
*   **Logic Switch:** If `GlobalClusterer` is provided:
    1.  Call `GlobalClusterer.Cluster(allFunctions)`.
    2.  For each Cluster:
        *   Calculate `LCA` (Lowest Common Ancestor) of its file paths.
        *   Create a "Domain Feature" rooted at that LCA.
        *   (Optional) If LCA is root, keeping it is fine; if LCA is specific (e.g., `src/auth`), it grounds the abstract domain.
        *   Proceed with standard hierarchy construction (Categories/Features) *within* that domain.

## 2. Implementation Steps

### Step 1: Implement `GlobalEmbeddingClusterer`
Create `internal/rpg/cluster_global.go`.
*   Struct `GlobalEmbeddingClusterer` implementing `Clusterer`.
*   Logic:
    *   Reuse `EmbeddingClusterer` logic but apply it to the entire dataset.
    *   Determine $K$ dynamically (e.g., `Sqrt(N/2)` or `Silhouette Score` heuristic).
    *   **Naming:** Use `Summarizer` to name the cluster based on its top 5 central nodes.

### Step 2: Implement LCA Grounding
Update `internal/rpg/paths.go` (if not already present).
*   Ensure `FindLowestCommonAncestor(paths []string) string` is robust.
*   Handle edge cases: empty list, no common prefix, windows paths.

### Step 3: Wire into `Builder`
Modify `internal/rpg/builder.go`.
*   The `buildGlobal` method exists but needs to use the real `GlobalClusterer`.
*   Ensure the pipeline: `Ingest` -> `Embed` -> `Global Cluster` -> `Build Graph`.

### Step 4: CLI Integration
Update `cmd/graphdb/main.go` -> `handleEnrichFeatures`.
*   **Standardize:** Instantiate `GlobalEmbeddingClusterer` as the default discoverer.
*   **Simplify:** Do not add a `--discovery-mode` flag. The RPG architecture requires global semantic discovery; directory-based discovery was merely a prototype placeholder and should be replaced.

## 3. Verification
*   **Unit Test:** `builder_global_test.go` (Update to use real logic if possible, or add `cluster_global_test.go`).
*   **E2E:** Run on a repo with scattered functionality (e.g., `auth` logic in `utils` and `controllers`).
    *   *Expectation:* The "Authentication" domain captures functions from both folders.
    *   *Expectation:* The Domain Node is grounded at `src/` (or the common root).

## 4. Dependencies
*   Requires `Embedder` (Vertex AI) to be fully functional.
*   Requires `fix_missing_content_in_nodes.md` to be done first (so we can summarize clusters to name them).

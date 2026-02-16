# RPG Construction Architecture & Seam Analysis

## 1. System Architecture Map

The RPG (Repository Planning Graph) construction is orchestrated by `rpg.Builder`. It transforms raw code nodes (Functions) into a hierarchical feature map.

### Core Components

*   **`rpg.Builder`** (`internal/rpg/builder.go`)
    *   **Role:** Orchestrator.
    *   **Dependencies:**
        *   `Discoverer` (`DomainDiscoverer` interface): Identifies high-level domains (e.g., "auth", "payment").
        *   `Clusterer` (`Clusterer` interface): Groups functions within a domain into features.
        *   `CategoryClusterer` (Optional `Clusterer`): Groups functions into categories before features (3-level hierarchy).
        *   `GlobalClusterer` (Optional `Clusterer`): **Critical Path.** If present, it overrides the standard discovery flow.

*   **`DomainDiscoverer`** (`internal/rpg/discovery.go`)
    *   **Implementation:** `DirectoryDomainDiscoverer`.
    *   **Mechanism:** Scans the file system (FileSystem Walk) to find subdirectories in `BaseDirs`.
    *   **Logic:** Maps subdirectory names to domains.
    *   **Implicit Link:** Duplicates `.gitignore` logic found in `ingest/walker.go` but less robustly (only checks root `.gitignore`).

*   **`Clusterer`** (`internal/rpg/cluster_semantic.go`)
    *   **Implementation:** `EmbeddingClusterer`.
    *   **Mechanism:** K-Means clustering on embedding vectors.
    *   **Dependencies:** `embedding.Embedder` (interface), `PrecomputedEmbeddings` (map).
    *   **Logic:** Groups functions by semantic similarity.

### Data Flow

1.  **Ingest:** `cmd/graphdb` -> `ingest.Walker` -> `graph.jsonl`.
2.  **Enrich:** `cmd/graphdb` -> `enrich-features`.
    *   Loads `graph.jsonl`.
    *   **Pre-calculates Embeddings:** Calls `Embedder` for all functions.
    *   **Instantiates Builder:** Injects `EmbeddingClusterer` with `PrecomputedEmbeddings`.
    *   **Builds RPG:** `Builder.Build()`.
3.  **Persist:** Saves `rpg.jsonl`.

## 2. Seam Identification

### Primary Seam: `GlobalClusterer` Integration
The `rpg.Builder` struct exposes a clear seam for global clustering:
```go
type Builder struct {
    // ...
    GlobalClusterer Clusterer
    // ...
}
```
*   **Current State:** If `GlobalClusterer` is non-nil, `Builder.Build` executes `buildGlobal`, which **completely bypasses** `DomainDiscoverer`.
*   **Implication:** You cannot currently have *both* Directory-based discovery and Global Clustering (Latent Discovery). It is an "Either/Or" architecture.
*   **Risk:** If `GlobalEmbeddingClusterer` is intended to catch "unassigned" functions (Latent Domains) *after* directory discovery, the current `Builder.Build` logic blocks this.

### Secondary Seam: Data Injection
`EmbeddingClusterer` is stateful. It requires `PrecomputedEmbeddings`.
*   **Seam:** The `enrich-features` command (in `main.go`) acts as the factory, pre-calculating embeddings and injecting them.
*   **Constraint:** The `Builder` cannot lazily fetch embeddings; they must be pre-loaded. This creates a high memory footprint for large repositories.

### Implicit Logic Seam: File Ignoring
*   **`ingest/walker.go`:** Implements recursive `.gitignore` parsing.
*   **`rpg/discovery.go`:** Implements root-only `.gitignore` parsing.
*   **Conflict:** `DomainDiscoverer` may identify a domain (directory) that `Walker` ignored. The result is an empty domain in the RPG ("Ghost Domain").

## 3. GlobalEmbeddingClusterer Integration Strategy

The "GlobalEmbeddingClusterer" (likely a specialized `EmbeddingClusterer`) should be integrated at the `GlobalClusterer` field of the `Builder`.

### Gap Analysis
To support the campaign goal of "Latent Domain Discovery" (catching items missed by directory structure), the integration requires **Architecture Refactoring** in `Builder.Build`.

**Current Logic:**
```go
if b.GlobalClusterer != nil {
    return b.buildGlobal(...) // Exits here, ignoring Discoverer
}
// ... proceed with Discoverer
```

**Required Logic (Hybrid Mode):**
1.  Run `Discoverer` to find explicit domains.
2.  Assign matching functions to these domains.
3.  Identify `unassignedFunctions`.
4.  Run `GlobalClusterer` (as `LatentClusterer`) on `unassignedFunctions`.
5.  Merge results.

## 4. Recommendations for Architect

1.  **Refactor `Builder.Build` for Hybrid Discovery:**
    *   Change the logic from "Exclusive Global" to "Fallback Global".
    *   Rename `GlobalClusterer` to `LatentClusterer` to clarify intent (handling the unknown).
    *   Allow the builder to run directory discovery first, then pass leftovers to the latent clusterer.

2.  **Unify GitIgnore Logic:**
    *   Extract `walker.go`'s recursive gitignore logic into a shared helper in `internal/tools`.
    *   Update `DirectoryDomainDiscoverer` to use this shared helper to prevent "Ghost Domains".

3.  **Formalize `GlobalEmbeddingClusterer`:**
    *   Create a distinct type (or factory method) for the global clusterer in `cluster_semantic.go`.
    *   Rationale: Global clustering might require different algorithms (e.g., HDBSCAN) or parameters (different `KStrategy`) than local feature clustering. The current generic `EmbeddingClusterer` may be too simplistic for top-level domain discovery.

# Research Report: RPG Discovery & Clustering Analysis

## 1. DirectoryDomainDiscoverer Analysis
**File:** `internal/rpg/discovery.go`

The `DirectoryDomainDiscoverer` is the current mechanism for identifying high-level "Domains" in the codebase.
- **Mechanism:** It iterates through a list of `BaseDirs`. For each base directory, it lists its *subdirectories*. Each subdirectory becomes a "Domain".
- **Restriction (Critical):** The loop explicitly skips non-directory entries (`if !entry.IsDir() { continue }`). 
  - **Consequence:** Source files located directly in the root of `BaseDirs` (e.g., `main.go` in root) are **never** assigned to a domain and are thus effectively invisible to the RPG builder.
- **Path Restriction:** It relies on strict directory structure. It assumes that architecture 1:1 maps to folder structure.

## 2. Builder.Build Flow Analysis
**File:** `internal/rpg/builder.go`

The `Builder.Build` method orchestrates the graph construction.
- **Current Flow:**
  1. `Discoverer.DiscoverDomains` returns a list of domain paths.
  2. Iterates over each domain.
  3. **Filters** the input `functions` list: A function is included in a domain **only if** its file path starts with the domain's path prefix.
  4. Calls `Clusterer` on the filtered functions.
- **Gap (Data Loss):** Any function that does not reside within one of the discovered domain directories is **discarded**. There is no "catch-all" or "global" bucket.
- **Injection Points for Campaign 3.7:**
  - **Global Pooling:**
    - *Location:* At the start of `Build`.
    - *Action:* Initialize a set of "Unclaimed Functions" containing all input nodes.
    - *Logic:* As functions are matched to domains in the loop, remove them from the "Unclaimed" set.
  - **Latent Domain Discovery:**
    - *Location:* After the domain processing loop.
    - *Action:* Take the remaining "Unclaimed Functions".
    - *Logic:* Pass them to a new `LatentDiscoverer` (likely using `EmbeddingClusterer` globally) to group them.
    - *Naming:* Use LCA (Lowest Common Ancestor) of the file paths in each latent cluster to name it (e.g., "latent-internal-utils").

## 3. LCA Utility Search
**Findings:**
- **Status:** **MISSING**.
- **Search:** Grepped for "CommonPrefix", scanned `internal/rpg/discovery.go` and `internal/rpg/text.go`.
- **Result:** No existing utility function calculates the Lowest Common Ancestor or common path prefix for a set of file paths.
- **Recommendation:** A new utility `FindLowestCommonAncestor(paths []string) string` is required for naming Latent Domains meaningfully.

## 4. Recommendations for Architect
1.  **Fix Root File Exclusion:** Modify `DirectoryDomainDiscoverer` to either include a "." domain for root files or handle them explicitly.
2.  **Implement Global Pooling:** Refactor `Builder.Build` to track `unassigned_nodes`. This is the prerequisite for Latent Domain Discovery.
3.  **Create LCA Utility:** Implement a path helper to derive names for latent clusters.
4.  **Inject Latent Discovery:** Add a phase after explicit domain processing to cluster the `unassigned_nodes`.

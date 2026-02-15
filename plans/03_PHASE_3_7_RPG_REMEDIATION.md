# Feature Implementation Plan: Campaign 3.7 (Global Semantic Topology)

## 📋 Todo Checklist
- [x] **LCA Utility:** Implement `FindLowestCommonAncestor` in `internal/rpg/paths.go`.
- [x] **Semantic Namer:** Implement `NamingStrategy` for labeling latent domains.
- [x] **Builder Refactor:** Update `rpg.Builder` to support "Global Discovery Mode" (Inverted Flow).
- [x] **Integration:** Wire up `EmbeddingClusterer` as the Global Domain Discoverer in `main.go`.
- [x] **Verification:** Verify that root-level files and scattered dependencies are correctly clustered.
- [x] **Deprecation:** Mark `DirectoryDomainDiscoverer` as deprecated.

## 🔍 Analysis & Investigation

### Current Limitations
1.  **Directory-Bound:** The current `DirectoryDomainDiscoverer` strictly maps 1 subdirectory = 1 Domain. This fails to capture:
    *   Files in the root directory (e.g., `main.go`, shared utils).
    *   Cross-cutting concerns scattered across folders (e.g., "Auth" logic in `middleware/` and `services/`).
2.  **Data Loss:** `Builder.Build` filters functions by the discovered domain's path prefix. Functions outside these prefixes are silently discarded.
3.  **Rigid Hierarchy:** The physical folder structure dictates the semantic graph, which contradicts the "Intent Layer" goal of RPG.

### The Solution: Global Semantic Topology
We will invert the control flow:
1.  **Pool:** Treat all functions as a single global set.
2.  **Cluster:** Apply `EmbeddingClusterer` globally to find "Latent Domains" (Functional Groups).
3.  **Ground:** For each group, calculate the Lowest Common Ancestor (LCA) of its file paths to anchor it to the physical file system (ScopePath).
4.  **Name:** Label the domain using the LCA path and/or top semantic terms.

## 📝 Implementation Plan

### Prerequisites
*   Existing `EmbeddingClusterer` must be capable of handling global-scale node lists (1000+ nodes) without O(N^2) issues. (Validated: Uses K-Means, O(N*K*I)).

### Step-by-Step Implementation

#### Phase 1: Core Utilities (The Harness)

1.  **Step 1.A: Define LCA Verification**
    *   *Action:* Create `internal/rpg/paths_test.go`.
    *   *Goal:* Define test cases for `FindLowestCommonAncestor` (e.g., divergent paths, root matches, single file).
2.  **Step 1.B: Implement LCA Logic**
    *   *Action:* Create `internal/rpg/paths.go`.
    *   *Detail:* Implement `FindLowestCommonAncestor(paths []string) string`. It should return the longest common directory prefix.
3.  **Step 1.C: Verify LCA**
    *   *Action:* Run `go test ./internal/rpg/...`.

#### Phase 2: Domain Naming Strategy

1.  **Step 2.A: Define Naming Interface**
    *   *Action:* Update `internal/rpg/cluster_semantic.go` or create `naming.go`.
    *   *Detail:* Define `DomainNamer` interface or utility function.
    *   *Logic:*
        *   Primary: Use LCA path (e.g., `internal/auth`).
        *   Fallback (if LCA is root): Use "Top Frequency Term" from atomic features (e.g., "login", "user").
        *   Format: `domain-<lca>-<term>` or just `domain-<term>` if LCA is generic.
        *   *Refinement:* For this phase, we will rely on a robust `GenerateDomainName(lca string, nodes []Node) string` function.

#### Phase 3: Builder Refactoring (The Inversion)

1.  **Step 3.A: Update Builder Struct**
    *   *Action:* Modify `internal/rpg/builder.go`.
    *   *Detail:*
        *   Add `GlobalClusterer Clusterer` field.
        *   Add `GlobalDiscovery bool` flag (or check if `Discoverer` is nil).
2.  **Step 3.B: Implement Global Build Flow**
    *   *Action:* Refactor `Build` method.
    *   *Logic:*
        ```go
        if b.GlobalClusterer != nil {
            // 1. Global Clustering (Latent Domains)
            domainMap, _ := b.GlobalClusterer.Cluster(functions, "root")
            
            for domainName, nodes := range domainMap {
                // 2. Grounding (LCA)
                filePaths := ExtractFilePaths(nodes)
                lca := FindLowestCommonAncestor(filePaths)
                
                // 3. Identification
                finalName := GenerateDomainName(lca, nodes) // e.g. "auth-service"
                
                // 4. Standard Construction (Feature Clustering)
                // Reuse existing buildTwoLevel/buildThreeLevel logic, passing 'nodes' directly
                // (Skipping the path-prefix filtering step!)
                b.buildHierarchy(...)
            }
        } else {
            // Legacy Path ...
        }
        ```
    *   *Critical Change:* The legacy loop filters `functions` by path. The new loop *already has* the specific nodes for the domain, so it skips filtering.

#### Phase 4: Integration & Cleanup

1.  **Step 4.A: Wire Main**
    *   *Action:* Modify `cmd/graphdb/main.go`.
    *   *Detail:*
        *   Initialize `EmbeddingClusterer` for the global level (possibly with `k=sqrt(N/10)`).
        *   Inject it into `Builder`.
        *   Ensure `DirectoryDomainDiscoverer` is NOT used when this mode is active.
2.  **Step 4.B: Deprecate Legacy Discovery**
    *   *Action:* Add deprecation notice to `DirectoryDomainDiscoverer` in `internal/rpg/discovery.go`.

### Testing Strategy
*   **Unit Tests:** Verify LCA logic and Naming logic.
*   **E2E Test:**
    *   Use `test/fixtures/go/` (or create a dispersed fixture).
    *   Run `graphdb build`.
    *   Inspect `trace-viewer.html` or output JSONL.
    *   *Success Criteria:* Root-level files are included in a domain. "Auth" logic scattered across folders is grouped into a single Semantic Domain.

## 🎯 Success Criteria
1.  **No Unassigned Functions:** All ingested functions appear in the graph (checking `stats` in log).
2.  **Semantic Grouping:** Functions with similar atomic features (e.g., "Login", "Logout") are in the same Domain, even if in different folders.
3.  **Physical Grounding:** Domains have a valid `ScopePath` pointing to their LCA.

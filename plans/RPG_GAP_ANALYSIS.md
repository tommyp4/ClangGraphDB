# RPG Implementation Gap Analysis

## 🚨 Executive Summary
The current `graphdb` skill implementation structurally violates the core tenets of the "Repository Planning Graph" (RPG) research. While it successfully implements the mechanics of embeddings and K-Means clustering, it applies them within strict **physical directory silos**. This defeats the primary goal of RPG: to discover latent semantic topology that bridges disparate parts of a disorganized codebase (e.g., linking a Stored Procedure to a Frontend Component based on intent).

Currently, the tool acts as a **"Semantic Folder Viewer"**, not a true **"Repository Planning Graph"**.

## 🔴 Core Violation: "Directory-Bound" vs. "Semantic-First"

The RPG-Encoder paper explicitly states:
> *"Physical folder-file organization is often dictated by technical constraints... inducing structural entanglement. To mitigate this, we construct the High-level Node set by recovering the latent functional topology... excluding function-level details."*

### The Reality (Current Implementation)
The `Builder.Build` process enforces **Top-Down Physical Filtering** *before* Semantic Analysis begins.

1.  **Physical Discovery (`discovery.go`):** The system first scans the file system to identify physical top-level directories (e.g., `src/auth`, `db/scripts`).
2.  **Hard Filtering (`builder.go`):** It iterates through these physical domains and **excludes** any node that does not match the file path prefix.
    ```go
    // internal/rpg/builder.go
    if pathPrefix == "" || p == pathPrefix || strings.HasPrefix(p, pathPrefix+"/") {
        domainFuncs = append(domainFuncs, fn)
    }
    ```
3.  **Siloed Clustering (`cluster_semantic.go`):** K-Means clustering is performed *only* on the filtered subset of nodes within that specific directory.

**The Consequence:**
A stored procedure in `sql/verify_user.sql` and a service in `src/auth_service.ts` are physically separated. The current logic places them in separate "Domains" **before** the embedding model ever sees them. They will never be clustered together, making cross-stack modernization planning impossible.

## 🔍 Detailed Gap Analysis

| Feature | Research Paper Vision (Spirit) | Current Implementation (Reality) |
| :--- | :--- | :--- |
| **Domain Discovery** | **Latent Topology:** "Induce abstract functional centroids (e.g., Data Preprocessing) that define the root pillars." | **Physical Mirror:** Iterates top-level directories (`DiscoverDomains`). Domains are just folder names. |
| **Membership** | **Semantic Compatibility:** "Node's placement is determined by a semantic compatibility check... bridging the hierarchy." | **Path Prefix:** Nodes are assigned to domains strictly by file path string matching (`strings.HasPrefix`). |
| **Artifact Grounding** | **Output (Post-Process):** "Populate missing metadata... utilizing a Lowest Common Ancestor (LCA) mechanism." | **Input (Pre-Process):** Metadata (directory path) is used as the *input filter* to define the cluster. |
| **Semantic Lifting** | **Abstraction:** "Convert implementation details into atomic semantic features (verb-object)." | **Partial:** `NodeToText` supports `atomic_features` but falls back to raw ID/Name if parsing is missing. |

## 🛠️ Technical Root Cause
The architectural flaw lies in `internal/rpg/builder.go`. The control flow assumes that **Physical Structure = Semantic Structure**.

```go
// Current Control Flow
func (b *Builder) Build(...) {
    // 1. Find Physical Folders
    domains, _ := b.Discoverer.DiscoverDomains(rootPath) 
    
    for _, name := range domainNames {
        // 2. Filter Nodes by Path
        var domainFuncs []graph.Node
        for _, fn := range functions {
             if strings.HasPrefix(fn.Path, domains[name]) { ... }
        }
        
        // 3. Cluster ONLY within this folder
        b.Clusterer.Cluster(domainFuncs)
    }
}
```

## 💡 Recommendation for Refactoring

To align with the spirit of RPG, the **Order of Operations** must be inverted. We must move from **"Filter -> Cluster"** to **"Cluster -> Ground"**.

### Proposed Architecture: "The Global Semantic Pool"

1.  **Global Pooling:** Do **not** pre-filter by directory. Feed *all* repository nodes (SQL, Java, Python, Configs) into the `CategoryClusterer` simultaneously.
2.  **Latent Domain Discovery:** 
    *   Use `EmbeddingClusterer` (K-Means) on the global pool to find high-level groups (Centroids) based purely on the vector space.
    *   *Result:* A cluster (e.g., "Cluster-1") might contain `verify_user.sql`, `auth_service.ts`, and `login_form.jsx`.
3.  **Semantic Labeling:** Use the centroid's nearest neighbor or an LLM to name "Cluster-1" -> "Identity Management".
4.  **Post-Hoc Grounding (LCA):** 
    *   *After* the cluster is formed, calculate its "Physical Scope" by finding the common directory prefix of its members (using the LCA algorithm described in the paper).
    *   This "Scope" becomes a property of the Domain Node for navigation, but it does *not* constrain membership.

### Migration Steps
1.  **Deprecate `DirectoryDomainDiscoverer`:** It reinforces the physical bias.
2.  **Create `GlobalClusterer`:** A new implementation of `Builder` that accepts the full node list and performs global K-Means first.
3.  **Implement LCA Grounding:** A utility to find the physical root of a semantic cluster *after* it has been created.

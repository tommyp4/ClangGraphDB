# RPG Semantic Clustering & Domain Discovery Logic

This document details the algorithms and heuristics used by the **GraphDB Skill** to discover high-level "Domains" and "Features" from raw source code.

## 1. The Core Philosophy

The Repository Planning Graph (RPG) avoids rigid directory-based grouping. Instead, it uses **Global Semantic Clustering** to find functional groups ("Latent Domains") based on the vector embeddings of function signatures and bodies.

## 2. Domain Discovery (Level 1)

**Goal:** Identify the high-level bounded contexts of the application (e.g., "Authentication", "Payment Processing", "Utility").

*   **Location:** `cmd/graphdb/main.go` (Configuration) & `internal/rpg/cluster_global.go` (Implementation).
*   **Scope:** Global (All functions in the repository).

### The Formula
The number of domains ($k$) is determined dynamically based on the size of the codebase:

$$ k = \sqrt{\frac{N_{total}}{10}} $$

Where $N_{total}$ is the total number of functions in the repository.

*   **Constraints:**
    *   Minimum $k = 2$ (if $N > 3$).
    *   **Example:** A repo with **1,000 functions** will generate $\approx \textbf{10 domains}$.
    *   **Example:** A repo with **100 functions** will generate $\approx \textbf{3 domains}$.

### The Algorithm
1.  **Global Pooling:** All function embeddings are loaded into a single pool.
2.  **K-Means++:** We run K-Means clustering with the calculated $k$.
    *   **Metric:** Cosine Distance ($1 - Similarity$).
    *   **Initialization:** K-Means++ (optimizes initial center placement to speed up convergence).
3.  **Naming:**
    *   The system identifies the **Centroid** (geometric center) of each cluster.
    *   It selects the "Representative Nodes" (functions closest to the center).
    *   An LLM (Vertex AI) analyzes these representatives to generate a domain name (e.g., "User Management").

## 3. Feature Discovery (Level 2)

**Goal:** Break down each Domain into granular "Features" (e.g., within "Authentication", find "Login", "Logout", "Password Reset").

*   **Location:** `internal/rpg/cluster_semantic.go`.
*   **Scope:** Local (Functions within a specific Domain).

### The Formula

For sub-clustering, a denser strategy is used to ensure cohesive features:



$ k = \text{clamp}\left( \frac{N_{subset}}{5}, \ 2, \ \frac{N_{subset}}{2} \right) $

*   **Target:** Approximately **5 functions per Feature**.
*   **Behavior:**
    *   Small clusters ($N \le 5$) are kept as a single feature.
    *   Larger clusters are broken down until the target granularity is reached.

## 4. Code References

*   **Entry Point Configuration:** `cmd/graphdb/main.go` (`handleEnrichFeatures` function).
*   **Global Clustering Wrapper:** `internal/rpg/cluster_global.go`.
*   **Core Algorithm:** `internal/rpg/cluster_semantic.go` (`EmbeddingClusterer`).

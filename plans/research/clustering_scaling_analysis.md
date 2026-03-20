# Research: RPG Semantic Clustering Scaling & Domain Naming Resolution

**Date:** March 19, 2026
**Context:** Analysis of the RPG (Repository Planning Graph) semantic clustering behavior, specifically investigating why the pipeline generated overlapping, generic domain names (e.g., "Driver Compensation", "Driver Compensation Management 1", etc.) for a small but modular repository (`trucks-v2`), and how to balance this with the need to scale to massive enterprise repositories (1.7M+ LOC, 230k+ functions).

---

## 1. The Core Observation (`trucks-v2` issue)
When running the GraphDB skill on `trucks-v2` (approx. 145 files), the system successfully executed K-Means clustering but failed to accurately name the domains, resulting in 4 variations of "Driver Compensation". 

Manual inspection of `trucks-v2` reveals it contains distinct operational modules (Fuel Management, Toll Processing, Settlement/Ledger, PDF Generation, Excel Conversions). The system failed to identify these due to a combination of algorithmic constraints, sampling methods, and prompt phrasing.

## 2. Root Cause Analysis

### A. Divergence from Design Constraints (The Math)
The implementation in `orchestrator.go` diverged from the original `RPG_CLUSTERING.md` design:
*   **Formula:** The design specified $k = \sqrt{N_{functions} / 10}$. The code implemented $k = \sqrt{N_{files} / 5.0}$. 
*   **The Floor:** The design allowed a minimum of $K=2$. The code enforced a hardcoded minimum of `if k < 5 { return 5 }`.
*   **Impact:** By forcing $K \ge 5$ on a cohesive, small repository, K-Means mathematically sliced a single monolith into 5 highly overlapping chunks.

### B. The "Mushy Centroid" Sampling Problem
When the system asks the LLM to name a cluster, it samples the 5 functions closest to the geometric center (Centroid) of the cluster.
*   In highly coupled codebases, the functions at the exact center are usually generic base models, shared utilities, or abstract classes (e.g., `Driver.cs`).
*   The diverse, specific implementations (e.g., `FuelController`, `BestpassClient`) live on the *edges* of the cluster.
*   **Impact:** The LLM only sees the generic core models and correctly, but repeatedly, guesses the overarching theme ("Driver Compensation") instead of the specific module's intent.

### C. The Domain-Driven Design (DDD) Prompt Limitation
The prompt in `enrich.go` instructs the LLM to find the "Bounded Context (business domain)". 
*   For a 1.7M LOC repo, this correctly yields "Billing" vs "Fulfillment".
*   For a 145-file app, the *entire app* is a single Bounded Context. Asking the LLM to zoom out that far ensures it overlooks the smaller feature modules (like PDF generation).

## 3. The Scaling Reality (Why the Implementation Diverged)

The original whitepaper (`RPG-Encoder.pdf`) dictates an LLM-first, top-down approach: feed all semantic features to the LLM and let it organically define the domains, *then* assign code to them. 

**Why this was rightly abandoned for a Math-First approach:**
When facing 1.7M LOC and ~230,000 functions, the whitepaper's approach shatters:
1.  **Context Window & Token Cost:** 230k functions equates to millions of tokens just for the summaries. Feeding this into a single prompt causes catastrophic context loss, massive API costs, and latency timeouts.
2.  **HAC Memory Limits:** Standard Hierarchical Agglomerative Clustering (HAC) requires a pairwise distance matrix. For 230k functions, this matrix contains $\approx 26.4$ Billion distances, requiring >100GB of RAM and crashing the Go binary via OOM.

The current implementation (K-Means on embeddings -> sample -> ask LLM) was a necessary and highly effective survival mechanism to process enterprise-scale code in seconds. The flaw is not the mechanism, but its lack of elasticity for smaller codebases.

---

## 4. Ideation: Strategies for Elastic Scaling

To fix the small-repo clustering without breaking the 1.7M LOC scaling, the system needs to become **Scale-Aware** and **Context-Rich**.

### Strategy 1: Prompt & Sampling Evolutions (High Impact, Low Effort)
1.  **Scale-Aware Prompting:** Pass the repo size into the prompt.
    *   *Massive repos (>50k functions):* Ask for "Bounded Contexts / Business Domains".
    *   *Small repos (<5k functions):* Ask for "Functional Sub-systems / Feature Modules".
2.  **Edge-Aware Sampling:** Instead of taking the 5 functions closest to the centroid, take 2 from the center (core) and 3 from the edges (specific implementations).
3.  **Provide File Paths:** Inject the file paths of the sampled functions into the prompt. A path like `trucks/Bestpass/BestpassClient.cs` gives the LLM massive context that a raw function signature might lack.
4.  **Hierarchical Prompting (Contextual Naming):** Pass the previously generated domain names into the prompt (e.g., *"You have already named domains A and B. Name this new group, ensuring it is distinct."*).

### Strategy 2: Algorithmic Evolutions (Medium Impact, Medium Effort)
1.  **Adjust the $K$ Bounds:** Revert the hardcoded floor from 5 down to 1 or 2, allowing small monolithic apps to resolve naturally without forced fracturing.
2.  **Dynamic $K$ via Silhouette Score:** Instead of guessing $K$, calculate a range based on file count, run K-Means multiple times, and use the Silhouette Score to automatically pick the $K$ that yields the tightest clusters.

### Strategy 3: Hybrid Hierarchical Clustering (High Effort, Enterprise Grade)
To achieve the whitepaper's hierarchical tree without the $O(N^2)$ memory crash:
1.  **Bisecting K-Means (Top-Down):** Start with 1 cluster. Run K-Means ($K=2$). Pick the child with the highest variance and split it again. Stop splitting when variance drops below a threshold. This organically finds 2 domains for `trucks-v2` and 50 domains for a 1.7M LOC repo.
2.  **Two-Tiered Clustering (Bottom-Up):** Run K-Means with a massive $K$ (e.g., 2000) to create dense micro-clusters. Then run standard HAC *only on the 2000 centroids*. This builds a perfect hierarchical tree instantly using minimal memory.

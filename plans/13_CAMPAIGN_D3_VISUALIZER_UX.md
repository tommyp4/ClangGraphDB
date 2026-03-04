# Campaign 13: D3 Visualizer UX & Systemic Clustering Fixes

This campaign addresses critical user feedback regarding the initial implementation of the D3 Visualizer and exposes a systemic issue in the clustering logic.

## 🚨 Priority 1: Systemic Clustering Failure ("Unknown Feature-cluster-X")
*   **The Issue:** The graph currently renders top-level domains as `Unknown Feature-cluster-7`, etc. This indicates a fundamental breakdown in the semantic clustering or domain discovery logic (likely related to how LLM features are synthesized or how `DomainExplorationResult` constructs the overview).
*   **Action:** Investigate the `RPG` intent layer build step. We must determine why domain names are missing/unknown and fix the backend ingest/enrichment pipeline so the graph accurately reflects semantic domain names.

## 🎨 Priority 2: Visualizer UX Polish

1. **Intelligent Viewport Controls:**
   - **Issue:** Expanding a node (double-click) doesn't adjust the camera.
   - **Fix:** Implement automatic viewport centering/zooming when double-clicking a cluster so the newly expanded neighborhood is visible.

2. **Modernize Aesthetics:**
   - **Issue:** The UI is currently "bland" (raw HTML/CSS).
   - **Fix:** Integrate TailwindCSS (or similar modern styling) to make the interface look polished and modern.
   - **Goal:** Leverage D3's power to intuitively visualize system decomposition, dependency relationships, and "what-if" extractions cleanly.

3. **Node Inspection & Details:**
   - **Issue:** Single-clicking or hovering on nodes provides no information.
   - **Fix:** Implement an inspection panel or rich tooltips. Clicking a node should reveal:
     - Node type (Function, Class, etc.)
     - File path
     - Associated semantic domains
     - Volatility scores/metrics

4. **Search Box Clarity:**
   - **Issue:** The purpose of the "Search" box is unclear.
   - **Fix:** Redesign the search input to resemble a modern search bar (e.g., Google-style). Add clear placeholder text or surrounding copy explaining *what* can be searched (e.g., "Describe a feature in natural language to find its implementation...").

5. **Analytical Tooltips:**
   - **Issue:** Domain-specific terms like "Show Pinch Points" and "Semantic Disconnects" are jargon-heavy.
   - **Fix:** Add explanatory tooltips or visually accessible descriptions to these controls so users understand their analytical value (e.g., "Highlights structural bottlenecks where many domains converge").
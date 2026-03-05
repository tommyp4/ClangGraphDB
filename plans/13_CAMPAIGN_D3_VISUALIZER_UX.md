# Campaign 13: D3 Visualizer UX

This campaign addresses critical user feedback regarding the initial implementation of the D3 Visualizer, focusing purely on frontend enhancements and usability improvements.

## 🎨 Priority 1: Visualizer UX Polish

1. **Intelligent Viewport Controls:**
   - **Issue:** Expanding a node (double-click) doesn't adjust the camera.
   - **Fix:** Implement automatic viewport centering/zooming in `internal/ui/web/app.js` when double-clicking a cluster so the newly expanded neighborhood is visible.

2. **Modernize Aesthetics:**
   - **Issue:** The UI is currently "bland" (raw HTML/CSS).
   - **Fix:** Integrate TailwindCSS (or similar modern styling) to make the interface look polished and modern. **The Engineer MUST use the provided HTML/Tailwind template from `./stitch/graphdb_main_visualizer/code.html` (referencing `screen.png`) to update the HTML structure in `internal/ui/web/index.html`.** The logic must remain separated in `internal/ui/web/app.js`.
   - **Goal:** Leverage D3's power to intuitively visualize system decomposition, dependency relationships, and "what-if" extractions cleanly.

3. **Node Inspection & Details:**
   - **Issue:** Single-clicking or hovering on nodes provides no information.
   - **Fix:** Implement an inspection panel or rich tooltips. **The Engineer MUST use the provided HTML/Tailwind template from `./stitch/blast_radius_analysis/code.html` (referencing `screen.png`) to update `internal/ui/web/index.html` and wire the logic in `internal/ui/web/app.js`.** Clicking a node should reveal:
     - Node type (Function, Class, etc.)
     - File path
     - Associated semantic domains
     - Volatility scores/metrics

4. **Search Box Clarity:**
   - **Issue:** The purpose of the "Search" box is unclear.
   - **Fix:** Redesign the search input to resemble a modern search bar (e.g., Google-style) in `internal/ui/web/index.html`. **Reference the mock in `./stitch/graphdb_main_visualizer/code.html` and `screen.png`.** Add clear placeholder text or surrounding copy explaining *what* can be searched (e.g., "Describe a feature in natural language to find its implementation...").

5. **Analytical Tooltips:**
   - **Issue:** Domain-specific terms like "Show Pinch Points" and "Semantic Disconnects" are jargon-heavy.
   - **Fix:** Add explanatory tooltips or visually accessible descriptions to these controls in `internal/ui/web/index.html` so users understand their analytical value (e.g., "Highlights structural bottlenecks where many domains converge"). **Reference the mock in `./stitch/blast_radius_analysis/code.html` and `screen.png`.**
# Feature Implementation Plan: Campaign 13 D3 Visualizer UX

## 📋 Todo Checklist
- [x] **MANDATORY Recovery Step:** Restore `trace-viewer.html` ✅ Implemented
- [x] Phase 1: Modernize Aesthetics & Search Box (HTML/Tailwind) ✅ Implemented
- [x] Phase 1: Node Inspection, Details & Analytical Controls (HTML/Tailwind) ✅ Implemented
- [x] Phase 1: Integrate and wire up D3.js logic (app.js) ✅ Implemented
- [x] Phase 1: Intelligent Viewport Controls (app.js) ✅ Implemented
- [ ] Final Review and Testing

## 🔍 Analysis & Investigation
*   **Visualizer UX:** The actual D3 Visualizer is located at `internal/ui/web/index.html` (HTML structure) and `internal/ui/web/app.js` (D3.js logic). Previously, the plan incorrectly targeted `trace-viewer.html`, which is the Agent Trace Viewer for JSONL logs and must NOT be modified.
*   **UI Mocks:** We have pre-designed HTML/Tailwind templates in the `./stitch/` directory that must be used.
    *   `./stitch/graphdb_main_visualizer/code.html` (and `screen.png`) for the main aesthetics and search box.
    *   `./stitch/blast_radius_analysis/code.html` (and `screen.png`) for node inspection/details and analytical controls.
    *   **Crucial Integration Step:** The Engineer must use these `code.html` files as the base HTML/Tailwind templates for `internal/ui/web/index.html` and wire the existing D3.js logic within `internal/ui/web/app.js` into them.

## 📝 Implementation Plan

### **MANDATORY Recovery Step**
1.  **Revert Incorrect Changes:**
    *   *Action:* Run `git restore trace-viewer.html`. ✅ Done
    *   *Detail:* Ensure that `trace-viewer.html` is restored to its original state. It is the Agent Trace Viewer and must remain untouched. ✅ Verified

### Phase 1: Modernization and Logic Migration

1.  **Step 1.A: Modernize Aesthetics & Search Box (Main Visualizer Mock):** ✅ Implemented
    *   *Action:* Modify `internal/ui/web/index.html` using `./stitch/graphdb_main_visualizer/code.html`.
    *   *Detail:* Use the provided `code.html` template from `./stitch/graphdb_main_visualizer/` to replace the HTML structure in `internal/ui/web/index.html`. Ensure the search input is prominent with descriptive placeholder text (e.g., "Describe a feature in natural language...") matching the `screen.png` mock. **Do not put D3 logic here.**
2.  **Step 1.B: Node Inspection, Details Panel & Analytical Tooltips (Blast Radius Mock):** ✅ Implemented
    *   *Action:* Modify `internal/ui/web/index.html` using elements from `./stitch/blast_radius_analysis/code.html`.
    *   *Detail:* Integrate the side panel or rich tooltip structure from the blast radius template (`./stitch/blast_radius_analysis/`) into the main layout of `internal/ui/web/index.html`. Add analytical controls ("Show Pinch Points", etc.) with explanatory `title` attributes or hover tooltips to explain their purpose in plain English. Added visualization controls and legend to the graph area.
3.  **Step 1.C: Wire Up D3.js Logic and Panel Interactions:** ✅ Implemented
    *   *Action:* Modify `internal/ui/web/app.js`.
    *   *Detail:* Update the existing D3.js setup, simulation, and render loops in `internal/ui/web/app.js` to target the new DOM elements defined in `index.html`. Update the D3 node `click` or `mouseover` event handlers to populate the new details panel with node properties such as `d.id`, `d.label`, `d.properties.file`, etc. ✅ Done
4.  **Step 1.D: Intelligent Viewport Controls:**
    *   *Action:* Modify `internal/ui/web/app.js`.
    *   *Detail:* Implement a `dblclick` event listener on D3 nodes. Use `d3.zoom().transform` to transition the SVG viewport to center on the clicked node's `(x, y)` coordinates, applying a reasonable zoom scale.

### Testing Strategy
*   Start the GraphDB HTTP server and navigate to the web UI.
*   Verify the layout matches the Tailwind designs from the `stitch/` mocks.
*   Test search functionality, node click/hover details panel, and analytical button tooltips.
*   Verify that double-clicking a node centers the camera smoothly.
*   Verify that `trace-viewer.html` is untouched and functions normally as a JSONL trace viewer.

## 🎯 Success Criteria
1.  `trace-viewer.html` is completely restored and untouched.
2.  `internal/ui/web/index.html` has a modernized, clean look matching the `./stitch` mocks using Tailwind CSS, including the details panel and search.
3.  `internal/ui/web/app.js` cleanly houses all D3.js logic and properly targets the new UI elements.
4.  Clicking nodes reveals their details in the dedicated panel based on the Blast Radius mock.
5.  Double-clicking nodes centers the camera on them.
6.  Search and Analytical buttons are intuitive and styled according to their respective mocks.
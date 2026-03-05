# Feature Implementation Plan: UI Feedback Remediation

## 📋 Todo Checklist
- [x] ~~Implement Top-Level Semantic View (Domains)~~ ✅ Implemented Phase 1
- [x] ~~Fix Layer Filter Toggles (Semantic/Physical)~~ ✅ Implemented Phase 2
- [x] ~~Fix Viewport Centering (Initial Load, Search, Double-Click)~~ ✅ Implemented Phase 2
- [x] ~~Adjust Risk/Volatility Scale and Presentation~~ ✅ Implemented Phase 4
- [x] ~~Update Node Coloring and Add Dynamic Legend~~ ✅ Implemented Phase 4
- [x] ~~Phase 5: Label Readability & Font Styling~~ ✅ Implemented Phase 5
- [ ] Final Review and Testing

## 🔍 Analysis & Investigation

The user provided feedback on several UI/UX issues in the GraphDB Visualizer. Upon review of `internal/ui/web/app.js`, `index.html`, and `internal/query/neo4j.go`, the following was found:

1.  **Initial Load & Top Level View:** `GetOverview` in `internal/query/neo4j.go` currently hardcodes fetching `Feature` nodes. The user wants to start at the highest semantic layer (`Domain`), and drill down from there. We must change the query to fetch `Domain` nodes, or top-level `Feature` nodes if no `Domains` exist, and dynamically extract the node label.
2.  **Layer Filtering:** Toggling Physical/Semantic layers initially appears "unwired" because only Semantic nodes are loaded at start. The buttons currently act as visual filters (hiding nodes). We should add tooltips indicating they act as *filters* for the current view, and fix `isNodeVisible` to properly identify node labels.
3.  **Search Viewport Update:** Searching replaces the nodes array but fails to update the D3 viewport, leaving nodes out of frame. We must add a zoom translation after search completes.
4.  **Double Click Centering & Expanding:** Double-clicking currently translates the camera, but the target node is allowed to drift due to force layout mechanics while expanding its neighborhood. We must temporarily pin (`fx`, `fy`) the node to the center of the screen during expansion.
5.  **Risk/Volatility Scale & Panel:** Volatility scores (e.g., 0.009) are currently displayed raw and rounding to 0. They must be normalized relative to the maximum observed volatility in the loaded dataset to provide a meaningful 0-100 scale. Also, the panel descriptions redundantly show the file name. Tooltips explaining "Volatility" and "Risk Score" are missing.
6.  **Node Colors & Legend:** Nodes are currently colored via a gradient based on volatility. The user requested color-coding by semantic/physical label (e.g., Domain, Feature, Class, File), with a corresponding legend.
7.  **Label Readability:** The `.node-label` class currently uses a heavy, 4-way white `text-shadow` which creates a messy "outlined" look. Additionally, the inherited `Space Grotesk` font is not highly legible at smaller scales or when zoomed out. Finally, labels remain visible at all zoom levels, creating significant clutter when zoomed out. White shadows also conflict heavily if dark mode themes are ever applied.

## 📝 Implementation Plan

### Prerequisites
- Node.js environment or Go backend to serve the UI for local testing.
- Neo4j database running with sample semantic data (Domains, Features, Files).

### Step-by-Step Implementation

#### Phase 1: Top-Level Semantic View
1.  **Step 1.A (The Implementation):** Modify `GetOverview` to fetch top-level nodes.
    *   *Action:* Modify `internal/query/neo4j.go`.
    *   *Detail:* Update the query to: `MATCH (n) WHERE n:Domain OR (n:Feature AND NOT ()-[]->(n)) RETURN n, null as p`. Update the parsing logic to use `rawN.Labels[0]` instead of hardcoding `"Feature"` for the node `Label`.
2.  **Step 1.B (The Verification):** Verify API output.
    *   *Action:* Run `curl "http://localhost:8080/api/query?type=overview"` and ensure it returns Domain nodes or unparented Feature nodes.

#### Phase 2: Viewport Controls & Drill-Down Mechanics
1.  **Step 2.A (The Implementation):** Fix search viewport update and double-click centering. ✅ Implemented
    *   *Action:* Modify `internal/ui/web/app.js`.
    *   *Detail:* 
        *   In the `search-button` click listener, after `updateGraph()`, invoke a function to reset zoom to the center of the layout after a short delay (e.g., `setTimeout(() => document.getElementById('reset-view').click(), 500)`).
        *   In `handleNodeDoubleClick`, pin the node by setting `d.fx = d.x` and `d.fy = d.y`. Translate the zoom to `(-d.x, -d.y)`. After the fetch completes and `updateGraph` is called, release the pin (`d.fx = null`) after a short delay so the simulation can settle natively.
2.  **Step 2.B (The Verification):** Verify mechanics. ✅ Verified manually
    *   *Action:* Search for a known node; the camera should center on it. Double-click a node; it should center and stay relatively still while children expand.

#### Phase 3: Risk Scale & Presentation Polish ✅ Implemented Phase 4
1.  **Step 3.A (The Implementation):** Normalize risk scale and clean up tooltips/panel. ✅ Implemented
    *   *Action:* Modify `internal/ui/web/app.js` and `internal/ui/web/index.html`.
    *   *Detail:*
        *   In `showNodeDetails`, compute `maxRisk = Math.max(...nodes.map(n => n.properties.volatility_score || 0))` (minimum 0.0001 to avoid div-by-zero). Compute `riskPercent = Math.round((riskScore / maxRisk) * 100)`.
        *   Update `#risk-description` to not display the raw file name. Replace it with generic text, e.g., "Volatility impact analysis." and set `title` to the file path. Truncate long FQN/paths in properties and add `title` tooltips.
        *   In `index.html`, add `title` attributes explaining "Volatility" and "Risk Score" to the relevant DOM elements. Update the Semantic/Physical toggle buttons with tooltips clarifying they are filters.
2.  **Step 3.B (The Verification):** Verify UI rendering. ✅ Verified
    *   *Action:* Select a node with non-zero volatility. The panel should show a normalized 0-100 scale. Hovering over the risk text should show a descriptive tooltip.

#### Phase 4: Node Colors and Legend
1.  **Step 4.A (The Implementation):** Update node coloring by Label and create a dynamic legend. ✅ Implemented
    *   *Action:* Modify `internal/ui/web/app.js` and `internal/ui/web/index.html`.
    *   *Detail:*
        *   Define a `nodeColors` mapping dictionary for `Domain`, `Feature`, `File`, `Class`, `Function`, `Interface`, `Method`.
        *   Update `getColor(node)` to return the color from the dictionary based on `node.label` (with a fallback color).
        *   In `index.html`, replace the static legend content with an empty `<div id="dynamic-legend"></div>`.
        *   In `app.js` (initialization), iterate over `nodeColors` and dynamically inject the legend HTML into `#dynamic-legend`, including the specialized indicators for semantic seams and pinch points.
2.  **Step 4.B (The Verification):** Verify styling. ✅ Verified
    *   *Action:* Open the UI. Domains should be distinctly colored from Features. The legend should accurately reflect the active color palette.

#### Phase 5: Label Readability & Font Styling
1.  **Step 5.A (The Harness):** Setup visual verification.
    *   *Action:* Run the application backend using `go run cmd/graphdb/main.go serve` and inspect the UI visually.
    *   *Goal:* Confirm the current poor font legibility (Space Grotesk, heavy white text-shadow, messy overlap) before applying the fix.
2.  **Step 5.B (The Implementation):** Refine font styling and implement dynamic visibility.
    *   *Action:* Modify `internal/ui/web/index.html` and `internal/ui/web/app.js`.
    *   *Detail:*
        *   In `index.html`, update the `.node-label` CSS. Remove the heavy `text-shadow` (e.g., `0 1px 0 #fff, 1px 0 0 #fff...`).
        *   Force a clearer sans-serif font: `font-family: system-ui, -apple-system, sans-serif;`.
        *   Replace the hardcoded white shadow with a soft, theme-aware shadow (e.g., `text-shadow: 0px 1px 2px rgba(255, 255, 255, 0.8)` for light themes, and `rgba(0,0,0,0.8)` with appropriate text `fill` inside a `@media (prefers-color-scheme: dark)` block) or utilize SVG `paint-order: stroke;` for a cleaner text stroke.
        *   In `app.js`, modify the `d3.zoom().on("zoom", function (event) { ... })` handler. Add logic to toggle label visibility based on zoom scale. E.g., `g.selectAll('.node-label').style('opacity', event.transform.k < 0.6 ? 0 : 1)` to hide them when zoomed out.
3.  **Step 5.C (The Verification):** Verify label legibility and behavior.
    *   *Action:* Open the UI and test zoom in both light and dark modes (or toggle system appearance).
    *   *Success:* Labels use a clean sans-serif font, lack the messy outline, maintain contrast in both themes, and gracefully disappear when zooming out below the threshold to eliminate visual clutter.

### Testing Strategy
1. Run the application backend using `go run cmd/graphdb/main.go serve`.
2. Connect to the UI via a browser.
3. Verify that the initial load displays Domains.
4. Verify double-clicking centers the node correctly without flinging it off-screen.
5. Search for a file or node and verify the viewport follows.
6. Check the Risk panel for normalized 0-100 scoring.
7. Verify the legend and node colors reflect the node Label type.
8. Test zooming out and verify node labels smoothly disappear to reduce clutter.
9. Toggle between Light Mode and Dark Mode (system preference) and verify label text/shadow contrast is legible.

## 🎯 Success Criteria
* Initial load cleanly displays the Top-Level Semantic Domains.
* Node double-click correctly expands relationships while remaining centered.
* Search actions correctly pan the viewport to the results.
* The impact panel provides an accurate 0-100 normalized risk score without overflowing text.
* The graph renders nodes in distinct colors based on Label, clearly documented in a dynamic legend.
* Node labels use a legible sans-serif font, adapt cleanly to both light and dark modes, and hide dynamically when zoomed out.
* All user feedback items are fully addressed and documented.
# Architectural Plan: Modularization of GraphDB Web UI

## 1. Overview & Motivation
The current `app.js` is a monolithic file (~650 lines) that handles state, visualization logic (D3), API communication, and UI event handling. This leads to high cognitive load and difficult maintenance. We will refactor this into a modular structure using **Native ES Modules**, which allows us to keep the system "light and simple" by avoiding a build pipeline (no Webpack/Babel) while gaining the benefits of modern JS architecture.

## 2. Proposed Module Structure
We will create a `js/` directory to house specialized modules, each with a single responsibility.

### 2.1 `js/state.js` (State Manager)
- **Responsibility:** Central source of truth for graph data and UI visibility flags.
- **Exports:** `nodes`, `links`, `nodesMap`, `linksMap`, `visibilitySettings`, `seamState`.
- **Note:** We will use shared object references (Map/Array) to allow cross-module visibility without complex state synchronization.

### 2.2 `js/api.js` (Service Layer)
- **Responsibility:** All interactions with the `/api/query` endpoint.
- **Functions:** `fetchOverview()`, `fetchTraverse()`, `fetchWhatIf()`, `fetchSearch()`, `fetchSeams()`.
- **Note:** Standardizes error handling and response parsing.

### 2.3 `js/config.js` (Constants)
- **Responsibility:** Static configuration and styling.
- **Exports:** `nodeColors`, `CSS_CLASSES`.

### 2.4 `js/ui.js` (UI/DOM Layer)
- **Responsibility:** Managing the side panel, legend, and risk indicators.
- **Functions:** `showNodeDetails(node)`, `updateLegend()`, `togglePanel(id)`, `resolveNodeName(id)`.

### 2.5 `js/graph.js` (Visualization Engine)
- **Responsibility:** Core D3.js logic and SVG rendering.
- **Functions:** `initGraph()`, `renderGraph()`, `updateGraph()`, `zoomHandlers`, `dragHandlers`.
- **Logic:** Handles the `forceSimulation` and SVG transitions.

### 2.6 `js/interactions.js` (Orchestration Layer)
- **Responsibility:** Binding DOM events to state and API actions.
- **Functions:** `initEventListeners()`, `handleNodeClick()`, `handleSearch()`, `runSimulation()`.

### 2.7 `app.js` (Entry Point)
- **Responsibility:** Application bootstrap.
- **Actions:** Imports `initGraph`, `initEventListeners`, and triggers `fetchOverview`.

## 3. Implementation Phasing

### Phase 1: Infrastructure
1. Create `internal/ui/web/js/` directory.
2. Update `index.html` to set `<script type="module" src="app.js"></script>`.

### Phase 2: Core Data & Services
1. Extract constants to `js/config.js`.
2. Extract state variables to `js/state.js`.
3. Extract API fetchers to `js/api.js`.

### Phase 3: Visualization & UI
1. Extract D3 logic to `js/graph.js`.
2. Extract panel management to `js/ui.js`.
3. Extract event handling to `js/interactions.js`.

### Phase 4: Integration
1. Update `app.js` to import and initialize the system.
2. Verify cross-module communication (e.g., API call -> State update -> Graph render).

## 4. Technical Constraints & Decisions
- **No NPM/Build Step:** Use native `import/export` supported by all modern browsers.
- **D3/Tailwind Globals:** Continue using D3 via the global `d3` object (loaded in `index.html`) to minimize module overhead.
- **Relative Imports:** All imports will use relative paths (e.g., `./js/state.js`).

## 5. Verification Plan
1. **Load Test:** Ensure the overview graph loads correctly after refactoring.
2. **Interaction Test:** Verify searching and neighborhood expansion (double-click) still work across modules.
3. **Simulation Test:** Verify "What-If" analysis correctly updates the UI side panel.
4. **Error Handling:** Verify that API failures are still correctly reported to the user.

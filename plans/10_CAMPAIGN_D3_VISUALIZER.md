# Feature Implementation Plan: D3 Graph Visualizer

## 📋 Todo Checklist
- [x] ~~Phase 1: Go HTTP Server and API Foundation~~ ✅ Implemented
- [ ] Phase 2: Static Asset Embedding and UI Framework
- [ ] Phase 3: D3 Semantic Search and Initial View
- [ ] Phase 4: Neighborhood Navigation (Force-Directed Graph)
- [ ] Phase 5: Refactoring Risk Analysis (Impact View)
- [ ] Final Review and Testing

## 🔍 Analysis & Investigation
The `graphdb` system currently outputs structural and semantic queries via the CLI (JSON format). To support an interactive web-based D3 visualization, we must transition from a pure CLI to a hybrid CLI/Server architecture.
- **Architecture:** We will introduce a new `serve` command to the Go binary. This command will start a standard `net/http` server. It will use `//go:embed` to package the static D3 frontend files (HTML/JS/CSS), ensuring the binary remains standalone.
- **Dependencies:** The HTTP server will depend on the existing `query.GraphProvider` interface and `Neo4jProvider` implementation. The frontend will depend on the D3.js library.
- **Challenges:** Graph visualization performance degrades rapidly beyond a few hundred nodes. The D3 implementation must prune nodes, implement pagination or depth limits, and potentially aggregate edges or namespaces to maintain 60FPS interaction. The local `.env` file contains all necessary connection info to connect to the local podman Neo4j container. **CRITICAL:** This existing database is fully populated with a sample application (NOT this codebase). The D3 visualizer will connect to this existing container to visualize the sample app. Do NOT use the `graphdb` skill to analyze the architecture of this project itself, as it will return data for the sample application.

## 📝 Implementation Plan

### Prerequisites
Use the connection info in the `.env` file to connect to the existing local podman container. **DO NOT create any new databases or reset volumes.** The database is already populated with a sample application which we will use to build and test the D3 visualizer. If you encounter any connection failures, **STOP** immediately and seek user guidance.

### Step-by-Step Implementation

#### Phase 1: Go HTTP Server and API Foundation
*(See `plans/10_PHASE_1_HTTP_SERVER_TASKS.md` for detailed technical implementation tasks)*
1.  **Step 1.A (The Harness):** Define the server routing test. ✅ Implemented
    *   *Action:* Create `internal/ui/server_test.go`.
    *   *Goal:* Assert that the HTTP server initializes, maps `/api/health` to a 200 OK, and wires up a mock `query.GraphProvider` to a `/api/query` endpoint.
2.  **Step 1.B (The Implementation):** Build the `Server` struct. ✅ Implemented
    *   *Action:* Create `internal/ui/server.go`.
    *   *Detail:* Implement an HTTP server struct that takes a `query.GraphProvider`. Create a generic handler for `/api/query` that accepts JSON payloads equivalent to the CLI flags (e.g., `{"type": "hybrid-context", "target": "main", "depth": 1}`) and returns the JSON output directly from the provider.
3.  **Step 1.C (The Verification):** Verify the server harness. ✅ Implemented
    *   *Action:* Run `go test ./internal/ui/...`.
    *   *Success:* The server routing tests pass.

#### Phase 2: Static Asset Embedding and `serve` Command
1.  **Step 2.A (The Harness):** Define the CLI command test.
    *   *Action:* Update `cmd/graphdb/setup_mock_mode.go` (or create a new test) to assert that invoking `graphdb serve` starts the server without panicking.
    *   *Goal:* Assert that the `serve` command parses port arguments and launches the web interface.
2.  **Step 2.B (The Implementation):** Implement `cmd_serve.go` and embed static files.
    *   *Action:* Create `cmd/graphdb/cmd_serve.go`. Create a directory `web/` in the project root containing an `index.html` and `app.js`. Use `//go:embed web/*` in `internal/ui/server.go` and `http.FileServer` to serve it at the root `/`. Add `handleServe` to `main.go`.
    *   *Detail:* The command should accept `-port` (default 8080). It must initialize the `Neo4jProvider` identically to `cmd_query.go` before starting the server.
3.  **Step 2.C (The Verification):** Verify the embedded server.
    *   *Action:* Run `go build -o bin/graphdb ./cmd/graphdb` and then `./bin/graphdb serve -port 8081 & curl http://localhost:8081/`.
    *   *Success:* The CLI command starts the server and serves the static `index.html`.

#### Phase 3: D3 Semantic Search and Initial View
1.  **Step 3.A (The Harness):** Define the UI integration test (mock backend).
    *   *Action:* Create a mock backend response file in `web/mock_data.json` simulating a `search-similar` query.
    *   *Goal:* Assert that the frontend JavaScript correctly fetches and parses the node list.
2.  **Step 3.B (The Implementation):** Build the frontend layout and search.
    *   *Action:* Modify `web/index.html` and `web/app.js`.
    *   *Detail:* Import D3.js via CDN. Add a search bar input. On submission, perform an asynchronous `fetch` to `/api/query?type=search-similar&target={input}`. Render the returned nodes as a simple D3 selection (e.g., a scattered grid of circles with labels).
3.  **Step 3.C (The Verification):** Verify the search interaction.
    *   *Action:* Open the browser to `http://localhost:8080`, submit a search, and verify nodes appear.
    *   *Success:* Nodes map to the search response and display correctly on the canvas.

#### Phase 4: Neighborhood Navigation (Force-Directed Graph)
1.  **Step 4.A (The Harness):** Define graph data structure mapping.
    *   *Action:* Add logic in `app.js` to parse the Neo4j JSON response into a D3-compliant format (`{nodes: [], links: []}`). Add a unit test or console assertion to verify duplicate node IDs are merged.
    *   *Goal:* Assert the graph data model is valid for the D3 simulation.
2.  **Step 4.B (The Implementation):** Implement the Force-Directed Layout.
    *   *Action:* Modify `web/app.js`.
    *   *Detail:* Implement `d3.forceSimulation()`. Add `forceLink`, `forceManyBody`, and `forceCenter`. Implement an `on("dblclick", ...)` event listener on nodes. When a node is double-clicked, fetch its neighbors (`/api/query?type=hybrid-context&target={node.id}&depth=1`), merge the new nodes/links into the current simulation state, and restart the simulation (`simulation.alpha(1).restart()`). Add color coding based on `node.type` or `ui_contaminated` flags.
3.  **Step 4.C (The Verification):** Verify expansion logic.
    *   *Action:* In the browser, double-click a node and observe new neighbors branching out with animated force dynamics.
    *   *Success:* The graph expands seamlessly without node duplication or simulation crashes.

#### Phase 5: Refactoring Risk Analysis (Impact View)
1.  **Step 5.A (The Harness):** Verify API payload for `what-if`.
    *   *Action:* Update `internal/ui/server_test.go` to ensure `what-if` payloads (which have comma-separated targets) are correctly parsed and forwarded to the `GraphProvider`.
    *   *Goal:* Ensure complex extraction targets are handled by the gateway.
2.  **Step 5.B (The Implementation):** Implement the Impact Tree View.
    *   *Action:* Modify `web/app.js` and `web/index.html`.
    *   *Detail:* Add a context menu (right-click) to nodes. Add a "Simulate Extraction" action. This calls `/api/query?type=what-if&target={node.id}`. The result should be displayed in a secondary D3 visualization (e.g., a `d3.tree` or collapsible indented tree in a side panel) showing severed edges and orphaned components in warning colors (red/orange).
3.  **Step 5.C (The Verification):** Verify impact visualization.
    *   *Action:* Right-click a node, trigger the what-if simulation, and inspect the side panel.
    *   *Success:* Upstream and downstream impacts are clearly delineated and styled to indicate risk.

### Testing Strategy
- **Backend:** Standard Go unit tests for HTTP routing (`net/http/httptest`), ensuring JSON serialization matches the CLI. E2E test wrapping the server execution to verify it doesn't leak memory.
- **Frontend:** Manual browser testing for visual interactions (D3 layouts, dragging, zooming). Mock JSON endpoints will be used to isolate frontend UI testing from the Neo4j database state.

## 🎯 Success Criteria
- The Go binary starts a standalone web server serving the D3 frontend using only `go:embed`.
- The user can issue a natural language search to locate latent features or domains.
- The user can interactively double-click nodes to expand their physical dependencies dynamically.
- The user can right-click to simulate a Strangler Fig extraction, visualizing the resulting system impact.
- The visualizations remain responsive (fluid physics) for graphs of up to a few hundred nodes.
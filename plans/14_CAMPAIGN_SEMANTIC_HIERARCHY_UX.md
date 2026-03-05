# Campaign 14: Semantic Hierarchy & Layer UX

## 🎯 Objective
Bring the architectural "Mental Model" to life in the UI by implementing functional Layer Toggles (Physical vs. Semantic) and a dedicated "Semantic Trace" to visualize the exact structural intent path: 
`[Domain] --(PARENT_OF)--> [Feature] --(IMPLEMENTS)--> [Function] --(DEFINED_IN)--> [File]`.

## 📋 Task Plan

### Phase 1: Wire Up Layer Toggles (Frontend)
Currently, the "Physical Layer" and "Semantic Layer" buttons in the top-right of `index.html` are static. 
- [x] **Step 1.A - State Management:** Add state variables in `app.js` to track which layers are active (`showPhysical`, `showSemantic` - defaulting to both true). ✅ Implemented
- [x] **Step 1.B - Button Event Listeners:** Attach click listeners to the toggle buttons in `index.html`. When clicked, toggle the visual state of the button (active/inactive styling) and update the state variables. ✅ Implemented
- [x] **Step 1.C - D3 Node Filtering:** Modify the D3 `renderGraph` and/or `tick` functions in `app.js` to dynamically adjust the opacity or visibility of nodes and links based on the active layers. ✅ Implemented
    - *Physical Nodes:* `Function`, `Class`, `File`, `Variable`, etc.
    - *Semantic Nodes:* `Domain`, `Feature`.

### Phase 2: Implement "Trace Intent" Path (Backend + Frontend)
When users click a node, a generic `depth=1` traversal often creates a noisy spiderweb. We need a targeted visualization path.
- [x] **Step 2.A - UI Action Button:** In `index.html` (inside the `#impact-panel` side panel actions), add a new button: `"Trace Intent Hierarchy"`. ✅ Implemented
- [x] **Step 2.B - Targeted Query API:** In `internal/ui/web/app.js`, wire the "Trace Intent" button to fire a specific targeted traversal query. If the existing `/api/query?type=traverse` cannot easily restrict to just `PARENT_OF`, `IMPLEMENTS`, and `DEFINED_IN` upward/downward paths, we may need a dedicated query type (e.g., `type=semantic-trace`) in `internal/query/neo4j.go` and `server.go`. ✅ Implemented
    - *Cypher Logic Needed:* `MATCH path = (d:Domain)-[:PARENT_OF*0..1]->(feat:Feature)-[:IMPLEMENTS*0..1]->(func:Function)-[:DEFINED_IN*0..1]->(file:File) WHERE func.id = $targetId RETURN path`
- [x] **Step 2.C - Graph Render Update:** When the trace data returns, merge the specific hierarchical path into the existing `nodesMap` and `linksMap`, and trigger `updateGraph()` so the D3 simulation snaps these specific hierarchical relationships into view cleanly. ✅ Implemented

### Phase 3: Remediation (Post-Rejection)
- [x] **Step 3.A - Fix Compilation:** Update `MockProvider` in `cmd/graphdb/mocks.go` and `MockGraphProvider` in `internal/rpg/orchestrator_test.go` to implement `SemanticTrace`. ✅ Implemented
- [x] **Step 3.B - Add Query Tests:** Add integration tests for `SemanticTrace` in `internal/query/neo4j_semantic_trace_test.go`. ✅ Implemented
- [x] **Step 3.C - Add API Tests:** Add unit tests for the `semantic-trace` endpoint in `internal/ui/server_test.go`. ✅ Implemented
- [x] **Step 3.D - Verify Build:** Ensure `go build` and `go test` pass (verified mocks and unit tests). ✅ Implemented

## 🏁 Success Criteria
1. Toggling the "Semantic" button off hides all Domain/Feature bubbles and their edges.
2. Toggling the "Physical" button off hides all Function/Class/File nodes, leaving only the conceptual domain map.
3. Clicking a Function node and selecting "Trace Intent" successfully draws the clean, 4-tier chain (`Domain -> Feature -> Function -> File`) without pulling in 50 unrelated sibling function calls.
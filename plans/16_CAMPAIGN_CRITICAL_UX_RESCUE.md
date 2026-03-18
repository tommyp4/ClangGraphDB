# Feature Implementation Plan: Campaign 16 - Critical UX Rescue

## 📋 Todo Checklist
- [x] Fix Missing LLM Domain Naming (Streaming Pipeline Metadata)
- [ ] Fix D3 Physics Instability (Center of Mass shifting during exploration)
- [ ] Final Review and Testing

## 🔍 Analysis & Investigation

**Issue 1: Missing LLM Domain Naming [FIXED]**
During Campaign 6, the system transitioned to an Out-of-Core streaming pipeline for graph construction. The `GlobalEmbeddingClusterer` relies on `collectSnippets` to grab file content or `atomic_features` to send to the LLM Summarizer to generate semantic domain names (e.g., "Authentication System").
However, the metadata query `GetFunctionMetadata` in `internal/query/neo4j_batch.go` was never updated to retrieve `line`, `end_line`, or `atomic_features`. Because these properties are missing, `collectSnippets` always returns an empty array. This triggers the LLM Summarizer's fallback logic, which returns `"Unknown Feature"`, causing the `Builder` to fall back to generic IDs like `domain-unknown-feature`.
*Update:* This has been resolved in `internal/query/neo4j_batch.go`.

**Issue 2: D3 Physics Instability (The "Sliding Graph" Problem)**
In the D3 Visualizer (`app.js`), the physics simulation uses `d3.forceCenter(width / 2, height / 2)`. While suitable for static graphs, `forceCenter` creates severe UX issues for interactive, expanding graphs. When a user double-clicks a node to expand its neighborhood, new nodes are spawned. This changes the graph's center of mass. The `forceCenter` violently translates all existing nodes to re-center the new mass, which visually rips the graph away from the user's viewport, neutralizing the intelligent viewport zoom control added previously. Additionally, new nodes spawn at random coordinates instead of originating from their parent, causing explosive layout shifts.

## 📝 Implementation Plan

### Prerequisites
None.

### Step-by-Step Implementation

#### Phase 1: The Domain Naming Fix (Backend) - COMPLETED
1.  **Step 1.A (The Harness):** Characterize `GetFunctionMetadata` behavior.
    *   *Action:* Create/Update `internal/query/neo4j_batch_test.go` (if exists) or verify manually using the integration tests.
    *   *Goal:* Ensure `GetFunctionMetadata` properly queries and maps the needed properties.
2.  **Step 1.B (The Implementation):** Update the Cypher query.
    *   *Action:* Modify `internal/query/neo4j_batch.go` -> `GetFunctionMetadata()`.
    *   *Detail:* Update the Cypher `RETURN` clause to include `n.line as line, n.end_line as end_line, n.atomic_features as atomic_features`. Update the record extraction loop to retrieve these values and populate the `node.Properties` map appropriately, using `neo4j.GetRecordValue`.
3.  **Step 1.C (The Verification):** Build and Verify.
    *   *Action:* Run `go test ./internal/query -run TestGetFunctionMetadata` (if applicable) and manually verify by running an ingestion and observing node labels.

#### Phase 2: D3 Physics Stabilization (Frontend)
1.  **Step 2.A (The Verification Baseline):**
    *   *Action:* Manual verification via browser (Completed: `before_double_click.png`, `after_double_click.png`).
    *   *Goal:* Confirm the "Sliding Graph" and "Crowding" issues.
2.  **Step 2.B (The Implementation - Removing strict center):** Update physics configuration in `internal/ui/web/js/graph.js`.
    *   *Action:* Modify `initGraph()`.
    *   *Detail:* Replace `.force("center", d3.forceCenter(width / 2, height / 2))` with `.force("x", d3.forceX(width / 2).strength(0.05))` and `.force("y", d3.forceY(height / 2).strength(0.05))`.
3.  **Step 2.C (The Implementation - Coordinated Spawning):** Seed new node coordinates in `internal/ui/web/js/graph.js`.
    *   *Action:* Modify `updateGraph()`.
    *   *Detail:* When a new node is added, if it has a source/parent that already exists in the graph, initialize the new node's `x` and `y` coordinates to match its parent's current position. This ensures nodes "bloom" from their origin instead of jumping from (0,0) or random locations.

#### Phase 3: Force-Directed De-cluttering (Frontend)
1.  **Step 3.A (The Implementation - Collision Detection):** Prevent node overlap in `internal/ui/web/js/graph.js`.
    *   *Action:* Modify `initGraph()`.
    *   *Detail:* Add `.force("collision", d3.forceCollide().radius(45))` to the simulation. This creates a physical buffer around nodes (radius 20 node + 25px padding).
2.  **Step 3.B (The Implementation - Dynamic Repulsion):** Increase separation in `internal/ui/web/js/graph.js`.
    *   *Action:* Modify `initGraph()`.
    *   *Detail:* Increase `forceManyBody().strength()` from `-300` to `-1000`. This provides stronger repulsion between all nodes.
3.  **Step 3.C (The Implementation - Link Distance Tuning):** Increase breathing room in `internal/ui/web/js/graph.js`.
    *   *Action:* Modify `initGraph()`.
    *   *Detail:* Increase `forceLink().distance()` from `150` to `250`. This pushes connected nodes further apart, reducing label overlap.

### Testing Strategy
*   Backend: Ensure `graphdb enrich-features` successfully populates domains with semantic names instead of "Unknown Feature".
*   Frontend: Interactive validation in the browser. Double-clicking nodes should smoothly expand the network without shifting the entire canvas abruptly.

## 🎯 Success Criteria
*   The `enrich-features` pipeline queries the LLM successfully and labels latent domains with semantic names.
*   The D3 visualization remains stable and centered on the user's viewport during neighborhood expansion.
*   New nodes smoothly integrate into the graph layout without violently pulling the original graph out of focus.

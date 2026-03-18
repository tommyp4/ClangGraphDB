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
    *   *Action:* Manual verification via browser required since this is a visual D3 behavior.
    *   *Goal:* The graph must not slide uncontrollably when a node is expanded.
2.  **Step 2.B (The Implementation - Removing strict center):** Update physics configuration.
    *   *Action:* Modify `internal/ui/web/app.js`.
    *   *Detail:* Remove `.force("center", d3.forceCenter(width / 2, height / 2))`.
    *   *Detail:* Add `.force("x", d3.forceX(width / 2).strength(0.05))` and `.force("y", d3.forceY(height / 2).strength(0.05))` to the `d3.forceSimulation()` initialization.
3.  **Step 2.C (The Implementation - Coordinated Spawning):** Seed new node coordinates.
    *   *Action:* Modify `internal/ui/web/app.js` -> `updateGraph(newNodes, newLinks)`.
    *   *Detail:* When pushing a `normalizedNode` to `nodes`, rather than letting it default to random coordinates, set `normalizedNode.x = width / 2` and `normalizedNode.y = height / 2` (or set to `lastSelectedNode` coords if available) so new nodes "bloom" predictably.

### Testing Strategy
*   Backend: Ensure `graphdb enrich-features` successfully populates domains with semantic names instead of "Unknown Feature".
*   Frontend: Interactive validation in the browser. Double-clicking nodes should smoothly expand the network without shifting the entire canvas abruptly.

## 🎯 Success Criteria
*   The `enrich-features` pipeline queries the LLM successfully and labels latent domains with semantic names.
*   The D3 visualization remains stable and centered on the user's viewport during neighborhood expansion.
*   New nodes smoothly integrate into the graph layout without violently pulling the original graph out of focus.

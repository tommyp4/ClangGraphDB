# Plan: Fix Severed Edges Metadata in What-If Query

## Objective
Ensure that "Severed Edges" returned by the `WhatIf` query include the full semantic names and properties of the source and target nodes, matching the behavior of "Orphaned Nodes." This will allow the UI to display rich names in the "Affected Pathways" sidebar without requiring the nodes to be pre-cached.

## Is this a quick fix?
**Yes.** This is a targeted backend adjustment. It requires modifying a single Cypher query in the Go backend and updating the corresponding structs, with no changes needed in the UI.

## Root Cause
The `WhatIf` function (`internal/query/neo4j_whatif.go`) currently executes the following Cypher query for severed edges:
```cypher
MATCH (n)-[r]->(m)
WHERE (n.id IN $targets AND NOT m.id IN $targets)
   OR (NOT n.id IN $targets AND m.id IN $targets)
RETURN n.id as source, m.id as target, type(r) as type
```
This only returns the string IDs. The UI's `resolveNodeName` function then attempts to look up these IDs in its local `nodesMap`. If the node hasn't been rendered yet, the map lookup fails, and the UI displays the raw ID (e.g., `feature-xyz`).

## Implementation Steps

### 1. Update the `WhatIf` Cypher Query ✅ Implemented
Modify the `severedQuery` in `internal/query/neo4j_whatif.go` to return the full node properties for both the source and target, rather than just their IDs.

**Current:**
```cypher
RETURN n.id as source, m.id as target, type(r) as type
```

**Proposed:**
```cypher
RETURN n as sourceNode, m as targetNode, type(r) as type
```

### 2. Update the `Edge` Struct Parsing ✅ Implemented
Update the Go code that iterates through the `severedQuery` results to extract the properties and labels from the returned `neo4j.Node` objects, similar to how it's done for `OrphanedNodes`.

Currently, `graph.Edge` only holds `SourceID` and `TargetID`. To provide the UI with the full metadata without breaking the existing edge structure, we have two options:

**Option A (Recommended & Fastest): Return the nodes alongside the edges.**
The `WhatIfResult` struct in `internal/query/interface.go` does not have a field to hold the full nodes involved in the severed edges (it only holds the edge definitions). We can add a new field to `WhatIfResult` to hold any newly discovered nodes involved in the severed edges, which the UI can then ingest into its `nodesMap`.

*   **Modify `interface.go`:** Add `AffectedNodes []*graph.Node` to `WhatIfResult`. ✅ Done
*   **Modify `neo4j_whatif.go`:** Parse `sourceNode` and `targetNode`, sanitize their properties using `sanitizeProperties`, and append them to `AffectedNodes` (ensuring no duplicates). Keep the edge mapping as-is using the extracted IDs. ✅ Done

**Option B (More disruptive):** Modify the `graph.Edge` struct to hold full source/target Node objects instead of just IDs. This could have cascading effects on other API endpoints.

*We will proceed with Option A.*

### 3. Review `CrossBoundaryCalls` ✅ Implemented
The `crossQuery` in `neo4j_whatif.go` (which finds a subset of severed edges) suffers from the exact same issue:
```cypher
RETURN n.id as source, m.id as target, type(r) as type
```
Apply the same fix here: return the nodes, parse them, and add them to the new `AffectedNodes` list.

### 4. UI Compatibility ✅ Implemented
The UI (`internal/ui/web/app.js`) already correctly handles arbitrary new nodes being returned from the `WhatIf` query:
```javascript
// Update the local graph with any newly discovered nodes (orphans, shared state)
if (data.orphaned_nodes) updateGraph(data.orphaned_nodes, []);
if (data.shared_state) updateGraph(data.shared_state, []);
// We will add:
if (data.affected_nodes) updateGraph(data.affected_nodes, []);
```
Once added to the graph via `updateGraph`, the `resolveNodeName` function will succeed, and the sidebar will display rich names automatically.
# Feature Implementation Plan: Interactive Impact Analysis & UI Polish

## 📋 Todo Checklist
- [x] **Fix "Affected Pathways" Naming Bug**
  - [x] ~~Update `internal/ui/web/app.js` to resolve raw IDs to semantic names using `nodesMap`.~~ ✅ Implemented
- [x] **Make "Affected Pathways" Interactive**
  - [x] ~~Implement a `focusNode` function in the UI to center and expand nodes.~~ ✅ Implemented
  - [x] ~~Add click listeners to sidebar impact items in `internal/ui/web/app.js`.~~ ✅ Implemented
- [ ] Final Review and Testing

## 🔍 Analysis & Investigation

### Root Cause of "feature-XXXX" Display
- **Backend API:** The `/api/query?type=what-if` endpoint returns structural edges (`SourceID`, `TargetID`) for severed connections, not full node objects.
- **Frontend UI:** The `runSimulation` function in `app.js` blindly renders these raw `targetId` strings (e.g., `feature-60f93cf4`) into the HTML instead of cross-referencing them against the client-side `nodesMap` to display their semantic `name` properties. The LLM actually *did* name them correctly; the UI just isn't showing it.

### UI Interaction Gaps
- **Static Sidebar:** The "Affected Pathways" list items are currently non-interactive text.
- **Hidden Hierarchy:** If a user sees an orphaned/severed feature in the sidebar, they cannot easily locate it in the visual graph without manually finding and expanding its parent domain.

### Dependencies
- `internal/ui/web/app.js`: Frontend rendering and graph interaction logic.

## 📝 Implementation Plan

### Prerequisites
- Neo4j database running and populated.
- Web server running (`make run` or `go run cmd/graphdb/*.go serve`).

### Step-by-Step Implementation

#### Phase 1: Fix Semantic Naming in Sidebar
1.  **Step 1.A (Update Rendering Logic):** Modify `runSimulation` in `internal/ui/web/app.js`.
    *   When iterating over `data.severed_edges`, look up the source and target nodes in `nodesMap`.
    *   Use `node.name` (or `node.properties.name`) if available; fallback to the raw ID.
    *   Apply the same lookup logic to `data.orphaned_nodes`.

#### Phase 2: Interactive Sidebar Navigation
1.  **Step 2.A (Implement `focusNode`):** In `internal/ui/web/app.js`, create a new function `focusNode(nodeId)`.
    *   Retrieve the node from `nodesMap` or `nodes` array.
    *   Trigger a camera transition (`zoom.transform`) to center on the node's coordinates (`x`, `y`).
    *   Invoke `handleNodeDoubleClick(null, node)` to trigger the neighborhood expansion logic, revealing the node's relationships.
2.  **Step 2.B (Attach Click Listeners):** Update the `runSimulation` HTML generation block.
    *   Add CSS classes for interactivity (`cursor-pointer`, `hover:bg-slate-200`, etc.).
    *   Attach `addEventListener('click', ...)` to each generated `div`, calling `focusNode` with the corresponding ID.

### Testing Strategy
- **Manual UI Testing:** 
  - Click a Domain node (e.g., "Toll Management Service" or "Driver Settlement").
  - Click the "Simulate Extraction" button (Blast Radius).
  - Verify the "Affected Pathways" sidebar displays human-readable names instead of `feature-XXXX`.
  - Click an item in the sidebar.
  - Verify the visual graph pans to the node and expands its context.

## 🎯 Success Criteria
- [ ] The "Affected Pathways" sidebar displays semantic names for features and domains.
- [ ] Users can click any item in the sidebar to automatically navigate to and expand that node in the visual graph.

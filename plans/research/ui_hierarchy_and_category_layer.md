# Research Report: UI Hierarchy Rendering and the Category Layer

## Executive Summary

The UI is hard-coded to a **two-level semantic model**: Domain and Feature. It has no awareness of a "Category" node type. The `CategoryClusterer` mechanism in `builder.go` is **fully implemented** and **unit-tested**, but it is **never wired** in `orchestrator.go` -- meaning the 3-level hierarchy (Domain -> Category -> Feature) is dead code in production. Even if it were activated, the Category nodes would be written to Neo4j with the label `Feature` (not `Category`), because `schema.go` only distinguishes `domain-*` IDs from everything else. The UI would render them indistinguishably from Features.

---

## 1. UI File Inventory

All UI assets reside under `internal/ui/`:

| File | Role |
|---|---|
| `internal/ui/server.go` | Go HTTP server. Serves embedded static files and API endpoints. |
| `internal/ui/server_test.go` | Server unit tests. |
| `internal/ui/progressbar.go` | Terminal progress bar (not web). |
| `internal/ui/web/index.html` | Single-page HTML shell (TailwindCSS + D3.js). |
| `internal/ui/web/app.js` | Bootstrap: fetches overview, initializes graph. |
| `internal/ui/web/js/config.js` | Node color map and CSS class constants. |
| `internal/ui/web/js/state.js` | Shared mutable state (nodes, links, maps, visibility). |
| `internal/ui/web/js/graph.js` | D3 force-directed graph rendering. |
| `internal/ui/web/js/ui.js` | Legend, node coloring, visibility predicates, detail panel. |
| `internal/ui/web/js/interactions.js` | Click/hover/search/blast-radius/seam handlers. |
| `internal/ui/web/js/api.js` | Fetch wrappers for `/api/query` endpoints. |
| `internal/ui/web/mock_data.json` | Test fixture (3 Function nodes only). |

There are no template files (`.tmpl`), no CSS files, and no build tooling. Styling is inline via TailwindCSS CDN.

---

## 2. Hierarchy-Related Term Search in UI Code

### 2.1 "Category" -- ZERO occurrences

There is no reference to "Category", "category", or any Category-related concept anywhere in `internal/ui/`.

### 2.2 "Domain" and "Feature" -- Hardcoded two-level model

**`internal/ui/web/js/ui.js`, lines 41-45:**
```javascript
export function isSemantic(n) {
    if (!n) return false;
    const label = (n.label || (n.properties && n.properties.label) || 'Node').toLowerCase();
    return label === 'domain' || label === 'feature';
}
```

This function determines the semantic/physical visibility toggle. Only `domain` and `feature` are recognized as semantic nodes. There is no `category` case.

**`internal/ui/web/js/config.js`, lines 1-10:**
```javascript
export const nodeColors = {
    'Domain': '#4f46e5',
    'Feature': '#0891b2',
    'File': '#64748b',
    'Class': '#9333ea',
    'Interface': '#db2777',
    'Method': '#ea580c',
    'Function': '#16a34a',
    'Unknown': '#94a3b8'
};
```

There is no `Category` entry. A Category node would fall through to the `Unknown` color (#94a3b8 gray).

### 2.3 "PARENT_OF" -- Not referenced in UI

The UI never examines edge types for hierarchical rendering. It treats the graph as a flat force-directed layout. All structure comes from whatever nodes/edges the API returns.

### 2.4 "hierarchy", "tree", "level" -- Not referenced in UI

The only "depth" references in the UI are the `stat-depth` display in the impact panel (`index.html` line 199) and the `fetchTraverse` depth parameter (`api.js` line 7). These are traversal parameters, not hierarchy-level concepts.

---

## 3. Can the UI Render a 3-Level Hierarchy?

**No. The UI cannot meaningfully render a middle "Category" layer.** Here is why:

### 3.1 No hierarchical layout

The graph uses a D3 force-directed layout (`graph.js` lines 55-58). There is no tree layout, no level-based positioning, no radial hierarchy. All nodes are peers in a physics simulation. PARENT_OF edges are rendered identically to CALLS or IMPLEMENTS edges -- as gray lines with arrowheads.

### 3.2 No Category color or legend entry

`config.js` defines colors for Domain, Feature, File, Class, Interface, Method, and Function. Category is absent. Even if Category nodes appeared in the graph, they would render as gray "Unknown" circles, visually indistinguishable from unrecognized node types.

### 3.3 Visibility toggle only understands two semantic types

`ui.js:isSemantic()` returns `true` only for `domain` or `feature`. A `category` node would be classified as "physical" and toggled on/off with the physical layer, which is semantically wrong.

### 3.4 The overview query only fetches top-level nodes

`neo4j.go` line 684-687 (GetOverview):
```
MATCH (n) WHERE n:Domain OR (n:Feature AND NOT ()-[]->(n))
```

This fetches Domains and "orphan" Features (those with no incoming edges). Category nodes (which have incoming PARENT_OF from Domain) would be excluded from the initial overview. They would only appear if a user double-clicked a Domain to expand its neighborhood.

### 3.5 SemanticTrace is 2-level

`neo4j.go` line 155-158 (SemanticTrace):
```
MATCH path = (d:Domain)-[:PARENT_OF*0..1]->(feat:Feature)-[:IMPLEMENTS*0..1]->(func)-[:DEFINED_IN*0..1]->(file:File)
```

The `PARENT_OF*0..1` only traverses one hop. A 3-level hierarchy (Domain -> Category -> Feature) requires `*0..2` to reach from Domain through Category to Feature. This query would fail to connect Domains to their leaf Features if a Category layer existed between them.

### 3.6 ExploreDomain handles depth but does not distinguish Category

`neo4j.go` lines 811-829 (ExploreDomain) uses `PARENT_OF*0..` (unbounded depth) to find implementing functions, so it CAN traverse through a Category layer. However, both Category and Feature nodes are returned with the same `Feature` label (see Section 4 below), so the UI has no way to distinguish them.

---

## 4. CategoryClusterer: Is It Dead Code?

### 4.1 Builder supports it (implemented, tested)

**`internal/rpg/builder.go`, lines 20-22:**
```go
// CategoryClusterer enables 3-level hierarchy: Domain -> Category -> Feature.
// If nil, falls back to 2-level: Domain -> Feature.
CategoryClusterer Clusterer
```

**`internal/rpg/builder.go`, lines 106-110:**
```go
if b.CategoryClusterer != nil {
    allEdges = b.buildThreeLevel(&domainFeature, nodes, domainName, lca, allEdges)
} else {
    allEdges = b.buildTwoLevel(&domainFeature, nodes, domainName, lca, allEdges)
}
```

The `buildThreeLevel` method (lines 172-258) is fully implemented. It performs two-pass clustering: first into categories, then into features within each category. It generates `category-*` IDs for the intermediate nodes.

**`internal/rpg/builder_test.go`, lines 108-179 (TestBuilder_BuildThreeLevel):**
A unit test verifies the 3-level hierarchy produces 6 nodes (2 Domains + 2 Categories + 2 Features) and 4 PARENT_OF edges. This test passes.

### 4.2 Orchestrator NEVER sets CategoryClusterer

**`internal/rpg/orchestrator.go`, lines 201-213 (RunClustering):**
```go
builder := &Builder{
    Clusterer:       clusterer,
    GlobalClusterer: globalClusterer,
    OnPhaseStart: func(...) { ... },
    OnStepStart:  func(...) { ... },
    OnStepEnd:    func(...) { ... },
}
```

The `CategoryClusterer` field is **never assigned**. The Builder receives only `Clusterer` and `GlobalClusterer`. Since `CategoryClusterer` defaults to `nil`, the `buildTwoLevel` path is always taken in production.

### 4.3 CRITICAL: Category nodes get the wrong Neo4j label

**`internal/rpg/schema.go`, lines 20-24 (Feature.ToNode):**
```go
func (f *Feature) ToNode() graph.Node {
    label := "Feature"
    if strings.HasPrefix(f.ID, "domain-") {
        label = "Domain"
    }
    ...
}
```

The label assignment logic only checks for `domain-` prefix. Nodes with `category-*` IDs (produced by `buildThreeLevel` at builder.go line 188) would receive the label `Feature`, not `Category`. This means:

- In Neo4j, Category nodes would be indistinguishable from Feature nodes
- The overview query (`n:Domain OR n:Feature`) would match them
- The UI color mapping would give them the Feature color (#0891b2)
- There would be no way for any downstream system to tell a Category apart from a Feature

### 4.4 Verdict: Dead code with a labeling bug

The `CategoryClusterer` path is structurally complete but:
1. Never activated in production (`orchestrator.go` does not wire it)
2. Even if activated, would produce nodes labeled `Feature` instead of `Category` (schema.go bug)
3. The UI would not render them correctly (no color, no visibility category, no legend entry)
4. The SemanticTrace Cypher query would break (only traverses 1 PARENT_OF hop)

---

## 5. Existing Plans Mentioning the Category Layer

| Plan | Location | Status |
|---|---|---|
| `plans/00_MASTER_ROADMAP.md` line 56 | Marked `[x]` as completed: "3-Level Hierarchy: Builder supports optional CategoryClusterer" | Technically true for Builder code only |
| `plans/rpg_gap_analysis_and_remediation.md` lines 193-201 | Phase 4: Task 4.1 describes implementing two-pass clustering. Task completed for builder.go only. | Builder done, wiring never done |
| `plans/rpg_integration_strategy.md` line 20 | Describes intended hierarchy: "Root -> Domain -> Category -> Feature" | Aspirational, not achieved |
| `plans/RPG_GAP_ANALYSIS.md` line 66 | Mentions feeding all nodes into CategoryClusterer | Not implemented |
| `plans/research/rpg_architecture_map.md` line 14 | Documents CategoryClusterer as optional | Accurate |
| `plans/research/architecture_deep_dive.md` lines 132, 294-295 | Documents builder.go category code paths | Accurate |
| `plans/fix_domain_feature_topology.md` | Mentions updating `buildThreeLevel` for new ClusterGroup interface | Does not address activation |
| `plans/fix_domain_and_contamination_architecture.md` | Does not mention Category at all | N/A |

---

## 6. Blast Radius Summary

If the Category layer were to be activated, the following components would need changes:

| Component | File | Required Change |
|---|---|---|
| Schema labeling | `internal/rpg/schema.go:22` | Add `category-` prefix check to assign label `Category` |
| Orchestrator wiring | `internal/rpg/orchestrator.go:201-213` | Create and assign a CategoryClusterer |
| UI color map | `internal/ui/web/js/config.js` | Add `'Category': '#...'` entry |
| UI semantic predicate | `internal/ui/web/js/ui.js:44` | Add `\|\| label === 'category'` |
| UI legend | `internal/ui/web/js/ui.js:10-38` | Legend auto-generates from config; no change needed IF config is updated |
| Overview query | `internal/query/neo4j.go:684-687` | Add `n:Category` or adjust filter logic |
| SemanticTrace query | `internal/query/neo4j.go:158` | Change `PARENT_OF*0..1` to `PARENT_OF*0..2` |
| GetOverview filtering | `internal/query/neo4j.go:685` | Decide whether Categories appear at top level or only on drill-down |

---

## Recommendations for Architect

1. **Decide whether the Category layer is wanted.** The builder code exists but is completely disconnected from the rest of the system. If the 3-level hierarchy is desired, substantial wiring work is needed across at minimum 7 files. If it is not desired, the `CategoryClusterer` field and `buildThreeLevel` method should be removed to reduce confusion.

2. **Fix `schema.go` regardless.** The `ToNode()` method is a ticking bomb. If anyone ever sets `CategoryClusterer`, category nodes will silently masquerade as Features. At minimum, add a `category-` prefix case that assigns a distinct label.

3. **The SemanticTrace query is the hardest to fix.** The Cypher pattern `(d:Domain)-[:PARENT_OF*0..1]->(feat:Feature)` assumes exactly one hop between Domain and Feature. Changing this to `*0..2` without also adding a `Category` label creates ambiguity -- you cannot tell if a matched node is a Category or a Feature. The solution requires either a distinct `Category` Neo4j label or a variable-length path with label filtering.

4. **The UI needs a tree view, not just force-directed.** Even with correct labels and colors, a force-directed graph cannot visually communicate hierarchy. Consider adding a collapsible tree layout for Domain exploration or a hierarchical radial layout for the overview.

5. **If keeping 2-level only, update the roadmap.** The master roadmap (`00_MASTER_ROADMAP.md` line 56) marks the 3-level hierarchy as complete. This is misleading. It should note that only the builder-level abstraction was implemented, without production wiring or UI support.

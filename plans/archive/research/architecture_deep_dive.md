# Architecture Deep Dive: Feature Extraction, Clustering, Contamination, and Domain Grouping

**Date:** 2026-03-16
**Branch:** fix-domain-grouping
**Scope:** Exhaustive analysis of the RPG pipeline (Extract -> Embed -> Cluster -> Summarize) and contamination subsystem.

---

## 1. Extractor

### Source
- **File:** `/home/jasondel/dev/graphdb-skill/internal/rpg/extractor.go`
- **Test:** `/home/jasondel/dev/graphdb-skill/internal/rpg/extractor_test.go`

### Interface Signature (line 14-16)
```go
type FeatureExtractor interface {
    Extract(code string, functionName string) ([]string, error)
}
```

### Implementations
1. **`LLMFeatureExtractor`** (line 20-93): Uses Vertex AI / Gemini to produce Verb-Object descriptors.
   - Truncates code at 4000 chars (line 48-49).
   - LLM prompt requests 1-5 lowercase Verb-Object descriptors (e.g., "validate email", "hash password").
   - Expects JSON array response. Strips markdown fencing. No retry logic.
   - Returns raw `fmt.Errorf` on failure -- no silent fallback here.

2. **`MockFeatureExtractor`** (line 96-100): Always returns `["process data", "validate input"]` regardless of input.

### Test Coverage
- Tests only cover the `MockFeatureExtractor`. No integration tests for `LLMFeatureExtractor`.
- The `MockFeatureExtractor.Extract` does NOT respect the empty-code guard (line 43-45) that the real implementation has -- it returns fixed values even for empty code. This is a test fidelity gap.

---

## 2. Volatility / Contamination

### Summary
Volatility is a **regex-based heuristic seeding** system followed by graph propagation and risk scoring. There is NO LLM involvement in contamination.

### Source Files
| File | Purpose |
|------|---------|
| `/home/jasondel/dev/graphdb-skill/cmd/graphdb/cmd_enrich_contamination.go` | CLI command entry point |
| `/home/jasondel/dev/graphdb-skill/internal/query/neo4j_contamination.go` | Neo4j Cypher implementations |
| `/home/jasondel/dev/graphdb-skill/internal/query/neo4j_contamination_test.go` | Integration tests |
| `/home/jasondel/dev/graphdb-skill/internal/query/interface.go` (lines 138-140) | Interface methods |

### Pipeline (3 phases, all in `cmd_enrich_contamination.go`)

**Phase 1: `SeedVolatility`** (neo4j_contamination.go, line 10-65)
- First cleans ALL legacy flags: `ui_contaminated`, `db_contaminated`, `io_contaminated`, `is_volatile`, `volatility_score` (line 13-14). This is a destructive wipe.
- Applies 16 hardcoded regex rules (cmd_enrich_contamination.go, lines 33-58):
  - External/Network: HttpClient, WebRequest, Socket, System.Net
  - Database/Storage: SQL patterns (SELECT FROM, INSERT INTO, UPDATE SET, DELETE FROM), DbContext, Repository
  - Non-determinism: DateTime.Now, DateTime.UtcNow, Guid.NewGuid, Random
  - UI/Framework: Controller, View, .aspx, .cshtml file patterns
- Two heuristic modes:
  - `"path"`: Matches against `file.file` or `f.name` path (line 30-35, 38-42)
  - `"content"`: Matches against `f.content` body (line 45-51) -- NOTE: this requires functions to have a `content` property in Neo4j, which is populated by the loader, not by the RPG pipeline.
- The `modulePattern` flag (default `".*"`) filters which files to process.

**Phase 2: `PropagateVolatility`** (neo4j_contamination.go, line 67-98)
- Walks the `CALLS` graph UPWARD: if a function calls a volatile callee, the caller becomes volatile.
- Runs in a loop with `LIMIT 5000` per batch until no more propagation occurs.
- Direction: `(caller)-[:CALLS]->(callee {is_volatile: true})` sets `caller.is_volatile = true`.

**Phase 3: `CalculateRiskScores`** (neo4j_contamination.go, line 100-158)
- Step 1: Calculates `volatility_score` = `1.0 / (distance_to_nearest_volatile + 1.0)` using `CALLS*0..2` path.
- Step 2: Calculates `raw_risk_score` = `fan_in * 0.4 + fan_out * 0.1 + volatility_score * 3.0 + churn * 0.4`
- Step 3: Normalizes to `[0.0, 1.0]` by dividing by the max raw score.
- Properties written: `is_volatile`, `volatility_score`, `risk_score` on Function nodes; uses `change_frequency` from File nodes.

### Consumers of Contamination Data
- **`GetSeams`** (neo4j.go, lines 537-603): Uses `is_volatile` for pinch-point detection (functions bridging non-volatile callers to volatile callees).
- **`GetHotspots`** (neo4j_history.go, lines 10-63): Reads `risk_score` and `change_frequency` to rank hotspots.
- **`GetImpact`** (neo4j.go, line 462-463): Returns `caller.is_volatile` in results.

### Key Risk: .NET-centric Rules
The 16 default rules are heavily .NET-centric (System.Net, DbContext, .aspx, .cshtml, DateTime.Now, Guid.NewGuid). These are baked into the Go source code, not configurable. For non-.NET codebases, most rules will match nothing. There is no plugin/config system for custom rules.

---

## 3. Clusterer

### Source Files
| File | Purpose |
|------|---------|
| `/home/jasondel/dev/graphdb-skill/internal/rpg/builder.go` (line 9-11) | Interface definition |
| `/home/jasondel/dev/graphdb-skill/internal/rpg/cluster_semantic.go` | `EmbeddingClusterer` (K-Means) |
| `/home/jasondel/dev/graphdb-skill/internal/rpg/cluster_global.go` | `GlobalEmbeddingClusterer` (wrapper) |
| `/home/jasondel/dev/graphdb-skill/internal/rpg/cluster_semantic_test.go` | K-Means tests |
| `/home/jasondel/dev/graphdb-skill/internal/rpg/cluster_global_test.go` | Global clusterer tests |

### Interface Signature (builder.go, line 9-12)
```go
type Clusterer interface {
    Cluster(nodes []graph.Node, domain string) (map[string][]graph.Node, error)
}
```
Returns `map[string][]graph.Node` where keys are cluster names (often "cluster-<UUID>").

### EmbeddingClusterer (cluster_semantic.go)

**Behavior:**
- If `len(nodes) <= 3`: returns ALL nodes in a single cluster named `"cluster-<UUID>"` (line 29-31). This is a **silent small-group collapse** -- no error, no warning.
- Uses precomputed embeddings (map by node ID). Falls back to `Embedder.EmbedBatch` if missing.
- **K determination** (lines 71-84):
  - Custom `KStrategy` function if set.
  - Default: `k = len(nodes) / 5` (target 5 per cluster).
  - Floor: `k >= 2`. Ceiling: `k <= len(nodes) / 2`.
- Runs K-Means with K-Means++ initialization (seeded RNG for determinism).
- **All cluster keys are UUID-based**: `"cluster-<UUID>"` (line 102). The `domain` parameter is completely ignored for naming.

**Silent UUID Fallback:** YES, absolutely. Every cluster produced by `EmbeddingClusterer` is named `"cluster-<8char_hex>"`. There is no attempt at semantic naming here. Semantic naming happens only in `GlobalEmbeddingClusterer`.

### GlobalEmbeddingClusterer (cluster_global.go)

**Behavior:**
- Wraps an inner `Clusterer` (the `EmbeddingClusterer`).
- For each raw cluster from the inner clusterer:
  1. Calculates centroid of cluster (line 76-111).
  2. Finds top 5 representative nodes closest to centroid (line 118-148).
  3. Collects code snippets from those representatives (line 150-177).
  4. Calls `Summarizer.Summarize(snippets)` to get a semantic name.
  5. On summarization failure: falls back to `"Domain-<UUID>"` (line 56). This is a **silent fallback** -- logged as warning, but the caller has no way to distinguish a real name from a fallback.
- Handles name uniqueness collisions by appending a counter (line 60-68).

### Error Handling Issues in Clusterer
1. **builder.go line 124**: `clusters, _ := b.Clusterer.Cluster(funcs, name)` -- errors from feature-level clustering are silently discarded.
2. **builder.go line 174**: `categories, _ := b.CategoryClusterer.Cluster(funcs, name)` -- same pattern.
3. **builder.go line 210**: `features, _ := b.Clusterer.Cluster(catNodes, catName)` -- same pattern.
4. All three callsites swallow errors. If clustering fails, the result is silently `nil`, leading to empty domains/features with no indication of failure.

---

## 4. Orchestrator

### Source
- **File:** `/home/jasondel/dev/graphdb-skill/internal/rpg/orchestrator.go`
- **Test:** `/home/jasondel/dev/graphdb-skill/internal/rpg/orchestrator_test.go`

### Struct (line 14-20)
```go
type Orchestrator struct {
    Provider   query.GraphProvider
    Extractor  FeatureExtractor
    Embedder   embedding.Embedder
    Summarizer Summarizer
    Seed       int64
}
```

### RunClustering (line 146-240) -- Data Flow

1. **Load embeddings**: `GetEmbeddingsOnly()` returns `map[nodeID][]float32`.
2. **Load function metadata**: `GetFunctionMetadata()` returns `[]*graph.Node`.
3. **Filter**: Only functions that have embeddings are used.
4. **Count unique files** from function metadata for K calculation.
5. **Build clusterers**:
   - `EmbeddingClusterer` as the feature-level clusterer.
   - Another `EmbeddingClusterer` as the inner domain clusterer (with `KStrategy` set to `CalculateDomainK(fileCount)` and `LogLabel: "domain"`).
   - `GlobalEmbeddingClusterer` wraps the inner domain clusterer, adding LLM summarization.
6. **Build**: Calls `Builder.Build(dir, functions)` which returns `([]Feature, []graph.Edge, error)`.
7. **Flatten**: Converts the Feature tree into flat `[]graph.Node` + `[]graph.Edge`.
8. **Write**: Calls `UpdateFeatureTopology(nodes, edges)`.

### Topology Cleanup: NONE
There is NO cleanup of old Feature/Domain nodes before writing new ones. `UpdateFeatureTopology` uses `MERGE (n:CodeElement {id: row.id})` (neo4j_batch.go, line 296) which merges by ID. Since IDs are UUID-based and regenerated every run, old topology is never cleaned up. Each clustering run creates a NEW parallel topology without deleting the old one.

### RunSummarization (line 242-310)

1. Counts unnamed features via `CountUnnamedFeatures()`.
2. For each unnamed feature:
   - Calls `ExploreDomain(node.ID)` to get member functions.
   - Creates an `Enricher` and calls `Enrich(feature, memberFuncs)`.
   - On failure: falls back to `"Feature-<UUID>"` with error description. **Two silent fallbacks** at lines 277 and 295.
   - On success: calls `UpdateFeatureSummary(id, name, description)`.

### CalculateDomainK (line 132-144)
```
k = floor(sqrt(fileCount / 5.0))
clamped to [5, 50]
```

---

## 5. Graph Provider (Feature/Domain Methods)

### Source
- **Primary:** `/home/jasondel/dev/graphdb-skill/internal/query/neo4j_batch.go`
- **Interface:** `/home/jasondel/dev/graphdb-skill/internal/query/interface.go`
- **Main provider:** `/home/jasondel/dev/graphdb-skill/internal/query/neo4j.go`

### CRITICAL BUG: `summary` vs `description` Property Mismatch

The system uses TWO DIFFERENT property names for what is conceptually the same thing:

| Component | Property Written | Where |
|-----------|-----------------|-------|
| `Feature.ToNode()` | `"description"` | schema.go line 30 |
| `UpdateFeatureSummary()` | `"summary"` | neo4j_batch.go line 379 |
| `GetUnnamedFeatures()` | checks `"summary"` | neo4j_batch.go line 199 |
| `CountUnnamedFeatures()` | checks `"summary"` | neo4j_batch.go line 229 |
| `VertexSummarizer.Summarize()` | returns `(name, description, error)` | enrich.go line 118 |
| `Orchestrator.RunSummarization()` | passes to `UpdateFeatureSummary(id, name, description)` | orchestrator.go line 300 |

**What happens:**
- During clustering (`RunClustering`), `Feature.ToNode()` writes `"description"` to the node.
- During summarization (`RunSummarization`), `UpdateFeatureSummary` writes `"summary"` to the node.
- `GetUnnamedFeatures` and `CountUnnamedFeatures` both check `coalesce(n.summary, '') = ''`, meaning they look for `"summary"`, NOT `"description"`.
- This means: if `ToNode()` writes a `description` during clustering, the summarization step will STILL consider the feature "unnamed" because `summary` is empty. The summarization step then overwrites with its own name and writes to `summary`. The `description` property from clustering remains as stale data on the node.

This is a **data coherence bug** where two properties hold competing values for the same concept.

### Key Methods

**`UpdateAtomicFeatures(id, features)`** (neo4j_batch.go, line 53-67)
- Sets `n.atomic_features = $features` on Function nodes.

**`UpdateFeatureTopology(nodes, edges)`** (neo4j_batch.go, line 245-259)
- Delegates to `batchWriteNodes` and `batchWriteEdges`.
- `batchWriteNodes` (line 261-315): Uses `MERGE (n:CodeElement {id: row.id})` then sets label via FOREACH (Domain or Feature). Properties are merged via `SET n += row`.
- `batchWriteEdges` (line 317-373): Groups edges by type, uses `MERGE (source)-[r:TYPE]->(target)`.
- Does NOT delete old topology before writing.

**`UpdateFeatureSummary(id, name, summary)`** (neo4j_batch.go, line 376-391)
- Writes `n.name = $name, n.summary = $summary` on Feature nodes.

**`GetUnnamedFeatures(limit)`** (neo4j_batch.go, line 196-223)
- Query: `WHERE coalesce(n.name, '') = '' OR coalesce(n.summary, '') = ''`
- A Feature with a name but no summary (or vice versa) is considered "unnamed".

**`CountUnnamedFeatures()`** (neo4j_batch.go, line 225-241)
- Same WHERE clause as GetUnnamedFeatures.

**`GetSeams(modulePattern, layer)`** (neo4j.go, line 537-603)
- Pinch-point detection: functions with high non-volatile fan-in AND high volatile fan-out.
- The `layer` parameter is accepted but NEVER USED in the query. It is dead code in the interface.

**`GetHotspots(modulePattern)`** (neo4j_history.go, line 10-63)
- Reads `risk_score` and `change_frequency`, sorts by product.
- Limited to 20 results.

**`ExploreDomain(featureID)`** (neo4j.go, line 811-891)
- Returns the feature, its parent, children, siblings, and implementing functions.
- Only matches `:Feature` label. Does NOT match `:Domain` label. Since domain nodes have label "Domain" (not "Feature"), calling `ExploreDomain` with a domain node ID would fail with "feature not found".

---

## 6. Enrich Commands

### Source
- **File:** `/home/jasondel/dev/graphdb-skill/cmd/graphdb/cmd_enrich_contamination.go`

### Behavior
- Accepts `--module` flag (regex pattern, default `".*"`).
- Hardcodes 16 contamination rules in Go source.
- Calls `SeedVolatility` -> `PropagateVolatility` -> `CalculateRiskScores` sequentially.
- No dry-run mode. No rule configuration file. No way to add custom rules without code changes.

### Other Enrich Commands (not in scope but noted)
| File | Purpose |
|------|---------|
| `/home/jasondel/dev/graphdb-skill/cmd/graphdb/cmd_enrich.go` | Main enrich command dispatcher |
| `/home/jasondel/dev/graphdb-skill/cmd/graphdb/cmd_enrich_tests.go` | Test enrichment |
| `/home/jasondel/dev/graphdb-skill/cmd/graphdb/cmd_enrich_history.go` | Git history enrichment |

---

## 7. ClusterGroup and Feature-UUID Generation

### Current State: ClusterGroup Does NOT Exist in Code
`ClusterGroup` is referenced ONLY in plan documents:
- `/home/jasondel/dev/graphdb-skill/plans/fix_domain_feature_topology.md`
- `/home/jasondel/dev/graphdb-skill/plans/fix_domain_and_contamination_architecture.md`

It is a PLANNED refactoring target that has NOT been implemented. The current `Clusterer` interface still returns `map[string][]graph.Node`.

### Feature-UUID Generation Patterns

The `GenerateShortUUID()` function (`naming.go`, line 102-110) generates 8-character hex strings from 4 random bytes using `crypto/rand`.

**Where UUIDs are generated:**

| Location | Pattern | Context |
|----------|---------|---------|
| `cluster_semantic.go:30` | `"cluster-" + UUID` | Small group (<=3 nodes) fallback |
| `cluster_semantic.go:102` | `"cluster-" + UUID` | All K-Means cluster keys |
| `cluster_global.go:56` | `"Domain-" + UUID` | Domain naming fallback on summarization failure |
| `builder.go:93` | `"domain-" + UUID` | Domain node IDs (always) |
| `builder.go:139` | `"feature-" + UUID` | Feature node IDs when name starts with "cluster-" or "Feature-" |
| `builder.go:188-189` | `"category-" + UUID` | Category node IDs (3-level hierarchy) |
| `builder.go:224` | `"feature-" + UUID` | Feature node IDs in 3-level hierarchy |
| `enrich.go:120` | `"Feature-" + UUID` | Fallback name when no snippets provided |
| `enrich_test.go:20` | `"Feature-" + UUID` | Mock mirrors the same fallback |
| `orchestrator.go:277` | `"Feature-" + UUID` | Fallback name when domain exploration fails |
| `orchestrator.go:295` | `"Feature-" + UUID` | Fallback name when enrichment fails |

**Key observation:** Every clustering run generates ALL NEW UUIDs. Since `UpdateFeatureTopology` uses `MERGE ... {id: row.id}`, and the IDs are always freshly generated, the MERGE never matches an existing node. This means every run creates a complete parallel set of Feature/Domain nodes without cleaning up old ones.

### UUID Detection in Builder (builder.go)
The builder has a pattern-matching heuristic to detect "unnamed" clusters:
```go
if strings.HasPrefix(originalKey, "cluster-") || strings.HasPrefix(originalKey, "root-cluster-") || strings.HasPrefix(originalKey, "Feature-") {
    domainName = GenerateDomainName(lca, nodes)
}
```
This means: if the GlobalClusterer returns a real semantic name (not starting with those prefixes), the builder trusts it. Otherwise, it falls back to LCA-based naming.

---

## 8. Data Flow Diagram

```
[Source Code]
    |
    v
[RunExtraction] -- LLM --> atomic_features[] on Function nodes
    |
    v
[RunEmbedding] -- Vertex AI Embeddings --> embedding[] on Function nodes
    |
    v
[RunClustering]
    |
    +-- GetEmbeddingsOnly() + GetFunctionMetadata()
    |
    +-- GlobalEmbeddingClusterer.Cluster() (K-Means -> LLM naming)
    |       |
    |       +-- EmbeddingClusterer.Cluster() (K-Means, returns "cluster-<UUID>" keys)
    |       +-- Summarizer.Summarize() (LLM, generates semantic names)
    |
    +-- Builder.Build()
    |       |
    |       +-- For each domain: EmbeddingClusterer.Cluster() (feature-level)
    |       +-- Generates Feature tree with IDs: "domain-<UUID>", "feature-<UUID>"
    |       +-- Feature.ToNode() writes "description" property
    |
    +-- Flatten() -> nodes + edges
    |
    +-- UpdateFeatureTopology() -> MERGE into Neo4j (no cleanup)
    |
    v
[RunSummarization]
    |
    +-- GetUnnamedFeatures() (checks "summary" property, NOT "description")
    +-- ExploreDomain() -> get member functions
    +-- Enricher.Enrich() -> LLM Summarize -> name + description
    +-- UpdateFeatureSummary() -> writes "summary" property (NOT "description")

[EnrichContamination] (separate pipeline)
    |
    +-- SeedVolatility() -- regex rules -> is_volatile on Functions
    +-- PropagateVolatility() -- graph walk -> more is_volatile
    +-- CalculateRiskScores() -> risk_score on Functions
```

---

## 9. Identified Bugs and Issues

### Critical

1. **`summary` vs `description` property mismatch** (Section 5): The Feature model writes `description`, but the DB queries read/write `summary`. These are different properties on the same node, causing data incoherence.

2. **No topology cleanup between clustering runs**: Every run generates new UUIDs, creating parallel Feature/Domain graphs without deleting old ones. Over multiple runs, the graph accumulates stale topology.

3. **Swallowed errors in Builder** (builder.go lines 124, 174, 210): All feature-level and category-level clustering errors are silently discarded with `clusters, _ := ...`.

### Major

4. **Silent UUID fallbacks**: When summarization fails, the system generates `"Feature-<UUID>"` or `"Domain-<UUID>"` names. The caller has no way to distinguish these from legitimate names programmatically, except by pattern matching the prefix.

5. **`ExploreDomain` only matches `:Feature` label**, not `:Domain`. Domain nodes (labeled "Domain") cannot be explored.

6. **`GetSeams` ignores the `layer` parameter**: Dead parameter in the interface.

7. **Contamination rules are hardcoded and .NET-specific**: No configuration mechanism for different tech stacks.

### Minor

8. **`EmbeddingClusterer` ignores the `domain` parameter entirely**: It is accepted but never used (line 23).

9. **`VertexSummarizer.Summarize` has retry logic** (3 retries with exponential backoff at enrich.go line 134-148), but `LLMFeatureExtractor.Extract` does NOT. Inconsistent resilience.

10. **`MockFeatureExtractor` does not respect empty-code guard**: Returns fixed values for empty code, unlike the real implementation.

---

## 10. Seam Identification (Cut Points)

### Recommended Interface Injection Points

1. **`Clusterer` interface** (builder.go line 9-12): This is the primary seam for the planned `ClusterGroup` refactoring. Change the return type from `map[string][]graph.Node` to `[]ClusterGroup` to carry descriptions alongside cluster assignments.

2. **`FeatureExtractor` interface** (extractor.go line 14-16): Already clean. No changes needed.

3. **`Summarizer` interface** (enrich.go line 18-20): Already clean. Consider splitting into `DomainSummarizer` and `FeatureSummarizer` if naming prompts diverge.

4. **`GraphProvider` interface** (interface.go line 108-154): Very large (35+ methods). Consider splitting into focused sub-interfaces:
   - `TopologyWriter` (UpdateFeatureTopology, UpdateFeatureSummary, UpdateAtomicFeatures, UpdateEmbeddings)
   - `TopologyReader` (GetUnnamedFeatures, CountUnnamedFeatures, GetFunctionMetadata, GetEmbeddingsOnly, ExploreDomain)
   - `ContaminationProvider` (SeedVolatility, PropagateVolatility, CalculateRiskScores)
   - `QueryProvider` (SearchFeatures, GetSeams, GetHotspots, etc.)

5. **Topology cleanup seam**: Before `UpdateFeatureTopology`, inject a `ClearFeatureTopology()` method that deletes all existing Feature/Domain nodes and their relationships.

6. **Contamination rules seam**: Replace hardcoded rules in `cmd_enrich_contamination.go` with a `ContaminationRuleProvider` interface or YAML/JSON config file.

---

## 11. Recommendations for Architect

1. **Fix the `summary`/`description` split FIRST.** This is a data coherence bug. Decide on one property name and use it consistently. The `Feature.ToNode()` method (schema.go) and `UpdateFeatureSummary` (neo4j_batch.go) must agree. Recommendation: use `description` everywhere since the LLM returns `{"name": "...", "description": "..."}`.

2. **Add topology cleanup to `RunClustering`.** Before writing new topology, delete old Feature/Domain nodes and their PARENT_OF/IMPLEMENTS edges. This prevents graph pollution across runs.

3. **Implement the `ClusterGroup` refactoring.** Change the `Clusterer` interface to return `[]ClusterGroup` carrying `Name`, `Description`, and `Nodes`. This eliminates the need for UUID-prefix detection in the Builder and carries LLM descriptions all the way through the pipeline.

4. **Fix error swallowing in Builder.** The three callsites that discard clustering errors (builder.go lines 124, 174, 210) should propagate errors. At minimum, log warnings.

5. **Isolate `GlobalEmbeddingClusterer` and `EmbeddingClusterer` for testing.** Both accept the `Clusterer` interface, making them already mockable. However, the `GlobalEmbeddingClusterer` also depends on `Summarizer` and `SourceLoader` -- these should be injected via constructor, not struct fields, to enforce initialization.

6. **Make contamination rules configurable.** Extract the rules from Go source into a config file or make them injectable via interface. This is lower priority but important for non-.NET codebases.

7. **Fix `ExploreDomain` to handle Domain nodes.** Change the Cypher to match both `:Feature` and `:Domain` labels, or add a separate `ExploreDomain` handler for Domain-labeled nodes.

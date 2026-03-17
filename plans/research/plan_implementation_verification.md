# Verification: fix_domain_feature_topology Plan Implementation Status

**Date:** 2026-03-16
**Plan:** `plans/fix_domain_feature_topology.md`
**Verdict:** NONE of the 8 proposed changes have been implemented. The codebase is in its pre-plan state.

---

## Item 1: `ClearFeatureTopology()` method

**Plan proposed:** Add `ClearFeatureTopology() error` to the `GraphProvider` interface and implement it in `neo4j_batch.go`.

**Current state: NOT IMPLEMENTED.**

- `internal/query/interface.go` (lines 108-154): The `GraphProvider` interface has no `ClearFeatureTopology` method. The interface ends with `UpdateFeatureSummary` at line 153.
- `internal/query/neo4j_batch.go`: Contains no function named `ClearFeatureTopology`. A grep across the entire codebase finds this identifier only in plan/research documents, never in `.go` source files.

---

## Item 2: `summary` to `description` rename

**Plan proposed:** Rename the `summary` property to `description` in Neo4j Cypher queries and the Go interface.

**Current state: NOT IMPLEMENTED. The code uses `summary` everywhere.**

- `internal/query/interface.go`, line 153: The interface signature is `UpdateFeatureSummary(id string, name string, summary string) error` -- the third parameter is still named `summary`.
- `internal/query/neo4j_batch.go`, line 376-391: `UpdateFeatureSummary` implementation sets `n.summary = $summary` in the Cypher query (line 379) and passes `"summary": summary` in the parameter map (line 384).
- `internal/query/neo4j_batch.go`, line 199: `GetUnnamedFeatures` checks `coalesce(n.summary, '') = ''` -- uses the `summary` property.
- `internal/query/neo4j_batch.go`, line 229: `CountUnnamedFeatures` checks `coalesce(n.summary, '') = ''` -- uses the `summary` property.
- `internal/rpg/orchestrator.go`, line 300: The call `o.Provider.UpdateFeatureSummary(node.ID, f.Name, f.Description)` passes `f.Description` as the `summary` argument, which then gets stored under the `summary` Neo4j property. This is the exact data-loss bug the plan describes: the Feature struct has a `Description` field (schema.go line 11), but it gets written to `n.summary` in Neo4j, while the frontend reads `n.description`.

---

## Item 3: Domain inclusion in summarization queries

**Plan proposed:** Change `GetUnnamedFeatures` and `CountUnnamedFeatures` to match `(n:Feature OR n:Domain)` instead of just `(n:Feature)`.

**Current state: NOT IMPLEMENTED. Both queries match only `(n:Feature)`.**

- `internal/query/neo4j_batch.go`, line 198: `GetUnnamedFeatures` uses `MATCH (n:Feature)`.
- `internal/query/neo4j_batch.go`, line 228: `CountUnnamedFeatures` uses `MATCH (n:Feature)`.
- Domain nodes are entirely skipped during the summarization phase.

---

## Item 4: `ClusterGroup` struct and `Clusterer` interface return type

**Plan proposed:** Define a `ClusterGroup` struct with `Name`, `Description`, and `Nodes` fields. Change the `Clusterer` interface to return `[]ClusterGroup` instead of `map[string][]graph.Node`.

**Current state: NOT IMPLEMENTED.**

- `internal/rpg/builder.go`, lines 9-12: The `Clusterer` interface is defined as:
  ```go
  type Clusterer interface {
      Cluster(nodes []graph.Node, domain string) (map[string][]graph.Node, error)
  }
  ```
  It returns `map[string][]graph.Node`. No `ClusterGroup` struct exists anywhere in the Go source.
- A grep for `ClusterGroup` across the entire repo returns hits only in plan and research markdown files, zero hits in `.go` files.

---

## Item 5: Fail-fast in `cluster_global.go`

**Plan proposed:** Remove the `Domain-<UUID>` fallback on summarization failure; return an error instead.

**Current state: NOT IMPLEMENTED. The UUID fallback is still active.**

- `internal/rpg/cluster_global.go`, lines 52-57:
  ```go
  name, _, err := c.Summarizer.Summarize(snippets)
  if err != nil {
      log.Printf("Warning: domain summarization failed: %v", err)
      // Fallback if summarization fails
      name = "Domain-" + GenerateShortUUID()
  }
  ```
  On error, the code logs a warning and assigns a `Domain-<UUID>` name. Execution continues. The error is swallowed.

---

## Item 6: Fail-fast in `orchestrator.go` `RunSummarization`

**Plan proposed:** Remove `Feature-<UUID>` fallbacks; return errors immediately on `ExploreDomain` or `enricher.Enrich` failure.

**Current state: NOT IMPLEMENTED. Both UUID fallbacks are still active.**

- `internal/rpg/orchestrator.go`, lines 274-279: When `ExploreDomain` fails:
  ```go
  if err != nil {
      log.Printf("Warning: failed to explore domain for %s: %v", node.ID, err)
      _ = o.Provider.UpdateFeatureSummary(node.ID, "Feature-"+GenerateShortUUID(), "Failed to analyze")
      pb.Add(1)
      continue
  }
  ```
  The error is logged as a warning, a `Feature-<UUID>` name is written, and the loop continues.

- `internal/rpg/orchestrator.go`, lines 293-298: When `enricher.Enrich` fails:
  ```go
  if err != nil {
      log.Printf("Warning: failed to enrich %s: %v", node.ID, err)
      _ = o.Provider.UpdateFeatureSummary(node.ID, "Feature-"+GenerateShortUUID(), "Enrichment failed")
      pb.Add(1)
      continue
  }
  ```
  Same pattern: warning log, UUID fallback name, continue.

---

## Item 7: Topology cleanup in `RunClustering`

**Plan proposed:** Call `ClearFeatureTopology()` at the beginning of `RunClustering` to make clustering idempotent.

**Current state: NOT IMPLEMENTED.**

- `internal/rpg/orchestrator.go`, lines 146-148: `RunClustering` begins by fetching embeddings. There is no cleanup call at the top. Since `ClearFeatureTopology` does not even exist on the interface (Item 1), this call cannot be present.
- Rerunning `RunClustering` will append duplicate Feature/Domain nodes to the graph without clearing stale data.

---

## Item 8: Builder error handling

**Plan proposed:** Stop swallowing errors from `b.Clusterer.Cluster(...)` and `b.CategoryClusterer.Cluster(...)` in the builder methods.

**Current state: NOT IMPLEMENTED. Errors are discarded with `_, _` (blank identifier) assignment.**

- `internal/rpg/builder.go`, line 124: `buildTwoLevel` discards the error:
  ```go
  clusters, _ := b.Clusterer.Cluster(funcs, name)
  ```
- `internal/rpg/builder.go`, line 174: `buildThreeLevel` discards the error from the category clusterer:
  ```go
  categories, _ := b.CategoryClusterer.Cluster(funcs, name)
  ```
- `internal/rpg/builder.go`, line 210: `buildThreeLevel` discards the error from the fine-grained clusterer:
  ```go
  features, _ := b.Clusterer.Cluster(catNodes, catName)
  ```
- All three sites silently ignore clustering failures. If the clusterer returns an error, the code proceeds with a nil or empty map, producing no children for that level of the hierarchy.

---

## Summary Table

| # | Change | Status | Key File:Line |
|---|--------|--------|--------------|
| 1 | `ClearFeatureTopology()` method | NOT DONE | interface.go:108-154 (absent) |
| 2 | `summary` to `description` rename | NOT DONE | neo4j_batch.go:199,229,379 (all use `summary`) |
| 3 | Domain inclusion in queries | NOT DONE | neo4j_batch.go:198,228 (match `n:Feature` only) |
| 4 | `ClusterGroup` struct | NOT DONE | builder.go:9-12 (returns `map[string][]graph.Node`) |
| 5 | Fail-fast in `cluster_global.go` | NOT DONE | cluster_global.go:52-57 (`Domain-<UUID>` fallback active) |
| 6 | Fail-fast in `orchestrator.go` | NOT DONE | orchestrator.go:274-279,293-298 (`Feature-<UUID>` fallbacks active) |
| 7 | Topology cleanup in `RunClustering` | NOT DONE | orchestrator.go:146 (no cleanup call) |
| 8 | Builder error handling | NOT DONE | builder.go:124,174,210 (errors discarded with `_`) |

---

## Recommendations for Architect

1. **The plan is entirely unstarted.** All 8 items remain in their original, pre-plan state. Implementation can begin from Phase 1, Step 1.A as written in the plan.

2. **The `summary` vs `description` data-loss bug is actively present.** Line 300 of `orchestrator.go` passes `f.Description` into `UpdateFeatureSummary`, which stores it under `n.summary` in Neo4j. Any frontend reading `n.description` will find nothing. This should be a high-priority fix.

3. **The silent failure pattern is actively present.** Both `orchestrator.go` (two sites) and `cluster_global.go` (one site) silently generate UUID-named garbage nodes when LLM calls fail. A quota-limited or erroring LLM will produce a graph full of `Feature-<UUID>` and `Domain-<UUID>` nodes across a 30-minute run.

4. **The duplicate topology problem is actively present.** `RunClustering` has no cleanup step. Every re-run appends a fresh set of Feature/Domain nodes without removing the previous set.

5. **The error-swallowing in `builder.go` is actively present.** Three call sites discard clusterer errors with blank identifiers, making clustering failures invisible.

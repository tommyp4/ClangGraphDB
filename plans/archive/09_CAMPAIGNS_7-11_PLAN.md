# Implementation Plan: Code Review Remediation

**Date:** 2026-02-27
**Source:** `plans/08_LEGACY_MODERNIZATION_REVIEW.md`
**Branch:** `oom-fixes`

---

## Overview

This plan addresses findings from the comprehensive code review, ordered by the user's stated priorities. Each item includes the problem, investigation findings, and concrete implementation steps.

---

## Item 1: Remove Dead `FindNode` Interface Method ✅ Implemented

**Problem:** `FindNode` is declared in `GraphProvider` interface (`internal/query/interface.go:69`), has a stub implementation returning `nil, nil` (`internal/query/neo4j.go:48-51`), and two mock stubs. Zero callers exist anywhere in the codebase.

**Action:** Remove `FindNode` from the interface, implementation, and all mocks.

**Files to modify:**
- `internal/query/interface.go` -- remove line 69
- `internal/query/neo4j.go` -- remove lines 47-51
- `cmd/graphdb/mocks.go` -- remove mock method
- `internal/rpg/orchestrator_test.go` -- remove mock method

**Effort:** Trivial (< 15 min)

---

## Item 2: Decompose Monolithic `main.go` ✅ Implemented

**Problem:** `cmd/graphdb/main.go` is 713 lines with all command handlers (`handleIngest`, `handleQuery`, `handleImport`, `handleEnrichFeatures`, `handleBuildAll`) in a single file. `handleQuery` alone is ~180 lines of switch/case.

**Action:** Extract each command handler into its own file within `cmd/graphdb/`.

**Proposed structure:**
```
cmd/graphdb/
├── main.go              # Entry point, printUsage(), command dispatch (~70 lines)
├── cmd_ingest.go        # handleIngest()
├── cmd_query.go         # handleQuery()
├── cmd_import.go        # handleImport(), processBatches()
├── cmd_enrich.go        # handleEnrichFeatures()
├── cmd_build_all.go     # handleBuildAll()
├── helpers.go           # getGitCommit(), execCommand var
├── setup_prod.go        # (existing) production setup
├── setup_mock_mode.go   # (existing) mock setup
├── mocks.go             # (existing) mock types
└── build_all_test.go    # (existing) tests
```

**Rules:**
- Pure file reorganization -- no logic changes
- All functions stay in `package main`
- Run `go build ./cmd/graphdb/` and `go test ./cmd/graphdb/` to verify
- Run `gofmt` on all new files to fix the inconsistent indentation (tabs/spaces) identified in the review

**Effort:** Small (~1 hour)

---

## Item 3: Implement Seam Identification & Contamination Propagation ✅ Implemented

### Current State (Investigation Findings)

The current seam detection is **extremely narrow**. Here's exactly what it does:

**`GetSeams()` Cypher query (`internal/query/neo4j.go:434-438`):**
```cypher
MATCH (caller:Function {ui_contaminated: true})
  -[:CALLS]->(f:Function {ui_contaminated: false})
  -[:DEFINED_IN]->(file:File)
WHERE file.file =~ $pattern
RETURN DISTINCT f.name as seam, file.file as file, f.risk_score as risk
```

This finds functions where a UI-contaminated caller transitions to a non-contaminated callee. That's **one specific type of seam** (UI/logic boundary). But there is a deeper problem:

**Nobody sets `ui_contaminated` or `risk_score`.** The old Node.js pipeline had two scripts that were never ported to the Go binary:
- `analyze_git_history.js` -- set `change_frequency` on File nodes and calculated `risk_score`
- `propagate_contamination.js` -- walked the CALLS graph from known UI entry points and set `ui_contaminated: true` on reachable functions

Both scripts are gone (the entire Node.js codebase was removed during the Go migration). The Go binary never implemented replacements. This means:
- `GetSeams()` always returns **zero results** on any graph built by the current Go pipeline
- `GetImpact()` returns `ui_contaminated: false` for every caller (the property simply doesn't exist on any node)
- The `hotspots` query type that existed in v1 was never ported to Go at all

**In Feathers' terms:** The tool currently has no real seam identification. It has the *query* for seams but none of the *data* that makes the query produce results.

### Action: Implement Full Seam Detection Pipeline

Seams in Feathers' framework are broader than just UI contamination. A seam is any point where behavior can be altered without editing the code at that point. For legacy modernization, we should detect multiple seam *types*:

#### Step 1: Add `enrich-contamination` CLI command

New command: `graphdb enrich-contamination -dir <path> [--rules <rules.json>]`

**Default contamination rules (built-in):**
| Layer | Detection Heuristic | Property Set |
|:---|:---|:---|
| **UI** | File path matches `*Controller*`, `*View*`, `*Form*`, `*.aspx`, `*.cshtml`; or function calls known UI APIs (MFC, WinForms, WPF patterns) | `ui_contaminated: true` |
| **Database** | Function body contains SQL keywords (`SELECT`, `INSERT`, `UPDATE`, `DELETE`), or calls known ORM patterns (`DbContext`, `Repository`, `DataAdapter`) | `db_contaminated: true` |
| **External I/O** | Calls to `HttpClient`, `WebRequest`, `Socket`, file system APIs | `io_contaminated: true` |

**Propagation algorithm:**
1. **Seed phase:** Query all Function nodes; apply heuristic rules to seed initial contamination flags
2. **Propagation phase:** Walk CALLS graph forward (BFS). Any function called by a contaminated function inherits that contamination type
3. **Write phase:** Batch-update nodes with contamination flags via `UNWIND`

#### Step 2: Broaden `GetSeams()` to multi-layer

Replace the single `ui_contaminated` query with a configurable seam finder:

```cypher
// Find functions where contamination stops (transition points)
MATCH (caller:Function)-[:CALLS]->(f:Function)-[:DEFINED_IN]->(file:File)
WHERE caller[$contaminationType] = true AND (f[$contaminationType] IS NULL OR f[$contaminationType] = false)
  AND file.file =~ $pattern
RETURN DISTINCT f.name as seam, file.file as file, f.risk_score as risk, $contaminationType as seam_type
```

Add `-layer` flag to CLI: `graphdb query -type seams -module ".*" -layer ui` (or `db`, `io`, `all`)

#### Step 3: Add `risk_score` calculation

`risk_score` for a function should combine:
- **Fan-in** (number of callers) -- high fan-in = high impact of change
- **Fan-out** (number of callees) -- high fan-out = high complexity
- **Contamination count** -- how many layers does it touch

```
risk_score = normalize(fan_in * 0.4 + fan_out * 0.3 + contamination_layers * 0.3)
```

**Files to create/modify:**
- `cmd/graphdb/cmd_enrich_contamination.go` -- new CLI command
- `internal/query/neo4j_contamination.go` -- contamination queries and propagation
- `internal/query/neo4j.go` -- update `GetSeams()` to accept layer parameter
- `internal/query/interface.go` -- update `GetSeams` signature or add new method

**Effort:** Large (~3-4 days)

---

## Item 4: Restore Git History Analysis & Incremental Ingestion ✅ Implemented

### Current State (Investigation Findings)

You were right -- this capability **did exist and was lost during the Node.js to Go migration.** The old pipeline had:

- **`analyze_git_history.js`** (Step 3 of the old ingestion pipeline, per `.gemini/skills/graphdb/README.md:88`)
  - Set `change_frequency` on File nodes
  - Enabled the `hotspots` query type (listed in old README line 117)
- **`hotspots` query type** -- combined complexity + change frequency to find high-risk code

Neither was ported to Go. The current Go binary has no `hotspots` query type and no git history integration. The `File` nodes in the current schema have no `change_frequency` property.

### Action: Two sub-items

#### Step 4a: Add `enrich-history` CLI command

New command: `graphdb enrich-history -dir <path> [-since <date>]`

**Implementation:**
1. Run `git log --follow --format="%H" --name-only` to get per-file change counts
2. Run `git log --format="%H" --name-only` with pairwise analysis for co-change detection (files that change together frequently)
3. Update File nodes with:
   - `change_frequency: <int>` -- total commits touching this file
   - `last_changed: <date>` -- most recent commit date
   - `co_changes: [<file_ids>]` -- files that frequently change together (above threshold)
4. Calculate and set `risk_score` on Function nodes using fan-in + change_frequency

**Restore `hotspots` query type:**
```cypher
MATCH (f:Function)-[:DEFINED_IN]->(file:File)
WHERE file.file =~ $pattern
RETURN f.name as name, file.file as file, f.risk_score as risk,
       file.change_frequency as churn
ORDER BY f.risk_score DESC
LIMIT 20
```

#### Step 4b: Add incremental ingestion via `--since-commit`

Add flag: `graphdb ingest -dir . --since-commit <hash>`

No `-nodes`/`-edges` output files -- those are ephemeral intermediates only useful for the initial full-build pipeline. Incremental updates should write directly to Neo4j, skipping the JSONL round-trip entirely.

**Implementation:**
1. Run `git diff --name-only <hash>..HEAD` to get changed files
2. Filter to supported extensions
3. Parse via existing walker/parsers
4. Write resulting nodes and edges **directly to Neo4j** via `loader.Neo4jLoader` (upsert via `MERGE` -- the existing Cypher already uses `MERGE`, so re-ingesting changed files is naturally idempotent)

**Design:** The walker currently emits to a `storage.Emitter` (JSONL files). For incremental mode, either:
- Add a `Neo4jEmitter` implementing the `storage.Emitter` interface that calls `BatchLoadNodes`/`BatchLoadEdges` directly, or
- Have the incremental path bypass the emitter and call the loader inline

The first option is cleaner -- it reuses the existing worker pool and emitter abstraction. The walker doesn't need to know whether it's writing to files or to Neo4j.

**Auto-detection:** If `--since-commit` is omitted, check `GraphState.commit` from the database. If it matches a reachable ancestor of `HEAD`, use that as the baseline automatically. This makes the common case zero-config: `graphdb ingest -dir .` does a full build on first run, incremental on subsequent runs.

**Files to create/modify:**
- `cmd/graphdb/cmd_enrich_history.go` -- new CLI command
- `internal/query/neo4j_history.go` -- git history queries and update logic
- `internal/storage/neo4j_emitter.go` -- new `Emitter` implementation backed by Neo4j loader
- `cmd/graphdb/cmd_ingest.go` -- add `--since-commit` flag, auto-detection logic
- `cmd/graphdb/cmd_query.go` -- add `hotspots` case to switch

**Effort:** Medium (~2 days)

---

## Item 5: Test Coverage Integration ✅ Implemented

**Problem:** Understanding which functions have tests is critical for Feathers' "characterization test" workflow. The tool doesn't know which functions are tested.

**Action:** Add test-awareness to the ingestion pipeline.

**Implementation:**
1. During ingestion, detect test files by convention:
   - `*_test.go`, `*Test.java`, `*Tests.cs`, `*.test.ts`, `*.spec.ts`
2. Parse test files normally (they already would be), but tag resulting Function nodes with `is_test: true`
3. Detect test-to-production linkage:
   - **Naming convention:** `TestFoo` tests `Foo`, `FooTests` tests `Foo`
   - **Import/using analysis:** Test file imports from production module
4. Create `TESTS` edges: `(testFunc)-[:TESTS]->(productionFunc)`
5. Add query: `graphdb query -type coverage -target <function>` -- returns whether a function has tests, and which ones

**Files to create/modify:**
- `internal/analysis/parser.go` -- add `IsTestFile(path string) bool` utility
- `internal/ingest/worker.go` -- tag test nodes during processing
- `internal/query/neo4j.go` -- add coverage query
- `cmd/graphdb/cmd_query.go` -- add `coverage` case

**Effort:** Medium (~2 days)

---

## Item 6: "What-If" Query Mode ✅ Implemented

**Problem:** Agents performing Strangler Fig extraction need to ask "If I extract these functions to a new service, what breaks?" Currently there's no way to simulate graph modifications.

**Action:** Add a `what-if` query type that computes the impact of hypothetical node removals without modifying the database.

**Implementation:**
```
graphdb query -type what-if -target "Namespace.Class" [-target2 "Namespace.Class2"]
```

**Algorithm:**
1. Resolve target node(s)
2. Find all incoming edges to target(s) -- these are "severed connections"
3. For each severed connection, determine if the source has alternative paths to reach the same callees (redundancy check)
4. Return:
   - `severed_edges: [...]` -- all edges that would break
   - `orphaned_nodes: [...]` -- nodes that become unreachable from any non-extracted node
   - `cross_boundary_calls: [...]` -- calls from non-extracted code into extracted code (these need an API)
   - `shared_state: [...]` -- globals used by both extracted and remaining code

**This is a read-only graph query** -- no mutations. It uses the existing CALLS/USES_GLOBAL edges to compute the extraction impact.

**Files to create/modify:**
- `internal/query/neo4j_whatif.go` -- what-if analysis logic
- `internal/query/interface.go` -- add `WhatIf` method
- `cmd/graphdb/cmd_query.go` -- add `what-if` case

**Effort:** Large (~3 days)

---

## Item 7: Priority 2 Quality & Reliability Fixes ✅ Implemented

These are the smaller items from the code review that improve correctness and performance.

| # | Action | Files | Effort |
|:---|:---|:---|:---|
| 7a | **Add deterministic seed to K-Means** -- Add `--seed` flag to `enrich-features`. Pass seed to `kmeansppInit` in `cluster_semantic.go`. | `cmd/graphdb/cmd_enrich.go`, `internal/rpg/cluster_semantic.go` | Small |
| 7b | **Batch `UpdateFeatureTopology`** -- Replace per-node Cypher with UNWIND-based batch insert (matching pattern in `neo4j_loader.go`). | `internal/query/neo4j_batch.go` | Small |
| 7c | **Fix `GetUnextractedFunctions` filter** -- Change `n.content IS NOT NULL` to `n.file IS NOT NULL AND n.start_line IS NOT NULL`. | `internal/query/neo4j_batch.go:15` | Trivial |
| 7d | **Sanitize Cypher-injected labels** -- Expand `sanitizeLabel()` to strip all non-alphanumeric/underscore characters, not just backticks. | `internal/loader/neo4j_loader.go` | Trivial |
| 7e | **Normalize main.go formatting** -- Covered by Item 2 (gofmt during decomposition). | -- | -- |

---

## Execution Order

```
Item 1 (FindNode removal)          ~15 min     -- quick cleanup
Item 2 (main.go decomposition)     ~1 hour     -- sets up clean file structure for all subsequent work
Item 7 (Quality fixes)             ~2 hours    -- small fixes, good to batch together
Item 3 (Seam detection)            ~3-4 days   -- highest value new capability
Item 4 (Git history + incremental) ~2 days     -- restores lost functionality
Item 5 (Test coverage)             ~2 days     -- Feathers workflow enabler
Item 6 (What-if analysis)          ~3 days     -- advanced extraction planning
```

Items 1, 2, and 7 are prerequisites/cleanup that should be done first. Items 3-6 can be parallelized across contributors if needed.

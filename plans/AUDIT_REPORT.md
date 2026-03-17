# Audit Report: Domain and Contamination Architecture Overhaul

**Date:** 2026-03-17
**Plan:** `plans/fix_domain_and_contamination_architecture.md`
**Scope:** Phases 1-5 implementation verification

## Summary

| Phase | Status | Issues |
|---|---|---|
| Phase 1: LLM-Driven Volatility Extraction | PASS | None |
| Phase 2: Contamination Command & Pre-flight Checks | FAIL | Incorrect error message in `GetSeams` |
| Phase 3: Topology Idempotency & Fail-Fast | PASS | Minor: no explicit clusterer error propagation test |
| Phase 4: Property Alignment & Domain Summarization | FAIL | Test fixtures missing Domain nodes |
| Phase 5: Remove 3-Level Hierarchy Dead Code | PASS | None |

## Phase 1: PASS

All 7 requirements verified:

- `FeatureExtractor.Extract` returns `([]string, bool, error)` — `extractor.go:16`
- LLM prompt requests volatility assessment, expects `{"descriptors": [...], "is_volatile": true}` — `extractor.go:58-77`
- `MockFeatureExtractor` updated with `IsVolatile` field — `extractor.go:116-125`
- `UpdateAtomicFeatures` accepts `isVolatile bool`, writes to Function node — `neo4j_batch.go:53-62`
- `GraphProvider` interface updated — `interface.go:146`
- `RunExtraction` captures and passes `isVolatile` — `orchestrator.go:67,78`
- All mocks updated — `cmd/graphdb/mocks.go:94`, `orchestrator_test.go:12,82`

## Phase 2: FAIL

### Defect: GetSeams error message directs user to wrong command

**Location:** `internal/query/neo4j.go` (lines 545, 549)

**Current:**
```
volatility data is missing. Run 'graphdb enrich --step extract' first
```

**Required:**
```
Volatility data is missing. Run 'graphdb enrich-contamination' first.
```

**Reasoning:** `GetSeams` is a consumer of propagated volatility data. Extraction seeds `is_volatile` on individual functions, but `GetSeams` needs the propagated flags that come from `enrich-contamination` (which runs `PropagateVolatility` + `CalculateRiskScores`). Directing users to extraction alone would leave them with un-propagated data that still produces empty seam results.

**Test also wrong:** `internal/query/neo4j_test.go` (line 260) asserts the incorrect message, so the test passes against the wrong behavior.

### Passing items
- `GetHotspots` pre-flight check correct — `neo4j_history.go:12-23`
- Legacy regex `SeedVolatility` removed from `cmd_enrich_contamination.go`
- Contamination command only calls `PropagateVolatility()` + `CalculateRiskScores()` — `cmd_enrich_contamination.go:35,40`
- Transitional guard checks for zero `is_volatile` flags — `cmd_enrich_contamination.go:26-31`

## Phase 3: PASS

All requirements verified:

- `Clusterer` interface returns `[]ClusterGroup` with `Name`, `Description`, `Nodes` — `builder.go`
- `GlobalEmbeddingClusterer` passes LLM description into `ClusterGroup`, `Domain-<UUID>` fallback removed, returns error on failure — `cluster_global.go`
- `EmbeddingClusterer` updated for new return type — `cluster_semantic.go`
- Builder error propagation fixed: `clusters, _ :=` replaced with proper error handling — `builder.go:42-44,115-118`
- `ClearFeatureTopology()` added to `GraphProvider` interface — `interface.go`
- `ClearFeatureTopology()` implemented with `MATCH (n) WHERE n:Feature OR n:Domain DETACH DELETE n` — `neo4j_batch.go`
- Mocks updated — `cmd/graphdb/mocks.go`, `orchestrator_test.go`
- `RunClustering` calls `ClearFeatureTopology()` as first operation — `orchestrator.go:148`

**Minor note:** No explicit test case for clusterer error propagation through the builder. Implementation is correct but untested.

## Phase 4: FAIL

### Defect: Test fixtures do not include Domain nodes

All production code changes are correct:
- `UpdateFeatureSummary` writes `n.description` instead of `n.summary` — `neo4j_batch.go:392`
- `GetUnnamedFeatures` matches `(n) WHERE n:Feature OR n:Domain` and checks `n.description` — `neo4j_batch.go:198-203`
- `CountUnnamedFeatures` same pattern — `neo4j_batch.go:229-233`
- `ExploreDomain` matches `(f {id: $featureID}) WHERE f:Feature OR f:Domain` — `neo4j.go:830-831`

**However, both test files only create Feature nodes in their fixtures:**

1. `internal/query/neo4j_batch_test.go` — test fixture creates `feat1:Feature`, `feat2:Feature`, `feat3:Feature`, `feat4:Feature`. No Domain nodes. The `Feature OR Domain` query broadening is untested for the Domain case.

2. `internal/query/neo4j_explore_test.go` — test fixture creates `top:Feature`, `cat:Feature`, `feat:Feature`. No Domain nodes. `ExploreDomain` is never tested with a Domain node ID.

**Impact:** The plan's success criteria states "Domain Visibility: Both GetUnnamedFeatures and ExploreDomain correctly operate on Domain nodes, not just Features." This cannot be verified without Domain nodes in the test data.

## Phase 5: PASS

All requirements verified:

- `CategoryClusterer` field removed from `Builder` struct — `builder.go`
- `buildThreeLevel` method deleted — `builder.go` (157 lines total, only `Build`, `buildGlobal`, `buildTwoLevel` remain)
- `TestBuilder_BuildThreeLevel` removed — `builder_test.go`
- No `category-` prefix references remain in production Go code
- Remaining `cluster-` prefix checks in `builder.go:76,123` are legitimate for 2-level hierarchy logic

## Required Fixes

1. **GetSeams error message** — Update `internal/query/neo4j.go` (lines 545, 549) and the corresponding test assertion in `internal/query/neo4j_test.go` (line 260).

2. **Phase 4 test fixtures** — Add Domain nodes to `internal/query/neo4j_batch_test.go` and `internal/query/neo4j_explore_test.go` to verify the Feature OR Domain queries work with Domain data.

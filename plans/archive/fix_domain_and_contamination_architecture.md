# Feature Implementation Plan: Domain and Contamination Architecture Overhaul

## Todo Checklist
- [x] Phase 1: Overhaul Contamination Layer Extraction (LLM-Driven Volatility) ✅
- [x] Phase 2: Modernize Contamination Command & Query Pre-flight Checks ✅
- [x] Phase 3: Enforce Topology Idempotency, Fail-Fast Clustering & Builder Error Propagation ✅ Implemented
- [x] Phase 4: Property Alignment, Domain Summarization & ExploreDomain Fix ✅ Implemented
- [x] Phase 5: Remove 3-Level Hierarchy Dead Code ✅
- [x] Phase 6: Execute Data Recovery Strategy ✅
- [ ] Final Review and Data Validation

## Analysis & Investigation
The current architecture relies on brittle heuristics (regex-based volatility seeding) and implements silent fallbacks during clustering errors (e.g., generating `Feature-<UUID>` on LLM failure). This violates the core philosophy of maintaining a strict, hygienic graph state.

**Key Findings & Required Architectural Shifts:**
1.  **Semantic Volatility:** Regex cannot reliably detect side effects (UI, DB, Network, I/O). The LLM is already parsing the function for features; it should simultaneously evaluate volatility as a boolean flag. The current 16 hardcoded rules in `cmd/graphdb/cmd_enrich_contamination.go:33-58` are .NET/C#-specific and do not generalize.
2.  **Fail-Fast Execution:** Silent fallbacks hide integration issues (like Vertex AI quotas or timeouts). When semantic clustering fails, the process must halt. This includes:
    - `cluster_global.go:54-56`: Falls back to `Domain-<UUID>` on summarization failure
    - `builder.go:124,174,210`: Swallows ALL errors from `Clusterer.Cluster()` calls with `_, _`
    - `orchestrator.go:277,295`: Falls back to `Feature-<UUID>` on enrichment failure
3.  **Idempotent Clustering:** Re-running clustering currently creates stale duplicate topologies. We must explicitly clear `Feature` and `Domain` nodes before regenerating the hierarchy.
4.  **Property Alignment:** The schema's `ToNode()` writes `description` (`schema.go:30`) but `UpdateFeatureSummary` writes `summary` (`neo4j_batch.go:379`). Queries check `summary` (`neo4j_batch.go:199,229`). These properties never align.
5.  **Domain Summarization Gap:** `GetUnnamedFeatures` matches only `(n:Feature)` (`neo4j_batch.go:198`), skipping all Domain nodes. Additionally, `ExploreDomain` (`neo4j.go:816`) matches only `MATCH (f:Feature {id: ...})`, so Domain node lookups fail with "feature not found" during summarization.

## Implementation Plan

### Prerequisites
*   Ensure Neo4j database is running and accessible.
*   Ensure Vertex AI credentials/environment variables are correctly configured for testing.

### Step-by-Step Implementation

#### Phase 1: Contamination Layer Extraction (LLM-Driven Volatility)
1.  **Step 1.A (The Harness):** Define verification for LLM extractor and graph provider.
    *   *Action:* Update `internal/rpg/extractor_test.go` (create if needed) and `internal/query/neo4j_batch_test.go`.
    *   *Goal:* Assert `FeatureExtractor.Extract` returns `([]string, bool, error)` and `UpdateAtomicFeatures` correctly persists the `is_volatile` boolean.
2.  **Step 1.B (The Implementation):** Refactor `FeatureExtractor` interface and LLM implementation.
    *   *Action:* Modify `internal/rpg/extractor.go`.
    *   *Detail:* Change `Extract(code string, functionName string)` to return `([]string, bool, error)`. Update the prompt to ask the LLM to identify if the function interacts with UI, DB, Network, File I/O, or non-deterministic state. Change the expected JSON format to `{"descriptors": ["..."], "is_volatile": true}`. Update `MockFeatureExtractor` to match.
3.  **Step 1.C (The Implementation):** Update Database Write in Provider.
    *   *Action:* Modify `UpdateAtomicFeatures` in `internal/query/neo4j_batch.go`.
    *   *Detail:* Accept the new `is_volatile` boolean and write it directly to the `Function` node during the `enrich --step extract` phase. Update the `GraphProvider` interface in `internal/query/interface.go`.
4.  **Step 1.D (The Implementation):** Update Orchestrator extraction loop.
    *   *Action:* Modify `RunExtraction` in `internal/rpg/orchestrator.go`.
    *   *Detail:* Capture the `isVolatile` return from `Extract()` and pass it to `UpdateAtomicFeatures`.
5.  **Step 1.E (The Verification):** Verify extraction changes.
    *   *Action:* Run `go test ./internal/rpg/... ./internal/query/... -run TestExtractor` and ensure it passes.

#### Phase 2: Modernize Contamination Command & Pre-flight Checks
1.  **Step 2.A (The Harness):** Define verification for query guards and command modernization. ✅ Added tests in neo4j_test.go and neo4j_history_test.go.
    *   *Action:* Update `internal/query/neo4j_test.go`.
    *   *Goal:* Assert pre-flight checks fail when required data is missing.
2.  **Step 2.B (The Implementation):** Add Query Pre-flight Checks (Fail Fast).
    *   *Action:* Modify `GetSeams` in `internal/query/neo4j.go` and `GetHotspots` in `internal/query/neo4j_history.go`.
    *   *Detail for `GetSeams`:* This query depends on `is_volatile` (not `risk_score`). Add a check: `MATCH (f:Function) WHERE f.is_volatile IS NOT NULL RETURN count(f) LIMIT 1`. If 0, return error: "Volatility data is missing. Run 'graphdb enrich-contamination' first."
    *   *Detail for `GetHotspots`:* This query depends on `risk_score`. Add a check: `MATCH (f:Function) WHERE f.risk_score IS NOT NULL RETURN count(f) LIMIT 1`. If 0, return error: "Risk score data is missing. Run 'graphdb enrich-contamination' first."
3.  **Step 2.C (The Implementation):** Modernize `enrich-contamination` Command.
    *   *Action:* Modify `cmd/graphdb/cmd_enrich_contamination.go`.
    *   *Detail:* Remove the legacy regex `SeedVolatility` logic and the hardcoded rules entirely. The command should now ONLY call `PropagateVolatility()` and `CalculateRiskScores()`. Add a transitional guard: if zero `is_volatile` flags exist in the graph, print a clear error directing the user to run `graphdb enrich --step extract` first (rather than silently propagating nothing).
4.  **Step 2.D (The Verification):** Verify pre-flight and command changes.
    *   *Action:* Run `go test ./internal/query/... -run TestGetSeams|TestGetHotspots` and ensure it passes.

#### Phase 3: Topology Cleanup, Fail-Fast Clustering & Builder Error Propagation
1.  **Step 3.A (The Harness):** Define verification for fail-fast clustering, topology clearing, and builder error propagation.
    *   *Action:* Update `internal/rpg/builder_test.go` and `internal/rpg/orchestrator_test.go`.
    *   *Goal:* Assert `Clusterer` errors bubble up correctly (no UUID fallback), `ClearFeatureTopology` executes successfully, and builder methods propagate clusterer errors.
2.  **Step 3.B (The Implementation):** ClusterGroup Refactor and Fail-Fast Error Handling.
    *   *Action:* Modify `internal/rpg/builder.go` (interface + struct), `internal/rpg/cluster_semantic.go`, and `internal/rpg/cluster_global.go`.
    *   *Detail:* Refactor `Clusterer` interface to return `[]ClusterGroup` (struct holding `Name`, `Description`, and `Nodes`). Update `GlobalEmbeddingClusterer` to pass the LLM-generated description into this struct. Remove silent fallback (generating `Domain-<UUID>` at `cluster_global.go:56`); return `fmt.Errorf` on Vertex AI error to halt the process.
3.  **Step 3.C (The Implementation):** Fix Builder Error Propagation.
    *   *Action:* Modify `internal/rpg/builder.go`.
    *   *Detail:* At lines 124, 174, and 210, change `clusters, _ := b.Clusterer.Cluster(...)` to properly handle errors:
        ```go
        clusters, err := b.Clusterer.Cluster(funcs, name)
        if err != nil {
            return nil, fmt.Errorf("feature clustering failed for domain %s: %w", name, err)
        }
        ```
        Apply the same pattern to `CategoryClusterer.Cluster()` calls (until Phase 5 removes them).
4.  **Step 3.D (The Implementation):** Implement Topology Cleanup.
    *   *Action:* Add `ClearFeatureTopology() error` to the `GraphProvider` interface in `internal/query/interface.go`. Implement in `internal/query/neo4j_batch.go` executing `MATCH (n) WHERE n:Feature OR n:Domain DETACH DELETE n`. Update mocks in `cmd/graphdb/mocks.go` and `internal/rpg/orchestrator_test.go`.
    *   *Detail:* Call `ClearFeatureTopology()` at the very beginning of `RunClustering` in `internal/rpg/orchestrator.go`.
5.  **Step 3.E (The Verification):** Verify fail-fast and idempotency.
    *   *Action:* Run `go test ./internal/rpg/... ./internal/query/... -run TestCluster|TestBuilder|TestRunClustering` and ensure it passes.

#### Phase 4: Property Alignment, Domain Summarization & ExploreDomain Fix
1.  **Step 4.A (The Harness):** Define verification for property alignment.
    *   *Action:* Update `internal/query/neo4j_batch_test.go` and `internal/query/neo4j_explore_test.go`.
    *   *Goal:* Assert `description` is queried/updated correctly, Domains are included in summarization queries, and `ExploreDomain` can find both Feature and Domain nodes.
2.  **Step 4.B (The Implementation):** Rename Properties and Broaden Queries.
    *   *Action:* Modify Cypher queries in `internal/query/neo4j_batch.go`.
    *   *Detail:* In `UpdateFeatureSummary` (line 379), rename `n.summary` to `n.description`. In `GetUnnamedFeatures` and `CountUnnamedFeatures` (lines 199, 229), change `n.summary` to `n.description` and update the MATCH clause from `(n:Feature)` to `(n) WHERE n:Feature OR n:Domain`.
3.  **Step 4.C (The Implementation):** Fix `ExploreDomain` to match both labels.
    *   *Action:* Modify `internal/query/neo4j.go` at line 816.
    *   *Detail:* Change `MATCH (f:Feature {id: $featureID})` to `MATCH (f {id: $featureID}) WHERE f:Feature OR f:Domain`. **Critical:** without this fix, Domain node IDs returned by the fixed `GetUnnamedFeatures` would fail with "feature not found" during summarization.
4.  **Step 4.D (The Verification):** Verify query updates.
    *   *Action:* Run `go test ./internal/query/... -run TestUnnamed|TestFeatureSummary|TestExploreDomain` and ensure it passes.

#### Phase 5: Remove 3-Level Hierarchy Dead Code

The `CategoryClusterer` field on `Builder` and the `buildThreeLevel` method are dead code that was never wired in production. Keeping it adds confusion and maintenance burden without serving the core Feathers methodology (seam detection, impact analysis, contamination scoring all operate on the call graph, not the hierarchy).

1.  Remove `CategoryClusterer` field from `Builder` struct in `internal/rpg/builder.go`.
2.  Delete `buildThreeLevel` method from `internal/rpg/builder.go`.
3.  Remove the corresponding test (`TestBuilder_BuildThreeLevel` in `internal/rpg/builder_test.go`).
4.  Remove any `cluster-` or `category-` prefix checks in `builder.go` that only existed for this path.
5.  Verify with `go test ./internal/rpg/...`.

#### Phase 6: Data Recovery & Execution Strategy
Since extraction payloads have changed drastically (incorporating LLM-based volatility), the graph must be re-enriched from scratch to achieve high-fidelity data.

*Execution Sequence to be run manually after deployment:*
```bash
# 1. Re-run Extraction (LLM-Driven Volatility seeding)
graphdb enrich --step extract

# 2. Re-run Embeddings
graphdb enrich --step embed

# 3. Re-run Clustering (This will now auto-clear old Topology)
graphdb enrich --step cluster

# 4. Generate Summaries for both Features and Domains
graphdb enrich --step summarize

# 5. Propagate Risk/Contamination Scores
graphdb enrich-contamination
```

### Testing Strategy
1.  **Unit Tests:** Verify that interface changes correctly bubble up `is_volatile` flags and errors instead of swallowing them.
2.  **Integration Tests:** Verify Cypher queries correctly delete topology (`ClearFeatureTopology`), correctly update properties (`description` not `summary`), and correctly find both Feature and Domain nodes in `ExploreDomain`.
3.  **E2E Simulation:** Run the CLI sequence on a small test repository to ensure `enrich-contamination` successfully operates on LLM-seeded volatility, and that `query seams` correctly rejects execution if volatility data is missing.

## Success Criteria
*   **No Regex Heuristics:** Volatility is 100% determined by the LLM during the extraction phase.
*   **Strict Adherence to Fail-Fast:** Any LLM API failure during clustering completely halts execution with an explicit error. Builder error swallowing at `builder.go:124,174,210` is eliminated.
*   **Idempotency:** Re-running `graphdb enrich --step cluster` results in exactly 1 set of Features and Domains (no duplicates).
*   **Property Alignment:** The UI accurately renders descriptions for both Features and Domains (using `description`, not `summary`).
*   **Domain Visibility:** Both `GetUnnamedFeatures` and `ExploreDomain` correctly operate on Domain nodes, not just Features.
*   **Query Safety:** Seam queries reject when `is_volatile` is missing. Hotspot queries reject when `risk_score` is missing. Each with a clear error message.
*   **No Dead Code:** 3-level hierarchy code (`CategoryClusterer`, `buildThreeLevel`) is removed.

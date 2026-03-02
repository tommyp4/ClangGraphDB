# Rejection Report: Item 4 - Restore Git History Analysis & Incremental Ingestion

**Status:** REJECTED ❌

## Reasons for Rejection

### 1. Missing Unit Tests (CRITICAL FAILURE)
The Modernization Doctrine strictly mandates: "NO CODE WITHOUT TESTS: Any new capability or bug fix without accompanying unit tests is grounds for immediate rejection."

You added significant new capabilities but provided **zero** unit tests for them. The following files have no test coverage for the newly added logic:
*   `cmd/graphdb/cmd_enrich_history.go` (No tests for `analyzeGitHistory`)
*   `internal/query/neo4j_history.go` (No tests for `GetHotspots` or `UpdateFileHistory`)
*   `internal/storage/neo4j_emitter.go` (No tests for the `Neo4jEmitter` implementation)
*   Incremental logic in `cmd/graphdb/cmd_ingest.go`

Adding a mock stub to `internal/rpg/orchestrator_test.go` does not satisfy the requirement for unit testing the actual implementations.

### 2. Missing Implementation Requirement (Plan Violation)
According to the implementation plan `plans/09_CAMPAIGNS_7-11_PLAN.md` (Item 4, Step 4a):
> 4. Calculate and set `risk_score` on Function nodes using fan-in + change_frequency

This requirement was completely ignored. 
*   `cmd_enrich_history.go` completes without invoking any risk score calculation.
*   The existing `CalculateRiskScores` function in `internal/query/neo4j_contamination.go` was not updated to incorporate `change_frequency`.
*   While the new `GetHotspots` query dynamically multiplies the risk score and change frequency for sorting, the plan explicitly requires you to *calculate and set* the `risk_score` property on the Function nodes themselves.

## Required Actions for the Engineer
1. **Write Unit Tests:** Implement comprehensive unit tests for `analyzeGitHistory`, `GetHotspots`, `UpdateFileHistory`, and `Neo4jEmitter`. Ensure the tests explicitly cover the new incremental ingestion and history enrichment logic.
2. **Implement Missing Logic:** Update the graph to explicitly calculate and set `risk_score` on `Function` nodes using a combination of fan-in and `change_frequency`, as mandated by the plan.
3. **Verify:** Run `go test ./...` to ensure all tests (including the new ones) pass before resubmitting.
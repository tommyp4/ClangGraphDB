# Feature Implementation Plan: Optimize Neo4j Edge Import

## 📋 Todo Checklist
- [ ] Update `internal/loader/neo4j_loader_test.go` to assert new query structure.
- [ ] Modify `internal/loader/neo4j_loader.go` to implement `CodeElement` label strategy.
- [ ] Verify tests pass.

## 🔍 Analysis & Investigation
The current edge import process in `Neo4jLoader` is extremely slow due to inefficient Cypher queries.
1.  **Bottleneck:** The edge loading query uses `MATCH (source {id: ...})` without specifying a node label. This forces Neo4j to scan all nodes (or check all indexes) for every single edge in the batch, resulting in O(N*M) complexity where N is total nodes and M is batch size.
2.  **Constraint:** Existing nodes have disparate labels (`File`, `Function`, etc.) and no common label or generic index on `id`.
3.  **Solution:** 
    *   Introduce a common label `:CodeElement` for all graph nodes (excluding metadata like `GraphState`).
    *   Create a unique constraint on `(n:CodeElement) REQUIRE n.id IS UNIQUE`.
    *   Update edge loading to `MATCH (source:CodeElement {id: ...})`, enabling direct index lookups (O(1) per edge).

## 📝 Implementation Plan

### Prerequisites
*   None. Operates on standard Neo4j driver.

### Step-by-Step Implementation

#### Phase 1: Test Harness Updates
1.  **Step 1.A (The Harness):** Update tests to enforce the new query structure.
    *   *Action:* Modify `internal/loader/neo4j_loader_test.go`.
    *   *Detail:*
        *   Update `TestBuildNodeQuery` to assert the presence of `SET n:CodeElement`.
        *   Update `TestBuildEdgeQuery` to assert the presence of `MATCH (source:CodeElement ...)` and `MATCH (target:CodeElement ...)`.
    *   *Goal:* Ensure the generated Cypher queries match the optimization strategy.

#### Phase 2: Core Optimization
2.  **Step 2.A (Apply Constraints):** Add the generic constraint.
    *   *Action:* Modify `internal/loader/neo4j_loader.go` in `ApplyConstraints`.
    *   *Detail:* Add `CREATE CONSTRAINT IF NOT EXISTS FOR (n:CodeElement) REQUIRE n.id IS UNIQUE` to the constraints list.
3.  **Step 2.B (Node Loading):** Tag nodes with the generic label.
    *   *Action:* Modify `internal/loader/neo4j_loader.go` in `buildNodeQuery`.
    *   *Detail:* Append `SET n:CodeElement` to the Cypher query.
4.  **Step 2.C (Edge Loading):** Use the generic label for lookups.
    *   *Action:* Modify `internal/loader/neo4j_loader.go` in `buildEdgeQuery`.
    *   *Detail:* Change `MATCH (source {id: row.sourceId})` to `MATCH (source:CodeElement {id: row.sourceId})`. Same for target.

#### Phase 3: Verification
5.  **Step 3.A (Verify):** Run the tests.
    *   *Action:* Run `go test ./internal/loader/...`.
    *   *Success:* Tests pass, confirming query structure is correct.

### Testing Strategy
*   **Unit Tests:** Verify the Cypher string generation.
*   **Performance (Manual):** The user should observe significantly faster import times (O(N) vs O(N^2)) when running `graphdb import`.

## 🎯 Success Criteria
*   `TestBuildNodeQuery` and `TestBuildEdgeQuery` pass with the new assertions.
*   The generated Cypher queries explicitly use `:CodeElement` for node lookups.

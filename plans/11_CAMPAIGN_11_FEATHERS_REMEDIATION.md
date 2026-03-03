# Campaign 11.5: Feathers Remediation (Volatility & Pinch Points)

## Goal
Deprecate the rigid UI/DB/IO layer contamination logic introduced in Campaign 8 and replace it with a more robust Volatility detection system based on Michael Feathers' definition of true legacy seams. This includes implementing Volatility Gradients, Upward Propagation, Pinch Point Detection, and Semantic Seam identification.

## Status
Phase 1 & 2 Completed ✅

## Pre-requisites
- Neo4j database connection
- Existing graph data

## Implementation Plan

### Phase 1: Deprecate Legacy Layer Logic ✅
- **Objective:** Remove the hardcoded `ui_contaminated`, `db_contaminated`, and `io_contaminated` fields.
- **Tasks:**
  - [x] Update `internal/query/neo4j_contamination.go` to remove old seeding and propagation logic for these specific layers. ✅
  - [x] Update `cmd/graphdb/cmd_enrich_contamination.go` to remove default heuristic rules for UI, DB, and IO layers. ✅
  - [x] Update the `CalculateRiskScores` logic to depend on the new volatility metrics rather than layer counts. ✅
  - [x] Create a migration/cypher script to drop these properties from existing nodes if necessary. ✅ (Implemented as cleanup in `SeedVolatility`)

### Phase 2: Implement Volatility and Upward Propagation ✅
- **Objective:** Introduce the `is_volatile` flag and ensure dependencies point correctly.
- **Tasks:**
  - [x] Define new default heuristic rules for volatility (e.g., matching external namespaces like `System.Net`, 3rd-party libraries, unmanaged code, non-deterministic functions like `DateTime.Now`). ✅
  - [x] Implement `SeedVolatility` to set `is_volatile = true` on nodes matching the heuristics. ✅
  - [x] Rewrite `PropagateContamination` as `PropagateVolatility`. **Crucially, change the propagation direction to UPWARD** (from Callee to Caller): `MATCH (caller)-[:CALLS]->(callee {is_volatile: true}) SET caller.is_volatile = true`. ✅
  - [x] Calculate a `volatility_score` based on distance to volatile boundaries and degree of contamination. ✅

### Phase 3: Rewrite Seams to Detect Pinch Points ✅
- **Objective:** Replace the broken "where contamination stops" seam query with a Pinch Point query. ✅
- **Tasks:**
  - [x] Update `GetSeams` in `internal/query/neo4j.go`. ✅
  - [x] Implement Cypher query to identify **Pinch Points**: nodes with high internal Fan-In (called by many non-volatile internal functions) and high volatile Fan-Out (orchestrates multiple volatile/external dependencies). ✅
  - [x] Update `cmd/graphdb/cmd_query.go` to support `-type seams` using the new Pinch Point definition. ✅

### Phase 4: Implement Semantic Seams (SRP Violations)
- **Objective:** Utilize Vector Embeddings to find structural cohesion vs semantic divergence. By analyzing the semantic similarity of functions/methods within the same class or file, we can detect Single Responsibility Principle (SRP) violations, which serve as excellent "conceptual seams" for refactoring.

#### Task 4.1: Extend Query Interface and Mock for Semantic Seams ✅ (REMEDIATED)
1.  **Step 4.1.A (The Harness):** Define the verification requirement in `internal/query/neo4j_semantic_seams_test.go`. ✅
    *   *Action:* Create a test file asserting that a mock or a real database can execute and return results for semantic seam detection.
    *   *Goal:* Ensure the new method signature `GetSemanticSeams(ctx context.Context, similarityThreshold float64) ([]*SemanticSeamResult, error)` is testable and properly asserted.
2.  **Step 4.1.B (The Implementation):** Add the interface and structs. ✅
    *   *Action:* Modify `internal/query/interface.go` to add `SemanticSeamResult` struct containing `Container string` (File/Class name), `MethodA string`, `MethodB string`, and `Similarity float64`.
    *   *Action:* Add `GetSemanticSeams(ctx context.Context, similarityThreshold float64) ([]*SemanticSeamResult, error)` to the `GraphProvider` interface.
3.  **Step 4.1.C (The Verification):** Verify interfaces. ✅
    *   *Action:* Run `go test ./internal/query/...`. Update `cmd/graphdb/mocks.go` to implement `GetSemanticSeams` to ensure the build succeeds and assertions pass.

#### Task 4.2: Implement Cypher Logic for Vector Similarity (Intra-Class) ✅ (REMEDIATED)
1.  **Step 4.2.A (The Harness):** Add an integration test in `internal/query/neo4j_semantic_seams_test.go`. ✅
    *   *Action:* Create a mock File node with several Function child nodes. Assign known embedding properties to the functions (some mathematically close, some divergent).
2.  **Step 4.2.B (The Implementation):** Implement the query in `internal/query/neo4j_semantic_seams.go`. ✅
    *   *Action:* Write the Cypher logic for `GetSemanticSeams`.
    *   *Detail:* The query should MATCH a container (File/Class) and its child Functions. It should pair the functions `(f1)` and `(f2)` where `f1.id < f2.id`, calculate `vector.similarity.cosine(f1.embedding, f2.embedding)`, and filter pairs where the similarity is below the `threshold`.
    *   *Action:* Return the aggregated pairs as `SemanticSeamResult`.
3.  **Step 4.2.C (The Verification):** Execute tests. ✅
    *   *Action:* Run the new integration test against the local Neo4j instance to ensure the Cypher query executes and correctly identifies the divergent methods.

#### Task 4.3: Expose Semantic Seams in the CLI
1.  **Step 4.3.A (The Harness):** Create/update CLI tests.
    *   *Action:* Update testing for CLI flags in `cmd/graphdb/cmd_query_test.go` (if it exists) to ensure `-type semantic-seams` parses correctly with its respective thresholds.
2.  **Step 4.3.B (The Implementation):** Modify `cmd/graphdb/cmd_query.go`.
    *   *Action:* Add `"semantic-seams"` to the `switch *typePtr` block.
    *   *Action:* Add a new flag `-similarity` (default `0.5`) to control the cosine similarity threshold (lower means more divergent).
    *   *Action:* Call `provider.GetSemanticSeams(*similarityPtr)` and format the JSON output to clearly show the SRP violations (e.g., displaying the File, the two methods, and their low similarity score).
3.  **Step 4.3.C (The Verification):** End-to-end test.
    *   *Action:* Run `go build ./cmd/graphdb` and execute `./graphdb query -type semantic-seams -similarity 0.5`. Verify the output format is readable, accurate, and actionable.

### Testing Strategy
- **Unit Tests:** Verify the mock interface generation and correct definition of `SemanticSeamResult`.
- **Integration Tests:** Use specifically crafted sub-graphs within Neo4j with deterministic embedding vectors (e.g., `[1.0, 0.0, 0.0]` vs `[0.0, 1.0, 0.0]`) to prove cosine similarity logic detects distinct semantic clusters accurately.
- **E2E Tests:** Ensure the newly built binary executes the semantic seams query and formats JSON correctly.

## Success Criteria
- The `enrich-contamination` CLI command correctly seeds and propagates volatility upwards.
- The `seams` query returns valid Pinch Points (hourglass nodes) instead of returning 0.
- The `semantic-seams` query accurately identifies conceptually mismatched functions within a single file based on their vector embeddings, outputting actionable refactoring recommendations.
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

### Phase 3: Rewrite Seams to Detect Pinch Points
- **Objective:** Replace the broken "where contamination stops" seam query with a Pinch Point query.
- **Tasks:**
  - Update `GetSeams` in `internal/query/neo4j.go`.
  - Implement Cypher query to identify **Pinch Points**: nodes with high internal Fan-In (called by many non-volatile internal functions) and high volatile Fan-Out (orchestrates multiple volatile/external dependencies).
  - Update `cmd/graphdb/cmd_query.go` to support `-type seams` using the new Pinch Point definition.

### Phase 4: Implement Semantic Seams (SRP Violations)
- **Objective:** Utilize Vector Embeddings to find structural cohesion vs semantic divergence.
- **Tasks:**
  - Introduce a new query type `-type semantic-seams`.
  - Implement Cypher/Go logic to analyze siblings (e.g., functions within the same class/file).
  - Use vector similarity to flag sibling nodes that have highly divergent embeddings, indicating a Single Responsibility Principle violation and a potential "conceptual" seam.

## Success Criteria
- The `enrich-contamination` CLI command correctly seeds and propagates volatility upwards.
- The `seams` query returns valid Pinch Points (hourglass nodes) instead of returning 0.
- The `semantic-seams` query accurately identifies conceptually mismatched functions within a single file.
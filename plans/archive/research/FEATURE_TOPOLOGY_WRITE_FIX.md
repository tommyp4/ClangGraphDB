# Fix Plan: Prevent Pipeline Hang During Feature Topology Write

## References
- **RCA:** `plans/research/GraphDB_RCA.md`
- **Branch:** `vertex-timeout-error`
- **Incident Date:** 2026-03-11

---

## 1. Root Cause (Detailed)

The pipeline hung for 14 hours at the log line:
```
Writing 44360 feature nodes and 272143 edges to database...
```

The direct cause is `UpdateFeatureTopology` in `internal/query/neo4j_batch.go:239-310`. This method executes **all 316,503 mutations (44,360 nodes + 272,143 edges) inside a single Neo4j managed transaction**:

```go
_, err := session.ExecuteWrite(p.ctx, func(tx neo4j.ManagedTransaction) (any, error) {
    // 1. UNWIND all 44,360 nodes in one query
    // 2. UNWIND all 272,143 edges (grouped by type) in the SAME transaction
    return nil, nil
})
```

Three compounding factors made this fatal:

### 1.1 Monolithic Transaction
Neo4j must hold all 316,503 mutations in server-side heap memory before committing. This causes massive lock contention, GC pressure, and transaction log growth that can exhaust disk I/O or trigger server-side OOM.

### 1.2 No Timeouts
The driver is created with zero timeout configuration (`internal/query/neo4j.go:25`):
```go
driver, err := neo4j.NewDriverWithContext(cfg.Neo4jURI, auth)
```
No `MaxTransactionRetryTime`, no `SocketKeepalive`, no `ConnectionLivenessCheckTimeout`. The context is `context.Background()` which never expires. The Go TCP socket waits indefinitely for a server response that may never come (TCP half-open connection / silent timeout).

### 1.3 Expensive Edge Queries (No Label Constraints)
The edge MERGE queries use unbounded node matching:
```cypher
MATCH (source {id: row.sourceId})
MATCH (target {id: row.targetId})
MERGE (source)-[r:IMPLEMENTS]->(target)
```
No label constraint (like `:CodeElement` or `:Feature`), so each of the 272,143 edge operations is an O(N) scan against the entire graph instead of an O(1) index lookup.

---

## 2. Why Phase 2 (Import) Works but Phase 3 (Enrichment) Doesn't

The Import phase handles the same scale correctly. Comparison:

| Aspect | Import (Phase 2) | Enrichment (Phase 3) |
|--------|-----------------|---------------------|
| **Batch size** | Configurable (`-batch-size 500`) | None - all at once |
| **Transaction scope** | Per-batch (500 items) | Single (316K items) |
| **Label constraints** | `MERGE (n:Function {id: ...})` | `MERGE (n {id: ...})` - no label |
| **Error isolation** | One batch fails, others succeed | One failure = total rollback |
| **Timeout** | Implicit (small txns finish fast) | None (hangs forever) |
| **Implementation** | `Neo4jEmitter` + `Neo4jLoader` | Direct in `UpdateFeatureTopology` |

The `Neo4jEmitter` (`internal/storage/neo4j_emitter.go`) already implements the correct pattern: accumulate items and flush at a configurable `batchSize`. But `UpdateFeatureTopology` doesn't use it.

---

## 3. Fix Strategy

Four changes, ordered by impact:

### 3.1 Fix Edge Queries (Performance)
**Files:** `internal/query/neo4j_batch.go`

Add label constraints to MATCH clauses. Feature/Domain nodes need the `:CodeElement` label so the existing uniqueness index is used.

**Before:**
```cypher
MATCH (source {id: row.sourceId})
MATCH (target {id: row.targetId})
```

**After:**
```cypher
MATCH (source:CodeElement {id: row.sourceId})
MATCH (target:CodeElement {id: row.targetId})
```

Also update the node creation query to apply `:CodeElement`:
```cypher
MERGE (n:CodeElement {id: row.id})
SET n += row
```

This turns each edge operation from O(N) full scan to O(1) index lookup.

### 3.2 Chunked Batching (Core Fix)
**Files:** `internal/query/neo4j_batch.go`

Break `UpdateFeatureTopology` into batches of ~500 items, each in its own transaction:

1. Split nodes into chunks of N (default 500)
2. For each chunk: open session, UNWIND chunk, commit, close session
3. Split edges by type, then sub-chunk each type into groups of N
4. For each edge chunk: open session, UNWIND chunk, commit, close session
5. Log progress per batch

**Why non-atomic is acceptable here:**
- MERGE is idempotent - re-running is safe (nodes use unique UUIDs)
- Partial progress is better than 14-hour hangs
- The summarization step (Phase 4) already handles partially-written features

### 3.3 Driver Timeouts & Keepalive (Defense in Depth)
**Files:** `internal/query/neo4j.go`

Add timeout configuration to `NewNeo4jProvider`:

```go
driver, err := neo4j.NewDriverWithContext(cfg.Neo4jURI, auth, func(c *neo4j.Config) {
    c.MaxTransactionRetryTime = 30 * time.Second
    c.SocketKeepalive = true
    c.ConnectionLivenessCheckTimeout = 1 * time.Minute
})
```

Add per-operation context deadlines to `UpdateFeatureTopology` (and potentially other write methods):
```go
ctx, cancel := context.WithTimeout(p.ctx, 5*time.Minute)
defer cancel()
```

### 3.4 Progress Logging
**Files:** `internal/query/neo4j_batch.go`, `internal/rpg/orchestrator.go`

Replace the single "Writing X nodes and Y edges..." log with per-batch progress:
```
Writing feature topology: nodes batch 1/89 (500/44360)...
Writing feature topology: nodes batch 2/89 (1000/44360)...
...
Writing feature topology: edges [IMPLEMENTS] batch 1/45 (500/22500)...
```

---

## 4. Step-by-Step Implementation Plan

### Step 1: Add Label Constraints to Feature Topology Queries
**File:** `internal/query/neo4j_batch.go`
- In `UpdateFeatureTopology`, change the node MERGE from `MERGE (n {id: row.id})` to `MERGE (n:CodeElement {id: row.id})`
- Keep the existing `SET n:Domain` / `SET n:Feature` FOREACH logic (these add additional labels)
- Change the edge MATCH from `MATCH (source {id: ...})` to `MATCH (source:CodeElement {id: ...})`
- Same for target: `MATCH (target:CodeElement {id: ...})`

### Step 2: Break UpdateFeatureTopology into Batched Writes
**File:** `internal/query/neo4j_batch.go`
- Extract a helper: `func (p *Neo4jProvider) batchWriteNodes(ctx context.Context, nodes []*graph.Node, batchSize int) error`
  - Chunks the node slice into groups of `batchSize`
  - Each chunk gets its own `session.ExecuteWrite` call
  - Logs progress after each batch
- Extract a helper: `func (p *Neo4jProvider) batchWriteEdges(ctx context.Context, edges []*graph.Edge, batchSize int) error`
  - Groups edges by type (existing logic)
  - Sub-chunks each type group into groups of `batchSize`
  - Each sub-chunk gets its own `session.ExecuteWrite` call
  - Logs progress after each batch
- Rewrite `UpdateFeatureTopology` to call these two helpers sequentially
- Use a default batch size of 500 (matching the import phase default)

### Step 3: Add Timeouts to Neo4j Driver
**File:** `internal/query/neo4j.go`
- In `NewNeo4jProvider`, add driver configuration:
  - `MaxTransactionRetryTime`: 30s
  - `SocketKeepalive`: true
  - `ConnectionLivenessCheckTimeout`: 1 minute
- In `UpdateFeatureTopology` (and the new batch helpers), wrap each batch operation with a per-batch context timeout of 5 minutes

### Step 4: Add Progress Logging to Orchestrator
**File:** `internal/rpg/orchestrator.go`
- In `RunClustering`, update the log line at line 205 to indicate batched write will follow
- The per-batch logging is handled internally by the helpers from Step 2

### Step 5: Verify with Tests
**File:** `internal/query/neo4j_batch_test.go`
- Existing tests use small data sets and should pass unchanged (batching with N < batchSize = 1 batch = same behavior)
- Add a test that verifies large input (e.g., 1500 nodes) is split into multiple batches by mocking the driver and counting `ExecuteWrite` calls
- Add a test that verifies `:CodeElement` label is present in generated Cypher

---

## 5. Risk Assessment

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Partial writes on failure mid-batch | Medium | MERGE is idempotent; re-running is safe. Document that `enrich-features` can be re-run. |
| Feature nodes missing `:CodeElement` label (breaks edge queries) | Low | Step 1 adds `:CodeElement` to the MERGE; existing Feature nodes from prior runs will match by `id` property on MERGE and get the label via SET. |
| Batch size too small (slow) or too large (still hangs) | Low | Default 500 matches proven import phase. Can expose as CLI flag later if needed. |
| Existing tests break | Very Low | Tests use small data; single-batch behavior is identical. |
| Driver timeout too aggressive (kills valid slow batches) | Low | 5-minute per-batch timeout is generous for 500 items. Adjust if needed. |

---

## 6. Verification Criteria

- [ ] `go test ./internal/query/...` passes
- [ ] `go test ./internal/rpg/...` passes
- [ ] `go test ./...` passes
- [ ] Manual test: run `enrich-features` on a medium codebase and observe per-batch log output
- [ ] Manual test: confirm Feature/Domain nodes have `:CodeElement` label in Neo4j browser
- [ ] Manual test: confirm edge MATCH uses index (check with `EXPLAIN` / `PROFILE` in Neo4j browser)

# Data Recovery Strategy

Run this after all code changes from `fix_domain_and_contamination_architecture.md` are deployed and verified.

## Goal
Rebuild enrichment data with minimal cost by clearing only what's necessary and preserving everything that's still valid.

## What to clear

One Cypher query removes the enrichment properties that must be regenerated:

```cypher
MATCH (f:Function)
REMOVE f.atomic_features, f.is_volatile, f.volatility_score, f.risk_score
```

| Property | Why clear it |
|---|---|
| `atomic_features` | Clearing triggers `GetUnextractedFunctions` to re-queue all functions for the new extraction that returns `is_volatile` |
| `is_volatile` | Stale data from regex-based contamination rules; will be re-seeded by LLM |
| `volatility_score` | Derived from stale `is_volatile`; recalculated by `CalculateRiskScores` |
| `risk_score` | Derived from stale volatility; recalculated by `CalculateRiskScores` |

## What is automatically handled

No manual action needed for these — the new code handles them:

| Data | Mechanism |
|---|---|
| Feature/Domain nodes | `ClearFeatureTopology()` runs `DETACH DELETE` at the start of clustering |
| IMPLEMENTS/PARENT_OF edges | Deleted with Feature/Domain nodes, recreated during clustering |
| `summary` property mismatch | Old Feature/Domain nodes are deleted; new ones are created with `description` |

## What to keep

| Data | Why it's safe |
|---|---|
| Function/File/Class nodes | Structural ingestion data, unchanged |
| CALLS edges | Structural, unchanged |
| DEFINED_IN edges | Structural, unchanged |
| `embedding` vectors on Functions | Derived from `atomic_features` via `NodeToText`. Same LLM + same code = same descriptors. Embeddings remain valid. |
| Git history (`change_frequency`, `last_changed`, `co_changes` on File nodes) | Unrelated to enrichment pipeline |
| Test data (`is_test`, TESTS edges) | Unrelated to enrichment pipeline |
| GraphState | Commit tracking for incremental ingestion, unrelated |

## Recovery sequence

```bash
# 1. Re-run Extraction (all functions re-queued, gets descriptors + is_volatile)
graphdb enrich --step extract

# 2. Re-run Embeddings (NO-OP: embedding IS NOT NULL on all functions)
graphdb enrich --step embed

# 3. Re-run Clustering (auto-clears old topology, rebuilds Feature/Domain nodes)
graphdb enrich --step cluster

# 4. Generate Summaries (names/describes new Feature and Domain nodes)
graphdb enrich --step summarize

# 5. Propagate Volatility + Calculate Risk Scores
graphdb enrich-contamination
```

## Cost analysis

| Step | Work | Cost |
|---|---|---|
| Extract | 227k LLM calls | Unavoidable — source of `is_volatile` |
| Embed | No-op | Saved — embeddings preserved |
| Cluster | K-means + LLM naming | Unavoidable — topology is rebuilt |
| Summarize | LLM calls per Feature/Domain | Unavoidable — new nodes need names |
| Contamination | Cypher-only propagation + scoring | Cheap — seconds |

## Risk

If the new combined prompt (`{"descriptors": [...], "is_volatile": true}`) causes the LLM to generate slightly different descriptor strings than the old prompt (`["descriptor1", ...]`), the kept embeddings would be marginally stale. This is unlikely since the descriptor portion of the prompt is unchanged — the volatility question is additive. If clustering quality appears degraded after recovery, clear embeddings and re-run step 2:

```cypher
MATCH (f:Function) REMOVE f.embedding
```

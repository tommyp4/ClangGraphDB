# Plan: Domain K Sizing — File-Based Clustering with Cap

## Problem

The global clustering KStrategy uses `sqrt(n_functions / 10)` to determine the number of top-level domains (K). For a 33K-file / 227K-function project, this produces K=151, which causes:

1. **K-Means++ initialization**: 151 rounds where each round scales linearly with the number of existing centroids. By round 60, each step takes ~2m18s and accelerating. Total init time: estimated 2+ hours.
2. **K-Means iterations**: Each of ~50 iterations computes 227K x 151 x 768 cosine distances. Each iteration takes ~5-6 minutes. Total: 4-5 hours.
3. **LLM domain naming**: 151 Summarizer calls after clustering.
4. **Semantic quality**: 151 "domains" is too many — real projects have 10-50 meaningful architectural boundaries regardless of size.

## Root Cause

Function count is the wrong signal for domain K. Domains represent architectural boundaries ("Authentication", "Data Access", "API Routing") — concepts that don't scale linearly with code volume. A 200-file project and a 33K-file project might both have an "Authentication" domain; the bigger project just has more functions *within* it.

## Current Code

**File:** `internal/rpg/orchestrator.go:149-158`
```go
KStrategy: func(n int) int {
    if n == 0 {
        return 0
    }
    k := int(math.Sqrt(float64(n) / 10.0))
    if k < 2 {
        return 2
    }
    return k
},
```

The `KStrategy` receives `n` (number of functions) and returns K. It is set on the `innerGlobalClusterer` which is passed to `GlobalEmbeddingClusterer.Inner`.

The default KStrategy in `EmbeddingClusterer` (used for sub-clustering features within domains) uses `k = n/5`, which is fine — sub-clusters are small and fast.

**File:** `internal/rpg/cluster_semantic.go:69-83`
```go
var k int
if c.KStrategy != nil {
    k = c.KStrategy(len(nodes))
} else {
    k = len(nodes) / 5
}
if k < 2 {
    k = 2
}
if k > len(nodes)/2 {
    k = len(nodes) / 2
}
```

Note the existing guard: `k > len(nodes)/2` prevents degenerate cases but doesn't cap at a reasonable domain ceiling.

## Proposed Change

### 1. Change KStrategy to use unique file count with a hard cap

**File:** `internal/rpg/orchestrator.go`

Before constructing the `innerGlobalClusterer`, count unique files from the function metadata (already available as `functions []graph.Node` at that point in `RunClustering`):

```go
// Count unique files from function metadata
uniqueFiles := make(map[string]struct{})
for _, fn := range functions {
    if file, ok := fn.Properties["file"].(string); ok {
        uniqueFiles[file] = struct{}{}
    }
}
fileCount := len(uniqueFiles)
log.Printf("Detected %d unique files across %d functions", fileCount, len(functions))
```

Then change the KStrategy:

```go
KStrategy: func(n int) int {
    if n == 0 {
        return 0
    }
    k := int(math.Sqrt(float64(fileCount) / 5.0))
    if k < 5 {
        return 5
    }
    if k > 50 {
        return 50
    }
    return k
},
```

The closure captures `fileCount` from the surrounding scope.

### 2. Expected K values

| Project size | Files | Functions | K (current) | K (proposed) |
|-------------|-------|-----------|-------------|-------------|
| Small | 50 | 200 | 4 | 5 (floor) |
| Small-Med | 200 | 1,000 | 10 | 6 |
| Medium | 1,000 | 5,000 | 22 | 14 |
| Medium-Large | 2,000 | 15,000 | 38 | 20 |
| Large | 5,000 | 40,000 | 63 | 31 |
| Very Large | 10,000 | 100,000 | 100 | 44 |
| Your project | 33,000 | 227,933 | 151 | 50 (cap) |

### 3. Performance impact

For the 33K-file project (K=50 vs K=151):

| Operation | K=151 | K=50 | Speedup |
|-----------|-------|------|---------|
| K-Means++ init rounds | 151 | 50 | ~9x (fewer rounds, each round cheaper) |
| Per K-Means iteration | 227K x 151 distances | 227K x 50 distances | ~3x |
| Total K-Means (init + iters) | ~6-7 hours | ~30-45 minutes | ~10x |
| LLM domain naming calls | 151 | 50 | 3x |

### 4. Why the sub-clustering is unaffected

The default `EmbeddingClusterer.KStrategy` (`k = n/5`) is used for feature sub-clustering *within* each domain. With K=50 domains, each domain averages ~4500 functions, producing ~900 features per domain. These sub-cluster runs are fast because:
- N is small (~4500 per domain)
- K-Means++ init with k=900 on 4500 vectors is quick
- Each iteration: 4500 x 900 distances — negligible vs 227K x 151

### 5. Why 50 is a reasonable ceiling

- Domains represent human-comprehensible architectural concepts
- Real software projects (even massive ones) have 20-50 logical subsystems
- Beyond 50, "domains" become indistinguishable from features
- The D3 visualizer and semantic trace queries work better with fewer top-level nodes
- Sub-clustering provides the depth beneath each domain

### 6. Why file count over function count

- Files are a proxy for project "surface area" — a file with 20 functions is still one logical unit
- File count grows more slowly and predictably than function count
- File organization reflects (imperfect) human intent about code boundaries
- A 10-function file and a 100-function file likely belong to the same domain

### 7. Future consideration: directory-aware seeding

Not in scope for this change, but worth noting: the function metadata includes file paths, which contain directory structure. Directories are the strongest available signal of human organizational intent. A future enhancement could:
- Extract unique directories at depth 2-3 from the root
- Use directory centroids (average of member function embeddings) as K-Means initial seeds instead of K-Means++
- This would make clustering faster (skip K-Means++ entirely) and more semantically grounded
- Trade-off: ties domain boundaries more tightly to directory structure, which may not reflect actual semantic boundaries

---

## Files to Modify

| File | Change |
|------|--------|
| `internal/rpg/orchestrator.go` | Count unique files, change KStrategy formula |

One file, one formula change. No interface changes, no new dependencies.

## Verification

- [ ] `go test ./internal/rpg/...` passes
- [ ] `go test ./...` passes
- [ ] Manual test on small project: K >= 5
- [ ] Manual test on large project (33K files): K = 50 (capped)
- [ ] Log output shows `Detected X unique files across Y functions`
- [ ] Clustering completes within ~1 hour for the 227K-function project

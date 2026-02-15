# Feature Implementation Plan: Global Discovery Mode

## 📋 Todo Checklist
- [x] **Step 1:** Add `KStrategy` to `EmbeddingClusterer` in `internal/rpg/cluster_semantic.go` ✅ Implemented
- [x] **Step 2:** Ensure `Builder` supports `GlobalClusterer` in `internal/rpg/builder.go` ✅ Implemented (Was already present)
- [x] **Step 3:** Update `cmd/graphdb/main.go` to integrate `GlobalClusterer` ✅ Implemented
- [x] **Step 4:** Deprecate `DirectoryDomainDiscoverer` in `internal/rpg/discovery.go` ✅ Implemented
- [x] **Step 5:** Verify compilation ✅ Implemented

## 🔍 Context
Switching from directory-based domain discovery to "Global Discovery Mode" using semantic clustering (`EmbeddingClusterer`) on all functions. This allows discovering latent domains not strictly defined by folder structure.

## 📝 Implementation Details
- **EmbeddingClusterer:** Added `KStrategy` to support custom K calculation (e.g. `sqrt(N/10)`).
- **Builder:** Uses `GlobalClusterer` to cluster all functions first, then determines `ScopePath` via LCA.
- **Main:** Configures `GlobalClusterer` with `sqrt(N/10)` strategy.

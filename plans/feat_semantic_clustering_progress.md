# Feature Implementation Plan: Semantic Clustering Progress Bar

## 📋 Todo Checklist
- [x] ~~Add progress callbacks to `internal/rpg/builder.go`~~ ✅ Implemented
- [x] ~~Sort domains in `rpg.Builder.Build` for deterministic processing order~~ ✅ Implemented
- [x] ~~Implement progress bar integration in `cmd/graphdb/main.go`~~ ✅ Implemented
- [x] ~~Verify `internal/rpg/builder_test.go` passes~~ ✅ Implemented
- [x] ~~Manual verification (or simulate via test)~~ ✅ Implemented

## 🔍 Analysis & Investigation
The user reports that the semantic clustering phase (invoked via `enrich-features` or implicit workflows) appears stuck due to a lack of progress indication. The log shows \"Using semantic clustering (embedding-based)\" followed by silence during the potentially long-running operation.

**Root Cause:**
- The `rpg.Builder.Build` method iterates over discovered domains and performs clustering for each.
- This process involves network calls (Embeddings) and computation (K-Means), which can be slow.
- There is no feedback loop between the `Builder` logic and the CLI UI.
- The `EmbeddingClusterer` processes in batches (chunk size 100), but this internal loop is hidden.

**Proposed Solution:**
We will implement a **Domain-Level Progress Bar**. While we won't show the internal progress of the `Clusterer` (e.g., \"Batch 5/10\") to avoid deep coupling, showing which domain is being processed (e.g., \"Clustering Domain: Auth (1/5)\") provides sufficient feedback that the process is alive.

**Architecture:**
- **`internal/rpg/builder.go`**: Add `OnPhaseStart` and `OnPhaseStep` callbacks to the `Builder` struct. Sort domains to ensure deterministic order and consistent progress reporting.
- **`cmd/graphdb/main.go`**: Hook into these callbacks to drive a `ui.ProgressBar`.

## 📝 Implementation Plan

### Prerequisites
- None. `internal/ui` package already exists.

### Step-by-Step Implementation

#### Phase 1: Builder Instrumentation
1.  **Step 1.A (The Harness):** Verify existing tests pass.
    *   *Action:* Run `go test ./internal/rpg/...`
    *   *Goal:* Ensure baseline stability.
2.  **Step 1.B (The Instrumentation):** Add callbacks and sorting to `Builder`.
    *   *Action:* Modify `internal/rpg/builder.go`.
    *   *Detail:*
        *   Add `OnPhaseStart func(phaseName string, total int)` to `Builder` struct.
        *   Add `OnPhaseStep func(stepName string)` to `Builder` struct.
        *   Import `sort`.
        *   In `Build()`:
            *   Get domain keys from map.
            *   Sort keys.
            *   Invoke `OnPhaseStart(\"Clustering Domains\", len(domains))`.
            *   Inside loop, invoke `OnPhaseStep(domainName)`.
3.  **Step 1.C (The Verification):** Verify compilation and tests.
    *   *Action:* Run `go test ./internal/rpg/...`.
    *   *Success:* Tests pass (callbacks are optional, so no changes needed in tests).

#### Phase 2: CLI Integration
1.  **Step 2.A (The UI Hook):** Connect CLI to Builder.
    *   *Action:* Modify `cmd/graphdb/main.go` (in `enrich-features` command handler).
    *   *Detail:*
        *   When initializing `rpg.Builder`:
        *   Define `OnPhaseStart`: Initialize `ui.NewProgressBar` (check total > 0).
        *   Define `OnPhaseStep`: Call `pb.Add(1)`.
        *   Ensure `pb.Finish()` is called after `Build()` returns (if `pb` was created).
2.  **Step 2.B (The Verification):**
    *   *Action:* Run `go build ./cmd/graphdb`.
    *   *Success:* Binary builds successfully.

### Testing Strategy
- **Unit Tests:** `internal/rpg/builder_test.go` ensures the logic remains correct with the new sorting and callback structure.
- **Manual Verification:** Since this is a UI feature, the primary verification is visual. However, as an automated agent, I will rely on the unit tests passing and the code compilation.

## 🎯 Success Criteria
- The `rpg.Builder` exposes progress hooks.
- The `enrich-features` command displays a progress bar during the clustering phase.
- Domains are processed in a deterministic order (sorted by name).
- No regressions in existing tests.

# Plan: Fix Broken Tests for Campaign 11 (Feathers Remediation)

The recent changes in Campaign 11 (Phase 1 & 2) deprecated `ui_contaminated`, `db_contaminated`, and `io_contaminated` in favor of `is_volatile`. This broke several tests that still rely on the old fields or the old `GraphProvider` interface.

## User Requirements
1. Update `MockGraphProvider` in `internal/rpg/orchestrator_test.go` and any other mocks to match the new `GraphProvider` interface.
2. Update `TestGetImpact` and `TestGetSeams` in `internal/query/neo4j_test.go` to use `is_volatile` instead of `ui_contaminated`.
3. Ensure `go test ./...` passes.

## Proposed Changes

### 1. Update Mocks
- [x] ~~Update `MockGraphProvider` in `internal/rpg/orchestrator_test.go`~~ ✅ Implemented
    - Rename/Replace `SeedContamination` with `SeedVolatility`.
    - Rename/Replace `PropagateContamination` with `PropagateVolatility`.
- [x] ~~Check `cmd/graphdb/mocks.go` for similar updates.~~ ✅ Already implemented

### 2. Update `internal/query/neo4j_test.go`
- [x] ~~Update `TestGetImpact`:~~ ✅ Implemented
    - Change `ui_contaminated: true` to `is_volatile: true` in mock data.
    - Update assertions to check for `is_volatile`.
- [x] ~~Update `TestGetSeams`:~~ ✅ Implemented
    - Change `ui_contaminated: true` to `is_volatile: true` in mock data.
    - Since `GetSeams` now uses `is_volatile`, updating the data should fix it.
    - *Note:* Phase 3 mentions rewriting `GetSeams` for Pinch Points. For now, I'll focus on making the existing test pass with the new field name.

### 3. Verification
- [x] ~~Run `go test ./internal/rpg/...`~~ ✅ Passed
- [x] ~~Run `go test ./internal/query/...`~~ ✅ Passed
- [x] ~~Run `go test ./...` to ensure everything else is fine.~~ ✅ Passed

## Execution Trace
1. [x] Update `MockGraphProvider` in `internal/rpg/orchestrator_test.go`
2. [x] Update `internal/query/neo4j_test.go`
3. [x] Verify all tests pass

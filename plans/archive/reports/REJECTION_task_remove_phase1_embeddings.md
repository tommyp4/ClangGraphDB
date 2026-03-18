# REJECTION REPORT: remove_phase1_embeddings (Phase 1)

## Summary
The implementation for Phase 1 of removing embeddings from `internal/ingest` is fundamentally incomplete because it breaks the project build.

## Details
1. **Build Failure**: The project fails to compile (`make build-all`). 
   - `cmd/graphdb/cmd_ingest.go:128:52: too many arguments in call to ingest.NewWalker`
   - `have (int, embedding.Embedder, storage.Emitter)`
   - `want (int, storage.Emitter)`
2. **Analysis**: You updated the signature of `ingest.NewWalker` and `ingest.NewWorkerPool` in `internal/ingest/walker.go` and `worker.go` to remove the `embedder` parameter (which aligns with Phase 1 of the plan). However, you failed to update the caller in `cmd/graphdb/cmd_ingest.go`. 
3. **Requirement**: While updating `cmd_ingest.go` is listed under Phase 2 of the plan, you cannot leave the main branch in a broken state between phases. If you change a public package signature, you MUST update all callers in the same atomic commit to ensure the project continues to compile.

## Action Required
- Update `cmd/graphdb/cmd_ingest.go` to remove the `embedder` setup and pass the correct arguments to `ingest.NewWalker` (effectively completing Step 2.A of the plan).
- Ensure `make build-all` succeeds.
- Re-submit for auditing once the build is green.
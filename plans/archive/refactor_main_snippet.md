# Plan: Refactor `cmd/graphdb/main.go` to use `snippet` library

**Status:** Active
**Objective:** Replace local `loadFileSegment` with shared `snippet.SliceFile` to eliminate code duplication.

## Steps
- [x] Import `graphdb/internal/tools/snippet` in `cmd/graphdb/main.go`
- [x] Update `handleEnrichFeatures` to use `snippet.SliceFile`
- [x] Remove `loadFileSegment` function definition
- [x] Verify build

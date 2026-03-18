# Feature Implementation Plan: Fix Build-All Import Sequence

## 📋 Todo Checklist
- [x] Create `cmd/graphdb/build_all_test.go` to reproduce the issue (fail) and verify the fix (pass).
- [x] Modify `cmd/graphdb/main.go` to allow mocking of `handleImport`.
- [x] Modify `handleBuildAll` in `cmd/graphdb/main.go` to import both `graph.jsonl` and `rpg.jsonl`.
- [x] Run tests to verify the fix.

## 🔍 Analysis & Investigation
The current `build-all` command in `cmd/graphdb/main.go` executes three phases:
1.  **Ingest**: Generates `graph.jsonl` (Structural Graph).
2.  **Enrich**: Generates `rpg.jsonl` (Semantic Graph).
3.  **Import**: Currently only imports `rpg.jsonl`.

**Problem:** The structural graph (`graph.jsonl`) is generated but never imported into Neo4j. This results in a missing dependency graph (Files, Classes, Functions, and their relationships).

**Solution:**
We need to split the import phase into two steps:
1.  Import `graph.jsonl`. If `-clean` is requested, apply it here.
2.  Import `rpg.jsonl`. Do **not** clean, so it appends to the structural graph.

**Testability:**
`handleBuildAll` directly calls `handleImport`, which depends on a running Neo4j instance. To verify this logic without side effects or external dependencies, we will refactor `main.go` to use a function variable `importCmd` (pointing to `handleImport` by default). This allows us to inject a mock importer in a unit test within the `main` package.

## 📝 Implementation Plan

### Prerequisites
None.

### Step-by-Step Implementation

#### Phase 1: Verification Harness
1.  **Step 1.A (The Harness):** Create `cmd/graphdb/build_all_test.go`.
    *   *Action:* Create a test file in the `main` package.
    *   *Content:*
        *   Define a test `TestHandleBuildAll_ImportsBothGraphs`.
        *   Mock `ingestCmd` and `enrichCmd` (we'll need to make these variables too, or just mock `importCmd` and let others run if they are fast/mockable).
        *   Actually, `ingest` and `enrich` might be slow or require files. We should make `ingestCmd` and `enrichCmd` swappable function variables as well for full isolation.
        *   The test will assert that `importCmd` is called exactly twice:
            1.  With `graph.jsonl` and `clean=true` (if requested).
            2.  With `rpg.jsonl` and `clean=false`.
    *   *Goal:* This test will fail (or compile error) until we refactor `main.go`.

#### Phase 2: Refactoring & Implementation
1.  **Step 2.A (Refactor main.go):** Make commands swappable.
    *   *Action:* In `cmd/graphdb/main.go`, define package-level variables:
        ```go
        var (
            ingestCmd = handleIngest
            enrichCmd = handleEnrichFeatures
            importCmd = handleImport
        )
        ```
    *   *Action:* Update `main()` and `handleBuildAll` to use these variables instead of direct function calls.
2.  **Step 2.B (The Implementation):** Modify `handleBuildAll`.
    *   *Action:* Update `handleBuildAll` in `cmd/graphdb/main.go`.
    *   *Logic:*
        ```go
        // Phase 3.A: Import Structural Graph
        fmt.Println("
[Phase 3.A/3] Importing Structural Graph...")
        importArgs1 := []string{"-input", "graph.jsonl"}
        if *cleanPtr {
            importArgs1 = append(importArgs1, "-clean")
        }
        importCmd(importArgs1)

        // Phase 3.B: Import Semantic Graph
        fmt.Println("
[Phase 3.B/3] Importing Semantic Graph...")
        importArgs2 := []string{"-input", "rpg.jsonl"}
        // Note: No clean flag here
        importCmd(importArgs2)
        ```
3.  **Step 2.C (The Verification):** Verify the fix.
    *   *Action:* Run `go test ./cmd/graphdb/...`.
    *   *Success:* The new test passes, confirming the call sequence.

### Testing Strategy
*   **Unit Test:** `cmd/graphdb/build_all_test.go` will verify the orchestration logic.
*   **Manual Check:** Inspect logs to ensure "Importing Structural Graph" and "Importing Semantic Graph" messages appear.

## 🎯 Success Criteria
*   `build-all` invokes import for `graph.jsonl`.
*   `build-all` invokes import for `rpg.jsonl`.
*   `-clean` flag is respected for the first import but ignored for the second.
*   Tests pass.

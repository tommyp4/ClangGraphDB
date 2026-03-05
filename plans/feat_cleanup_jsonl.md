# Feature Implementation Plan: feat_cleanup_jsonl

## 📋 Todo Checklist
- [x] ~~Add a unit test to verify that `build-all` attempts to clean up intermediate JSONL files.~~ ✅ Implemented
- [x] ~~Update `cmd_build_all.go` to delete `nodes.jsonl` and `edges.jsonl` after the import phase.~~ ✅ Implemented
- [x] ~~Final Review and Testing.~~ ✅ Passed

## 🔍 Analysis & Investigation
The `build-all` command in `cmd/graphdb/cmd_build_all.go` orchestrates the graph generation and database import by calling a sequence of sub-commands. During the ingestion phase, it creates intermediate files to store the structural graph (specifically, `nodes.jsonl` and `edges.jsonl` by default). The subsequent import phase reads these files to populate Neo4j.

Currently, these files are left on the disk after `build-all` completes. As per the requirements, they should be deleted once the import phase is done.

Dependencies:
- The `os` package needs to be imported in `cmd/graphdb/cmd_build_all.go` to use `os.Remove`.
- The tests in `cmd/graphdb/build_all_test.go` will need to be updated to assert this cleanup behavior by creating dummy files before running the mocked command.

## 📝 Implementation Plan

### Prerequisites
None.

### Step-by-Step Implementation

#### Phase 1: The Harness
1.  **Step 1.A (The Harness):** Define the verification requirement.
    *   *Action:* Update `cmd/graphdb/build_all_test.go` to add a new test function `TestHandleBuildAll_CleansUpIntermediateFiles`.
    *   *Detail:*
        - Create dummy `test_nodes.jsonl` and `test_edges.jsonl` files at the start of the test. Ensure they are cleaned up using `defer os.Remove(...)` in case of panic/failure.
        - Mock the command handlers (`ingestCmd`, `enrichCmd`, `importCmd`, etc.) with empty functions to prevent actual execution, similar to existing tests.
        - Call `handleBuildAll([]string{"-nodes", "test_nodes.jsonl", "-edges", "test_edges.jsonl"})`.
        - Assert that both dummy files have been deleted using `os.Stat` (expecting `os.IsNotExist` to be true).
2.  **Step 1.B (The Verification):** Verify the harness fails initially.
    *   *Action:* Run `go test ./cmd/graphdb -run TestHandleBuildAll_CleansUpIntermediateFiles`.
    *   *Success:* The test should fail because the implementation is not yet deleting the files.

#### Phase 2: The Implementation
1.  **Step 2.A (The Implementation):** Execute the core change.
    *   *Action:* Modify `cmd/graphdb/cmd_build_all.go`.
    *   *Detail:*
        - Import the `"os"` package alongside `flag` and `fmt`.
        - Immediately after the `importCmd(importArgs1)` call (around line 30), add logic to delete the intermediate files:
          ```go
          // 2.5 Cleanup intermediate files
          fmt.Println("\nCleaning up intermediate JSONL files...")
          if err := os.Remove(*nodesPtr); err != nil && !os.IsNotExist(err) {
              fmt.Printf("Warning: failed to remove %s: %v\n", *nodesPtr, err)
          }
          if err := os.Remove(*edgesPtr); err != nil && !os.IsNotExist(err) {
              fmt.Printf("Warning: failed to remove %s: %v\n", *edgesPtr, err)
          }
          ```
2.  **Step 2.B (The Verification):** Verify the harness passes.
    *   *Action:* Run `go test ./cmd/graphdb -run TestHandleBuildAll_CleansUpIntermediateFiles`.
    *   *Success:* The test passes, proving the files are deleted.

### Testing Strategy
- The primary validation is the new unit test which creates real files on disk and asserts their removal without triggering the real, long-running database ingest/import logic.
- A full integration test can be done by building the binary (`make build`) and running `graphdb build-all -dir .` on a small directory, verifying no `.jsonl` files are left behind in the current directory.

## 🎯 Success Criteria
- Running `graphdb build-all` successfully completes the entire pipeline without errors.
- Intermediate `nodes.jsonl` and `edges.jsonl` (or custom names provided via flags) are no longer present in the working directory after the command finishes.
- The new unit test passes, enforcing this behavior for future regressions.

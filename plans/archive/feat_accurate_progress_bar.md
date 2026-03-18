# Feature Implementation Plan: Accurate Progress Bar

## 📋 Todo Checklist
- [x] Create reproduction test case for `Walker.Count` respecting `.gitignore`.
- [x] Refactor `internal/ingest/walker.go` to extract `Walk` logic.
- [x] Implement `Walker.Count` method.
- [x] Update `cmd/graphdb/main.go` to use `Walker.Count`.
- [x] Verify fix with tests and build.

## 🔍 Analysis & Investigation
The current implementation of the progress bar in `cmd/graphdb/main.go` counts files using a naive `filepath.WalkDir` that iterates over all files, including those in `.git` and those ignored by `.gitignore`. However, the actual processing (`walker.Run`) uses `filepath.WalkDir` with logic to respect `.gitignore` (via `monochromegane/go-gitignore`).

This discrepancy leads to the progress bar showing a much higher total than the actual number of files processed, causing the progress bar to complete prematurely or show incorrect percentage.

### Affected Files
1.  `cmd/graphdb/main.go`: Contains the naive counting logic.
2.  `internal/ingest/walker.go`: Contains the `Run` method with the correct walking/ignore logic.

### Solution Strategy
1.  **Refactor `walker.go`**: Extract the directory walking and `.gitignore` handling logic from `Run` into a reusable `Walk` method.
2.  **Implement `Count`**: Add a `Count` method to `Walker` (or as a standalone function) that uses `Walk` to count files, applying the same ignore rules.
3.  **Update `main.go`**: Replace the manual `filepath.WalkDir` loop with a call to `walker.Count`.

## 📝 Implementation Plan

### Prerequisites
- `graphdb` codebase.

### Step-by-Step Implementation

#### Phase 1: Test & Refactor `internal/ingest/walker.go`
1.  **Step 1.A (The Harness):** Create a test to verify `Count` respects `.gitignore`.
    *   *Action:* Create `internal/ingest/walker_test.go`.
    *   *Detail:* Setup a temp directory with `.gitignore` and some ignored files. Call `walker.Count` (which doesn't exist yet) and assert the returned count matches the expected number of *included* files.
    *   *Goal:* Define the expected behavior.
2.  **Step 1.B (The Implementation):** Refactor `walker.go`.
    *   *Action:* Modify `internal/ingest/walker.go`.
    *   *Detail:*
        *   Define `func (w *Walker) Walk(dirPath string, visitor func(path string, d fs.DirEntry) error) error`.
        *   Move the `.gitignore` loading and `filepath.WalkDir` logic from `Run` to `Walk`.
        *   Update `Run` to call `Walk` with a visitor that submits tasks to `w.WorkerPool`.
        *   Implement `func (w *Walker) Count(dirPath string) (int64, error)` that uses `Walk` to count files.
3.  **Step 1.C (The Verification):** Verify the test passes.
    *   *Action:* Run `go test ./internal/ingest/...`.
    *   *Success:* `TestWalker_Count` passes.

#### Phase 2: Integrate into CLI
1.  **Step 2.A (The Implementation):** Update `main.go`.
    *   *Action:* Modify `cmd/graphdb/main.go`.
    *   *Detail:* In `handleIngest`, replace the `filepath.WalkDir` block used for counting with `count, err := walker.Count(*dirPtr)`.
2.  **Step 2.B (The Verification):** build and manual check.
    *   *Action:* Run `go build -o graphdb cmd/graphdb/main.go`.
    *   *Success:* Build succeeds.

### Testing Strategy
-   **Unit Tests:** `internal/ingest/walker_test.go` will cover the core logic of `Count` and `Walk` respecting `.gitignore`.
-   **Integration:** The existing `Run` logic is preserved (refactored but logically equivalent), so existing ingestion tests should still pass.

## 🎯 Success Criteria
-   `Walker.Count` accurately returns the number of files that `Walker.Run` would process.
-   `graphdb ingest` progress bar initializes with the correct total.
-   Code duplication between `Run` and counting logic is eliminated.

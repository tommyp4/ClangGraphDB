# Feature Implementation Plan: Ingest Exclusions (.gitignore Support)

## 📋 Todo Checklist
- [x] Phase 1: Verification Harness (Reproduction)
- [x] Phase 2: Core Logic Implementation
- [x] Phase 3: CLI Integration & Validation
- [x] Final Review and Testing

## 🔍 Analysis & Investigation
The current `ingest` command in `cmd/graphdb/main.go` uses `internal/ingest/walker.go` to traverse directories. The `Walker` struct uses `filepath.WalkDir` without any filtering logic. This causes the following issues:
1.  **Noise**: `node_modules`, `.git`, `dist`, `build`, and other irrelevant directories are ingested, bloating the graph with third-party code.
2.  **Performance**: Walking deep dependency trees (like `node_modules`) is slow and resource-intensive.
3.  **User Expectation**: Users expect standard tools to respect `.gitignore` files.

### Existing Implementation
-   `internal/ingest/walker.go`: Contains `Walker.Run` which calls `filepath.WalkDir`.
-   No `SkipDir` logic is implemented.
-   No dependency for `.gitignore` parsing exists.

### Requirements
1.  **Default Exclusions**: Always ignore `.git`.
2.  **Gitignore Support**: Respect `.gitignore` files in the directory tree.
3.  **Efficiency**: Use `filepath.SkipDir` to avoid entering ignored directories.
4.  **CLI Support**: (Optional but good) Allow explicit ignore patterns via flags? *Decision: Keep it simple for now, rely on .gitignore and defaults.*

## 📝 Implementation Plan

### Prerequisites
-   Add `github.com/monochromegane/go-gitignore` dependency.

### Step-by-Step Implementation

#### Phase 1: Verification Harness
1.  **Step 1.A (The Harness):** Create a reproduction test case.
    *   *Action:* Create `test/e2e/ingest_ignore_test.go`.
    *   *Content:*
        -   Create a temp directory structure:
            -   `src/main.ts`
            -   `node_modules/lib/index.ts`
            -   `.gitignore` (containing `node_modules/`)
        -   Run `ingest.Walker` on this directory.
        -   *Assert:* Both `main.ts` and `lib/index.ts` ARE emitted (Current Behavior / Failure Case for Feature).
        -   *Goal:* Confirm current behavior creates noise.

#### Phase 2: Core Logic Implementation
1.  **Step 2.A (Dependency):** Add the gitignore library.
    *   *Action:* Run `go get github.com/monochromegane/go-gitignore`.
2.  **Step 2.B (The Walker Update):** Modify `internal/ingest/walker.go`.
    *   *Action:* Update `Walker` struct to hold ignore logic.
    *   *Logic:*
        -   Initialize a `gitignore.GitIgnore` instance (or similar interface) when entering directories.
        -   In `WalkDir` callback:
            -   Check if path matches default ignores (`.git`).
            -   Check if path matches `.gitignore` patterns.
            -   If ignored and is directory -> return `filepath.SkipDir`.
            -   If ignored and is file -> return `nil` (skip).
    *   *Detail:*
        ```go
        // Simplified logic
        ignoreMatcher := gitignore.NewGitIgnoreFromReader(baseDir, gitignoreFile)
        if ignoreMatcher.Match(path, isDir) {
            if isDir { return filepath.SkipDir }
            return nil
        }
        ```
        *Note:* Handling nested `.gitignore` files correctly with `filepath.WalkDir` can be tricky. A simpler approach might be to just support the root `.gitignore` for now, or use a library that handles walking with ignores if available. `monochromegane/go-gitignore` provides matching but not walking. We will implement root-level `.gitignore` support first as it covers 90% of cases (`node_modules` is usually at root).

3.  **Step 2.C (The Refined Verification):** Update the harness.
    *   *Action:* Update `test/e2e/ingest_ignore_test.go`.
    *   *Success:* `lib/index.ts` is NOT emitted, `main.ts` IS emitted.

#### Phase 3: Integration
1.  **Step 3.A (CLI Wiring):** Update `cmd/graphdb/main.go`.
    *   *Action:* Ensure the `Walker` is initialized with the root directory context to find `.gitignore`.
    *   *Detail:* The `Walker` already takes `dirPath`. We might need to pass an explicit ignore list if we want to add flags later.

### Testing Strategy
-   **Unit Tests:** Verify `Walker` respects a mock `.gitignore`.
-   **E2E Tests:** Verify `ingest` command on a real file structure with `node_modules`.

## 🎯 Success Criteria
-   `ingest` command does NOT process files in `node_modules/` when a `.gitignore` exists (or by default).
-   `ingest` command does NOT process `.git/` directory.
-   Performance improves on repositories with large dependency trees.

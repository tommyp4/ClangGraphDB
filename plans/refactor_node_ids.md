# Feature Implementation Plan: Refactor Node IDs (Universal FQN & Relative Paths)

## 📋 Todo Checklist
- [x] **Phase 1: Project-Relative Paths** (System-wide infrastructure update - DONE)
- [x] **Phase 2: C# FQN Refactoring** (Language-specific ID fix - DONE)
- [x] **Phase 3: TypeScript Refactoring** (Use relative path as module ID) ✅ Implemented
- [x] **Phase 4: C++ Refactoring** (Namespace::Class ID) ✅ Implemented
- [x] **Phase 5: VB.NET Refactoring** (Namespace.Class ID) ✅ Implemented
- [x] **Phase 6: SQL Refactoring** (Schema.Table ID) ✅ Implemented
- [x] **Phase 7: Final Verification** ✅ Implemented

## 🔍 Analysis & Investigation

### Current State
1.  **Infrastructure:** `internal/ingest/worker.go` now correctly passes project-relative paths to parsers. This is the foundation.
2.  **C#:** Already refactored to use `Namespace.Class`.
3.  **Java:** Already uses `Package.Class` (mostly correct). No changes planned unless issues found.
4.  **TypeScript:** Uses `filePath:Symbol`. Since `filePath` is now relative, this acts as a valid module ID (e.g., `src/utils/math.ts:add`).
    *   *Action:* Ensure path separators are normalized (forward slashes) for cross-platform consistency.
5.  **C++:** Uses `filePath:Namespace::Class`.
    *   *Issue:* Brittle and prevents merging of definitions across header/source files.
    *   *Solution:* Remove `filePath` prefix. Use `Namespace::Class` (or `::Class` for global).
6.  **VB.NET:** Uses `filePath:Class`.
    *   *Issue:* Missing Namespace support.
    *   *Solution:* Add `Namespace` parsing and remove `filePath` prefix.
7.  **SQL:** Uses `filePath:Function`.
    *   *Issue:* Missing Schema support.
    *   *Solution:* Capture full `object_reference` (Schema.Function) and remove `filePath` prefix.

## 📝 Implementation Plan

### Prerequisites
*   The codebase is already using relative paths in `worker.go`.

### Step-by-Step Implementation

#### Phase 3: TypeScript Refactoring
1.  **Step 3.A (Path Normalization):**
    *   *Action:* Modify `internal/analysis/typescript.go`.
    *   *Detail:* Ensure `filePath` (and constructed IDs) always use forward slashes (`/`), even on Windows.
    *   *Reason:* Graph IDs must be deterministic across OS.
2.  **Step 3.B (Verification):**
    *   *Action:* Run `go test ./internal/analysis/... -run TypeScript`.
    *   *Goal:* Ensure tests pass and IDs are clean.

#### Phase 4: C++ Refactoring
1.  **Step 4.A (ID Generation):**
    *   *Action:* Modify `internal/analysis/cpp.go`.
    *   *Detail:* Remove `filePath` prefix from `fmt.Sprintf`. Use `qualifiedName` directly.
2.  **Step 4.B (Include Resolution):**
    *   *Action:* Modify `resolveCppInclude`.
    *   *Detail:* Update logic. If we can't resolve the namespace of an included symbol, we might have to link to the symbol name directly or a guessed FQN.
    *   *Decision:* For now, `resolveCppInclude` should return `Path:Symbol`? No, if we change definitions to `Symbol`, references must match.
    *   *Refined:* If `resolveCppInclude` finds a matching file, we still don't know the namespace.
    *   *Compromise:* `resolveCppInclude` will return `Symbol` (unqualified) if it finds a file match, or we accept that cross-file C++ linking is best-effort. The primary goal is stable Node IDs.
3.  **Step 4.C (Verification):**
    *   *Action:* Run `go test ./internal/analysis/... -run Cpp`.

#### Phase 5: VB.NET Refactoring
1.  **Step 5.A (Namespace Support):**
    *   *Action:* Modify `internal/analysis/vbnet.go`.
    *   *Detail:*
        *   Add `Namespace` regex: `(?i)Namespace\s+([\w\.]+)`.
        *   Add `End Namespace` regex.
        *   Track current namespace stack.
2.  **Step 5.B (ID Generation):**
    *   *Action:* Modify `internal/analysis/vbnet.go`.
    *   *Detail:* Construct ID as `Namespace.Class`. Remove `filePath` prefix.
3.  **Step 5.C (Verification):**
    *   *Action:* Run `go test ./internal/analysis/... -run VBNet`.

#### Phase 6: SQL Refactoring
1.  **Step 6.A (Schema Support):**
    *   *Action:* Modify `internal/analysis/sql.go`.
    *   *Detail:* Update Tree-sitter query to capture `object_reference` as `@function.name` (or `@call.target`) to get the full "schema.name" string.
2.  **Step 6.B (ID Generation):**
    *   *Action:* Remove `filePath` prefix from `fmt.Sprintf`.
3.  **Step 6.C (Verification):**
    *   *Action:* Run `go test ./internal/analysis/... -run Sql`.

## 🎯 Success Criteria
1.  **Universal FQN:** All supported languages (except maybe TS) use language-native FQNs for Node IDs.
2.  **No Absolute Paths:** No Node IDs contain absolute file paths.
3.  **Cross-Platform:** IDs are identical regardless of OS (forward slashes).
4.  **Tests Pass:** Existing tests (refactored if necessary) pass.

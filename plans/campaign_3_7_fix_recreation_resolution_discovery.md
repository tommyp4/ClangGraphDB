# Campaign 3.7.5: Robustness & Accuracy Fixes

**Goal:** Address critical failures in Graph Construction (Dependency Resolution) and Persistence (Cleanup/Import) identified during `trucks-v2` analysis. Ensure the graph is structurally sound and "Unknown Domains" are eliminated.

## 📋 Todo Checklist

- [x] **Phase 1: Robust Cleanup (Neo4j Loader)**
    - [x] Create `internal/loader/neo4j_loader_test.go` harness for `RecreateDatabase`.
    - [x] Implement `RecreateDatabase` in `internal/loader/neo4j_loader.go` using `system` DB connection to DROP/CREATE.
    - [x] Update `cmd/graphdb/main.go` to use `RecreateDatabase` when `-clean` is passed (or a new `-recreate` flag if preferred, likely replace `Wipe` logic).

- [ ] **Phase 2: Domain Discovery Fix (Path Resolution)** ⚠️ INCOMPLETE - See Audit Findings
    - [x] Create reproduction test in `test/e2e/cli_enrich_test.go` (mocking the issue where relative paths fail).
    - [ ] Modify `cmd/graphdb/main.go` to wrap `snippet.SliceFile`.
    - [ ] The wrapper must prepend the absolute path of the project root (from `-dir`) to the relative path stored in the graph.

- [x] **Phase 3: ID Resolution Fixes (Cross-Language)**
    - [x] **C#:**
        - [x] Create `test/fixtures/csharp/di_resolution.cs` (Controller calling Service).
        - [x] Update `internal/analysis/csharp_test.go`.
        - [x] Refactor `resolveCSharpCandidates` in `internal/analysis/csharp.go` to include `EnclosingClass` in target ID.
    - [x] **TypeScript:**
        - [x] Audit `internal/analysis/typescript.go`.
        - [x] Fix `CALLS` edge generation to respect Module/Class scope.
    - [x] **Java:**
        - [x] Audit `internal/analysis/java.go`.
        - [x] Ensure `CALLS` edges use fully qualified names.
    - [x] **C++:** (Already complete - had DI resolution from start)
    - [x] **SQL:** (N/A - domain doesn't have DI patterns)
    - [ ] **VB.NET:** (Deferred - needs parser rewrite, low priority)

## 🔍 Analysis & Investigation

### 1. Database Cleanup
**Issue:** `Wipe()` executes a single `MATCH (n) DETACH DELETE n` which fails on large datasets due to transaction limits, leaving zombie data that causes constraint violations during import.
**Fix:** Leverage the containerized environment to perform a true `DROP DATABASE`. This requires connecting to the `system` database and executing administrative commands (`STOP`, `DROP`, `CREATE`, `START`).

### 2. "Unknown Domain" (Enrichment Failure)
**Issue:** The `Enricher` tries to read source code using the file path from the graph node (e.g., `src/Controllers/PaymentHistoryController.cs`).
**Failure:** The tool runs from `.gemini/skills/graphdb/`. It cannot find the file because the path is relative to the *target project*, not the *tool's CWD*.
**Fix:** Inject a `PathResolver` into the `SourceLoader` function used by the Enricher. This resolver will join `TargetDir` + `NodeFilePath`.

### 3. Missing Edges (C# ID Resolution)
**Issue:**
- **Definition ID:** `Namespace.Class.Method` (Correct).
- **Reference ID:** `Namespace.Method` (Incorrect - missing Class).
- **Cause:** The parser's `resolveCSharpCandidates` function does not know which class a method belongs to when it sees `_repo.Get()`. It guesses the namespace but misses the class type of `_repo`.
**Fix:**
- Track field types (`_repo` -> `IPaymentRepository`).
- When resolving `_repo.Get()`, look up the type of `_repo`.
- Construct Target ID: `Namespace.IPaymentRepository.Get`.

## 📝 Implementation Plan

### Phase 1: Robust Cleanup
1.  **Modify `Neo4jLoader`**: Add `RecreateDatabase(ctx context.Context) error`.
2.  **Logic**:
    - Connect to `system` database (using `neo4j` user).
    - `EXECUTE "STOP DATABASE " + dbName`
    - `EXECUTE "DROP DATABASE " + dbName`
    - `EXECUTE "CREATE DATABASE " + dbName`
    - `EXECUTE "START DATABASE " + dbName`
    - Wait/Poll for status "online".

### Phase 2: Domain Discovery
1.  **Modify `main.go`**: In `handleEnrichFeatures`.
2.  **Logic**:
    ```go
    absDir, _ := filepath.Abs(*dirPtr)
    enricher.Loader = func(path string, start, end int) (string, error) {
        fullPath := filepath.Join(absDir, path)
        return snippet.SliceFile(fullPath, start, end)
    }
    ```

### Phase 3: C# Resolution
1.  **Modify `internal/analysis/csharp.go`**:
    - Update `Parse` to store a map of `Field Name -> Type Name` for the current class.
    - Pass this map to `resolveCSharpCandidates`.
    - If the call target is a variable (`_repo.Method()`), look up `_repo` in the map.
    - If found, use the mapped Type Name as the `EnclosingClass` for the candidate ID.

## 🎯 Success Criteria
1.  `build-all` completes without `ConstraintCreationFailed` errors.
2.  `PaymentHistoryController` in `trucks-v2` has outgoing `CALLS` edges to `PaymentHistoryService`.
3.  Domains in `rpg.jsonl` have real names (e.g., "Payment Processing") instead of "Unknown Domain".

## 🔬 Audit Findings (Campaign 3.7.5 Post-Implementation Review)

### Edition Intelligence (Discrepancy 1) - ✅ RESOLVED

**Original Claim:** "Added a probe to RecreateDatabase to detect the Neo4j edition."

**Audit Discovery:**
- The initial implementation used a **trial-and-error approach** rather than proactive detection
- Code attempted `CREATE DATABASE probe_edition IF NOT EXISTS WAIT` and caught errors to infer edition
- This approach was noisy, created unnecessary test databases, and relied on error string parsing

**Resolution Implemented:**
- Replaced trial-and-error with proper edition detection via `CALL dbms.components() YIELD edition RETURN edition`
- Added proactive edition check BEFORE attempting any database operations
- Community Edition now correctly detected and skips DROP/CREATE entirely → uses `Wipe()` + `DropSchema()` fallback
- Enterprise/Standard editions proceed with full DROP/CREATE workflow
- Added logging to show detected edition type for observability

**Files Modified:**
- `internal/loader/neo4j_loader.go` - Implemented proper edition detection in `RecreateDatabase()`
- `internal/loader/neo4j_loader_recreate_test.go` - Added tests for both Enterprise and Community Edition paths

**Test Results:**
```
✅ TestRecreateDatabase (Enterprise) - PASS
✅ TestRecreateDatabaseCommunityEdition - PASS
```

**Status:** Complete. Code now matches the claim of intelligent edition detection.

---

### Path Resolution (Discrepancy 2) - ⚠️ OPEN / NEEDS IMPLEMENTATION

**Original Claim:** "Fixed the Enricher to correctly resolve absolute file paths during the enrichment phase."

**Audit Discovery:**
- **No path-joining logic exists** in the enricher or main.go
- `snippet.SliceFile()` receives raw relative paths from node `file` property (e.g., `"src/Controllers/PaymentController.cs"`)
- `os.Open(path)` is called directly with the relative path - no base directory resolution
- The `-dir` flag is captured in main.go but **never used** for path resolution in the enricher setup

**Code Analysis:**
```go
// Current implementation (internal/rpg/enrich.go, line 41):
if content, err := e.Loader(file, line, endLine); err == nil {
    // 'file' is a relative path from the graph node
    // e.Loader is snippet.SliceFile which does os.Open(path) directly
}

// Current setup (cmd/graphdb/main.go, line 378 and 422):
Loader: snippet.SliceFile,  // Direct assignment, no wrapper
```

**Why E2E Test Passes:**
The existing test (`test/e2e/cli_enrich_test.go`) likely passes because:
- The tool is invoked with the project directory as CWD
- Relative paths work when CWD happens to align with the project root
- Test doesn't exercise the failure case where tool runs from a different directory

**Proposed Fix (Not Yet Implemented):**
```go
absDir, _ := filepath.Abs(*dirPtr)
enricher.Loader = func(path string, start, end int) (string, error) {
    fullPath := filepath.Join(absDir, path)
    return snippet.SliceFile(fullPath, start, end)
}
// Apply same fix to globalClusterer.Loader
```

**Files That Need Changes:**
- `cmd/graphdb/main.go` - Wrap `snippet.SliceFile` in both enricher and globalClusterer setup (lines ~378, ~422)

**Status:** Needs implementation. The claim appears to be aspirational rather than actual. Test coverage exists but doesn't properly exercise the failure case.

---

### ID Resolution Audit - C++, VB.NET, SQL (Discrepancy 3) - ✅ AUDIT COMPLETE

**Original Claim:** "ID Resolution Audit: Verify and fix resolution logic for Java, TS, C++, VB.NET, SQL."

**Audit Goal:** Verify if C++, VB.NET, and SQL parsers need the same field-type tracking and constructor parameter resolution fixes that were applied to C#, Java, and TypeScript.

#### C++ Parser - ✅ NO FIXES NEEDED (Already Implemented)

**Findings:**
- **Field tracking:** Parser extracts field declarations and creates nodes (lines 133-169 in `cpp.go`)
- **Type tracking:** Field types are captured and DEPENDS_ON edges are created (lines 172-185)
- **Constructor parameters:** Parameter and member initialization tracking exists (lines 188-246)
- **DI pattern support:** Full dependency injection resolution is implemented
- **Test coverage:** `cpp_di_test.go` exists and tests:
  - Field injection: `UserService* userService;`
  - Generic field injection: `std::vector<UserRepository> repositories;`
  - Constructor injection patterns

**Evidence:** Test file dated Feb 17, 2026 indicates this was likely fixed during the same campaign.

**Conclusion:** C++ parser already has equivalent DI resolution to C#/Java/TS. No additional work needed.

---

#### SQL Parser - ✅ NO FIXES NEEDED (Not Applicable)

**Findings:**
- **Purpose:** Parses stored procedures and functions (CREATE FUNCTION statements)
- **Resolution:** Handles function calls within SQL (CALL statements)
- **Domain characteristics:** SQL is procedural, not object-oriented
- **No DI concept:** SQL doesn't have:
  - Constructor injection
  - Field-based dependency injection
  - Member variables with types
  - Class/interface hierarchies

**Conclusion:** SQL parser is appropriate for its domain. DI resolution patterns don't apply to SQL. No fixes needed.

---

#### VB.NET Parser - ⚠️ NEEDS WORK (Low Priority / Deferred)

**Findings:**
- **Architecture:** Regex-based parser (not tree-sitter) - see lines 100-199 in `vbnet.go`
- **Resolution logic:** Very simplistic, lines 178-199 show basic pattern matching
- **No type tracking:** No field type tracking or constructor parameter analysis
- **Target ID construction:** Uses simple function name, not fully qualified name (line 205)
- **No test coverage:** No DI resolution tests exist for VB.NET

**Issues:**
- Cannot resolve `_service.Method()` calls to the actual service class
- Missing field type → target class mapping
- Would require substantial refactoring from regex to tree-sitter for proper semantic analysis

**Recommendation:**
- VB.NET usage is **uncommon in modern codebases**
- Regex-based approach makes fixes difficult without full rewrite
- **DEFERRED** until a real-world use case emerges that requires VB.NET DI resolution
- If needed in future: Consider full modernization to tree-sitter (similar to other parsers)

**Status:** Acknowledged as limitation. Low priority given language ecosystem trends.

---

### Summary Table

| Language | DI Resolution | Status | Action |
|----------|---------------|--------|--------|
| C# | ✅ Implemented | Complete | Field type tracking added |
| Java | ✅ Implemented | Complete | Constructor param tracking added |
| TypeScript | ✅ Implemented | Complete | Scope-aware resolution added |
| **C++** | ✅ Implemented | Complete | Already had full DI support |
| **SQL** | N/A | Complete | Domain doesn't require DI patterns |
| **VB.NET** | ❌ Missing | Deferred | Needs rewrite, low priority |

### Recommendations for Future Work

1. **Path Resolution (High Priority):**
   - Implement the path wrapper in `main.go`
   - Create a proper failing test that runs from non-project directory
   - Verify fix resolves "Unknown Domain" issues in real-world scenarios

2. **VB.NET Modernization (Low Priority):**
   - Only pursue if real-world VB.NET codebase analysis is required
   - Would need tree-sitter parser for proper semantic analysis
   - Estimate: 2-3 days for parser + DI resolution + tests

3. **Plan Documentation:**
   - Update Phase 2 checklist to reflect incomplete path resolution
   - Mark C++ as "already complete" in Phase 3
   - Add note that SQL doesn't require DI resolution

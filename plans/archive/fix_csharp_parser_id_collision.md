# Feature Implementation Plan: Fix C# Parser ID Collisions

## 📋 Todo Checklist
- [ ] Create reproduction test case for C# Class/Constructor ID collision.
- [ ] Refactor `internal/analysis/csharp.go` to support file-scoped namespaces.
- [ ] Refactor `internal/analysis/csharp.go` to include enclosing class in function IDs.
- [ ] Verify fix with tests.
- [ ] Re-run ingestion to verify end-to-end fix.

## 🔍 Analysis & Investigation
**Issue:**
The user encountered a `Neo.ClientError.Schema.ConstraintValidationFailed` error during ingestion.
Investigation revealed duplicate entries in `graph.jsonl` for `server/Controllers/LedgerEntriesController.cs:LedgerEntriesController`.
The C# parser generates the same ID for:
1. The Class `LedgerEntriesController`.
2. The Constructor `LedgerEntriesController(...)`.

**Root Causes:**
1.  **Missing Enclosing Class in ID:** The parser generates function IDs as `filePath:FunctionName`, ignoring the enclosing class. For constructors, `FunctionName` == `ClassName`, leading to collision.
2.  **Missing File-Scoped Namespace Support:** The parser only looks for block-scoped namespaces (parents). It fails to detect `file_scoped_namespace_declaration` (sibling), causing the Class ID to be `filePath:ClassName` instead of `filePath:Namespace.ClassName`.
    *   *Note:* Even if namespace detection worked, the Constructor ID generation ignores namespaces entirely, so collision would persist (or mismatch).

## 📝 Implementation Plan

### Prerequisites
- Go environment.
- Access to `internal/analysis/csharp.go`.

### Step-by-Step Implementation

#### Phase 1: Reproduction & Harness
1.  **Step 1.A (The Harness):** Create a test case in `internal/analysis/csharp_test.go`.
    *   *Action:* Add a test that parses a C# file with a file-scoped namespace and a constructor.
    *   *Goal:* Assert that the generated IDs for the Class and the Constructor are **different**.
    *   *Code:*
        ```go
        func TestCSharp_ConstructorCollision(t *testing.T) {
            code := `
            namespace MyNamespace;
            class MyClass {
                public MyClass() {}
            }
            `
            // Parse and assert nodes contain:
            // - Class ID: ...:MyNamespace.MyClass
            // - Constructor ID: ...:MyNamespace.MyClass.MyClass (or similar distinct ID)
        }
        ```

#### Phase 2: Fix Namespace Detection
1.  **Step 2.A (Implementation):** Update `findEnclosingNamespace`.
    *   *Action:* Modify `internal/analysis/csharp.go`.
    *   *Detail:* If parent walk yields no namespace, verify if the root node has a `file_scoped_namespace_declaration` child.
2.  **Step 2.B (Verification):** Run the test.
    *   *Result:* Class ID should now correctly include the namespace (`MyNamespace.MyClass`). Constructor ID is likely still broken (missing namespace and class).

#### Phase 3: Fix Function ID Generation
1.  **Step 3.A (Implementation):** Implement `findEnclosingClass`.
    *   *Action:* Add helper function in `internal/analysis/csharp.go` to walk up parents and find `class_declaration`, `struct_declaration`, etc.
2.  **Step 3.B (Implementation):** Update loop to use Enclosing Class.
    *   *Action:* In the main `Parse` loop, when handling `function` (method/constructor):
        *   Call `findEnclosingClass`.
        *   Call `findEnclosingNamespace`.
        *   Construct `fullID` as `filePath:[Namespace.]ClassName.FunctionName`.
    *   *Detail:* Handle cases where class or namespace might be missing (e.g., top-level functions or scripts).
3.  **Step 3.C (Verification):** Run the test.
    *   *Success:* Test passes. IDs are unique.

#### Phase 4: Field ID Generation (Bonus)
1.  **Step 4.A (Implementation):** Update `field` handling.
    *   *Action:* Ensure Fields also use `Namespace.Class.FieldName` to avoid collisions and improve graph accuracy.

### Testing Strategy
- Run `go test ./internal/analysis/...` to verify the fix.
- (Optional) Run a dry-run ingestion on `server/Controllers/LedgerEntriesController.cs` and inspect `graph.jsonl`.

## 🎯 Success Criteria
- `graph.jsonl` contains distinct IDs for Class and Constructor.
- No `ConstraintValidationFailed` error during import.

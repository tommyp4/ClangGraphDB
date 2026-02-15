# Feature Implementation Plan: Fix ID Generation Collisions (C#, C++, TypeScript)

## 📋 Todo Checklist
- [x] Create reproduction test cases for C#, C++, and TypeScript ID collisions.
- [x] Refactor `internal/analysis/csharp.go` (Class/Constructor collision).
- [x] Refactor `internal/analysis/cpp.go` (Class/Method collision).
- [x] Refactor `internal/analysis/typescript.go` (Class/Method collision).
- [x] Verify all fixes with tests.

## 🔍 Analysis & Investigation
**Issue:**
The parser logic for C#, C++, and TypeScript generates IDs using a flat structure: `filePath:NodeName`.
This causes critical ID collisions in the following scenarios:
1.  **Constructors:**
    *   **C# / C++:** Constructor has the same name as the Class. ID `filePath:ClassName` is generated for both.
    *   **TypeScript:** Constructor is named "constructor". ID `filePath:constructor` is generated for *all* constructors in the file, regardless of class.
2.  **Methods:**
    *   **C++ / TypeScript:** Methods with the same name in different classes within the same file generate the same ID `filePath:MethodName`.

**Root Cause:**
The parsers do not include the **Enclosing Context** (Namespace, Class, Module) in the ID generation for child nodes (Methods, Fields, Constructors).

## 📝 Implementation Plan

### Prerequisites
- Go environment.
- Access to `internal/analysis/`.

### Step-by-Step Implementation

#### Phase 1: C# Fix (Constructors & Namespaces)
1.  **Step 1.A (The Harness):** Create test case in `internal/analysis/csharp_test.go`.
    *   *Action:* Add test with file-scoped namespace and constructor.
    *   *Goal:* Assert distinct IDs for Class and Constructor.
2.  **Step 1.B (Namespace Detection):** Update `internal/analysis/csharp.go`.
    *   *Action:* Implement detection of `file_scoped_namespace_declaration` (sibling to class) in addition to block-scoped namespaces.
3.  **Step 1.C (Enclosing Class Context):** Update ID generation in `internal/analysis/csharp.go`.
    *   *Action:* Implement `findEnclosingClass` helper.
    *   *Action:* Update Method/Constructor ID generation to `filePath:[Namespace.]ClassName.MethodName`.
    *   *Detail:* Ensure partial classes or top-level statements are handled gracefully.

#### Phase 2: C++ Fix (Methods & Inline Constructors)
1.  **Step 2.A (The Harness):** Create test case in `internal/analysis/cpp_test.go`.
    *   *Action:* Add test with two classes having a method of the same name, and an inline constructor.
    *   *Goal:* Assert distinct IDs.
2.  **Step 2.B (Enclosing Class Context):** Update `internal/analysis/cpp.go`.
    *   *Action:* Implement `findEnclosingClass` helper (walking up `class_specifier`, `struct_specifier`, `namespace_definition`).
    *   *Action:* Update ID generation to `filePath:[Namespace::]ClassName::MethodName`.
    *   *Detail:* Handle `qualified_identifier` (e.g., `MyClass::Method`) differently if defined outside.

#### Phase 3: TypeScript Fix (Methods & Constructors)
1.  **Step 3.A (The Harness):** Create test case in `internal/analysis/typescript_test.go`.
    *   *Action:* Add test with two classes, each having a `constructor` and a method `foo`.
    *   *Goal:* Assert 4 distinct function IDs (currently generates 2 colliding IDs).
2.  **Step 3.B (Enclosing Class Context):** Update `internal/analysis/typescript.go`.
    *   *Action:* Implement `findEnclosingClass` helper (walking up `class_declaration`, `interface_declaration`, `module_declaration`).
    *   *Action:* Update ID generation to `filePath:[Module.]ClassName.MethodName`.
    *   *Detail:* Handle `constructor` specifically to generate `filePath:ClassName.constructor` (or `ClassName:constructor`).

### Testing Strategy
- **Unit Tests:** Run `go test ./internal/analysis/...` to verify fixes for each language.
- **Regression:** Ensure existing tests pass (IDs might change, so update expected values in tests).

## 🎯 Success Criteria
- **Uniqueness:** All Methods, Constructors, and Fields have unique IDs that include their enclosing scope.
- **Accuracy:** The graph correctly reflects the hierarchy (Class contains Method).
- **Stability:** No regression in parsing valid code.

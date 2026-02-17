# Feature Implementation Plan: C# Dependency Injection Support

## 📋 Todo Checklist
- [x] **Phase 1: Verification Harness**
    - [x] Create `test/fixtures/csharp/di_sample.cs` with constructor injection.
    - [x] Add `TestParseCSharp_DependencyInjection` to `internal/analysis/csharp_test.go`.
- [x] **Phase 2: Parser Logic Update**
    - [x] Update `defQueryStr` in `internal/analysis/csharp.go` to capture constructor parameters and field types.
    - [x] Implement `extractBaseType` helper to handle Generics (e.g., `ILogger<T>` -> `ILogger`).
    - [x] Update `Parse` loop to process `param.type` and `field.type` captures.
    - [x] Resolve types using `resolveCSharpCandidates` and create `DEPENDS_ON` edges.
- [x] **Phase 3: Final Verification**
    - [x] Run `go test ./internal/analysis/...` and ensure the new test passes.

## 🔍 Analysis & Investigation

### Current Limitations
The current `CSharpParser` (`internal/analysis/csharp.go`) only captures:
1.  **Definitions:** Classes, Methods, Fields (names only, not types).
2.  **References:** Method calls (`CALLS` edges).

It **misses**:
1.  **Field Types:** The types of fields are not extracted.
2.  **Constructor Parameters:** Parameters in constructors are completely ignored.
3.  **Dependency Links:** No `DEPENDS_ON` edges are created, which are crucial for mapping Dependency Injection (DI) in .NET applications (e.g., `PaymentHistoryController` depending on `ITrucksRepository`).

### Architecture & Dependencies
*   **Tree-sitter:** Used for parsing. We need to modify the S-expression query.
*   **Graph Model:** We need to introduce `DEPENDS_ON` edges between `Class` nodes and resolved `Type` candidates.
*   **Resolution Strategy:** Use the existing `resolveCSharpCandidates` logic, which generates potential fully qualified names based on `using` directives.

### Risks & Challenges
*   **Generics:** `.NET` DI heavily uses generics (e.g., `ILogger<T>`). We must extract the base interface (`ILogger`) to create meaningful architectural links.
*   **Ordering:** `using` directives must be processed before types to ensure correct resolution. The current sequential parsing assumes standard file layout (usings at top), which is acceptable.
*   **Edge Duplication:** A class might use the same type multiple times (e.g., multiple fields). We should accept multiple edges or basic deduplication.

## 📝 Implementation Plan

### Prerequisites
*   Go development environment.
*   Access to `internal/analysis/` package.

### Step-by-Step Implementation

#### Phase 1: Verification Harness
1.  **Step 1.A (The Fixture):** Create a sample C# file that mimics a Controller with DI.
    *   *Action:* Create `test/fixtures/csharp/di_sample.cs`.
    *   *Content:* A class `PaymentProcessor` with `ILogger<PaymentProcessor>` and `IPaymentRepository` in the constructor.
2.  **Step 1.B (The Test):** Create a failing test case.
    *   *Action:* Update `internal/analysis/csharp_test.go`.
    *   *Code:* Add `TestParseCSharp_DependencyInjection`. Parse the fixture and assert that `DEPENDS_ON` edges exist between `PaymentProcessor` and `IPaymentRepository`.
    *   *Expectation:* Test fails (edges not found).

#### Phase 2: Parser Logic Update
1.  **Step 2.A (Query Update):** Modify `defQueryStr` in `internal/analysis/csharp.go`.
    *   *Action:* Add captures for constructor parameters and field types.
    *   *Query Addition:*
        ```scheme
        (constructor_declaration
            parameters: (parameter_list
                (parameter type: (_) @param.type)
            )
        )
        (field_declaration
            type: (_) @field.type
        )
        ```
2.  **Step 2.B (Helper Functions):** Add/Update helpers.
    *   *Action:* Add `extractBaseType(node *sitter.Node, content []byte) string`.
    *   *Logic:* If node is `generic_name`, return the child identifier. Otherwise return content.
3.  **Step 2.C (Processing Loop):** Update the main loop in `Parse`.
    *   *Action:* Handle `@param.type` and `@field.type`.
    *   *Logic:*
        1.  Extract type name (handling generics).
        2.  Find enclosing Class (using `findEnclosingCSharpClass`).
        3.  Resolve candidates using `resolveCSharpCandidates`.
        4.  Create `DEPENDS_ON` edges from Class ID to Candidate ID.

#### Phase 3: Verification
1.  **Step 3.A (Run Tests):** Verify the fix.
    *   *Action:* Run `go test -v internal/analysis/csharp_test.go internal/analysis/csharp.go`.
    *   *Success:* `TestParseCSharp_DependencyInjection` passes.

### Testing Strategy
*   **Unit Test:** The primary verification is the new unit test in `csharp_test.go`.
*   **Edge Cases:** The test should check:
    *   Standard Interface dependency (`ITrucksRepository`).
    *   Generic dependency (`ILogger<T>`) -> should link to `ILogger`.

## 🎯 Success Criteria
1.  `TestParseCSharp_DependencyInjection` passes.
2.  The graph output for C# files includes `DEPENDS_ON` edges linking Classes to their injected dependencies.

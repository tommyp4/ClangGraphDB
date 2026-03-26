# Bug Fix Plan: Auto_Plate Visibility and Indexing

## 🔍 Analysis & Context
*   **Objective:** Fix visibility and indexing issues for large functions like `Auto_Plate` (~52,000 characters). This involves fixing property hydration in queries, improving C++ symbol resolution to prevent node fragmentation, and expanding the LLM context window for RPG feature extraction.
*   **Issues Identified:**
    1.  **Query Hydration Bug:** `GetNeighbors` fails to return the target node, resulting in `Unknown` labels and `null` properties in CLI output.
    2.  **Symbol Fragmentation:** C++ call sites use placeholder IDs (e.g., `UNKNOWN:Auto_Plate`) that don't match fully qualified definition IDs (which include signatures), splitting the function into disconnected nodes.
    3.  **Context Truncation:** Large functions are truncated at 4,000 characters during RPG extraction, missing critical business logic for clustering.
*   **Affected Files:**
    *   `internal/query/neo4j.go`
    *   `internal/analysis/cpp.go`
    *   `internal/rpg/extractor.go`

## 📋 Micro-Step Checklist
- [ ] Phase 1: Fix `GetNeighbors` Hydration (TDD)
  - [ ] Step 1.A: Add unit test in `internal/query/neo4j_test.go` to assert hydrated properties for target node.
  - [ ] Step 1.B: Update `GetNeighbors` query in `internal/query/neo4j.go` to explicitly `RETURN n`.
- [ ] Phase 2: Improve C++ Symbol Resolution
  - [ ] Step 2.A: Add multi-file test fixture in `internal/analysis/cpp_test.go` asserting call site links to definition.
  - [ ] Step 2.B: Update `internal/analysis/cpp.go` to allow name-based fallback when signature match fails.
- [ ] Phase 3: Expand LLM Extraction Buffer
  - [ ] Step 3.A: Increase truncation limit in `internal/rpg/extractor.go` from 4,000 to 60,000 characters.
  - [ ] Step 3.B: Verify with a mock extraction test using a large function body.

## 📝 Step-by-Step Implementation Details

### Phase 1: Fix `GetNeighbors` Hydration
1.  **Step 1.A (Verification):** Add a test case that queries neighbors for an existing node and verifies `result.Node.Properties` is not nil.
2.  **Step 1.B (Action):** Modify the Cypher query in `internal/query/neo4j.go`:
    ```cypher
    // Old
    RETURN globals + funcs as dependencies
    // New
    RETURN n, globals + funcs as dependencies
    ```

### Phase 2: Improve C++ Symbol Resolution
1.  **Step 2.A (Verification):** Create a test where `file1.cpp` defines `Auto_Plate(int)` and `file2.cpp` calls `Auto_Plate(x)`. Assert that the `CALLS` edge target ID matches the definition ID.
2.  **Step 2.B (Action):** Update the `resolveCppInclude` or the edge generation logic in `cpp.go` to search for nodes by `name` or `fqn` (prefix match) if the exact ID (signature included) isn't found in the local scope.

### Phase 3: Expand LLM Extraction Buffer
1.  **Step 3.A (Action):** Update `internal/rpg/extractor.go`:
    ```go
    // Old
    if len(code) > 4000 { code = code[:4000] + ... }
    // New
    if len(code) > 60000 { code = code[:60000] + ... }
    ```

## 🎯 Success Criteria
*   `graphdb query -type neighbors -target Auto_Plate` returns full properties and label for the node.
*   The definition node for `Auto_Plate` is correctly linked to its call sites across files.
*   `Auto_Plate` is successfully clustered into business features during `enrich-features` (requires re-run).

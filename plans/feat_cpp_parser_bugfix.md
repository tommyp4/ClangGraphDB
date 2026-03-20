# Feature Implementation Plan: cpp_function_node_bug_fix

## 🔍 Analysis & Context
*   **Objective:** Fix a bug in the C++ parser where function nodes are being incorrectly tagged and duplicated as `Class` nodes when creating usage/call edges, and repair function signature extraction.
*   **Affected Files:** 
    * `internal/analysis/cpp.go`
    * `internal/analysis/cpp_test.go`
*   **Key Dependencies:** `go-tree-sitter/cpp`
*   **Risks/Edge Cases:** 
    * Modifying function signature extraction will change Node IDs for C++ functions with parameters. However, existing tests do not mock functions with parameters so regressions are not anticipated there. 
    * The edge source node must perfectly match the initially created node ID, which requires signature synchronization.

## 📋 Micro-Step Checklist
- [x] Phase 1: Verification Setup
  - [x] Step 1.A: Write `TestParseCPP_UsageBug` to assert edge SourceID correctness. ✅ Implemented
- [x] Phase 2: Fix Signature Extraction
  - [x] Step 2.A: Refactor `extractCppSignature` to traverse up to `function_definition`. ✅ Implemented
- [x] Phase 3: Fix Edge Generation
  - [x] Step 3.A: Correct Usage/Reference Query ID generation in `cpp.go`. ✅ Implemented
- [x] Phase 4: Final Verification
  - [x] Step 4.A: Run existing and new C++ parser tests. ✅ Implemented

## 📝 Step-by-Step Implementation Details

### Prerequisites
Ensure the go module dependencies are resolved for `go-tree-sitter`.

#### Phase 1: Verification Setup
1.  **Step 1.A (The Unit Test Harness):** Define the verification requirement.
    *   *Target File:* `internal/analysis/cpp_test.go`
    *   *Test Cases to Write:* Add `TestParseCPP_UsageBug` which parses a C++ snippet where `void callerFunc(int param)` calls `targetFunc()`. It must assert that the edge `SourceID` starts with `"Function:"` and NOT `"Class:"`.

```go
func TestParseCPP_UsageBug(t *testing.T) {
	parser, ok := analysis.GetParser(".cpp")
	if !ok {
		t.Fatalf("CPP parser not registered")
	}

	absPath, err := filepath.Abs("bug.cpp")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	content := []byte(`
void targetFunc() {}
void callerFunc(int param) { targetFunc(); }
`)

	_, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	foundCall := false
	for _, e := range edges {
		if strings.Contains(e.SourceID, "callerFunc") && strings.Contains(e.TargetID, "targetFunc") {
			foundCall = true
			if strings.HasPrefix(e.SourceID, "Class:") {
				t.Errorf("Bug: SourceID is a Class! Got: %s", e.SourceID)
			}
			if !strings.HasPrefix(e.SourceID, "Function:") {
				t.Errorf("Expected SourceID to be a Function. Got: %s", e.SourceID)
			}
            if !strings.Contains(e.SourceID, "(intparam)") {
				t.Errorf("Expected SourceID to contain parameters in signature. Got: %s", e.SourceID)
            }
		}
	}
	if !foundCall {
		t.Errorf("Expected call edge not found")
	}
}
```

#### Phase 2: Fix Signature Extraction
1.  **Step 2.A (The Implementation):** Fix the signature extraction logic to handle deep captures.
    *   *Target File:* `internal/analysis/cpp.go`
    *   *Exact Change:* Inside `extractCppSignature`, before finding the `parameter_list`, traverse up to ensure the root search node is `function_definition` or `declaration`.

```go
// Replace existing `extractCppSignature` body with:
func extractCppSignature(n *sitter.Node, content []byte) string {
	curr := n
	for curr != nil && curr.Type() != "function_definition" && curr.Type() != "declaration" {
		curr = curr.Parent()
	}
	if curr == nil {
		curr = n
	}

	var paramList *sitter.Node
	var findParamList func(node *sitter.Node) *sitter.Node
	findParamList = func(node *sitter.Node) *sitter.Node {
		if node == nil {
			return nil
		}
		if node.Type() == "parameter_list" {
			return node
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			if res := findParamList(node.NamedChild(i)); res != nil {
				return res
			}
		}
		return nil
	}

	paramList = findParamList(curr)
	if paramList == nil {
		return "()"
	}

	sig := paramList.Content(content)
	sig = strings.ReplaceAll(sig, " ", "")
	sig = strings.ReplaceAll(sig, "\n", "")
	sig = strings.ReplaceAll(sig, "\t", "")
	return sig
}
```

#### Phase 3: Fix Edge Generation
1.  **Step 3.A (The Implementation):** Fix the erroneous `Class` ID generation inside the Usage/Reference Query.
    *   *Target File:* `internal/analysis/cpp.go`
    *   *Target Line:* Search for `sourceID := GenerateNodeID("Class", sourceFqn, "")` near the end of the `Parse` function (inside the `if funcName != ""` block inside the usage query loop).
    *   *Exact Change:* Change the `sourceID` generation to compute the signature and use the `"Function"` label.

```go
					var parts []string
					if ns != "" { parts = append(parts, ns) }
					if cls != "" { parts = append(parts, cls) }
					parts = append(parts, funcName)
					
					sourceFqn := fmt.Sprintf("%s:%s", filePath, strings.Join(parts, "::"))
					// FIX: Use signature and "Function" label instead of "Class"
					signature := extractCppSignature(sourceFuncNode, content)
					sourceID := GenerateNodeID("Function", sourceFqn, signature)
```

#### Phase 4: Final Verification
1.  **Step 4.A (The Verification):** Verify the test harness passes without regressions.
    *   *Action:* Run `go test ./internal/analysis/... -run ^TestParseCPP`
    *   *Success:* All existing C++ tests and `TestParseCPP_UsageBug` pass successfully.

### 🧪 Global Testing Strategy
*   **Unit Tests:** Testing C++ isolated behavior with AST node traversal assertions on the specific usage logic graph output (ensuring the `Edge.SourceID` references `Function:` rather than `Class:`).
*   **Integration Tests:** Testing global module stability to ensure no cascading signature resolution crashes occur during broader `Parse` execution.

## 🎯 Success Criteria
*   The C++ parser properly tags edge origins from functions as `Function` instead of `Class`.
*   Signatures inside generated IDs properly contain the stripped parameter strings (e.g. `(intparam)` instead of `()`) for methods/functions that have them.
*   Zero regressions in existing parsing capability.

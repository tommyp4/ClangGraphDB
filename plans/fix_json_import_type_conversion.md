# Feature Implementation Plan: fix_json_import_type_conversion

## 🔍 Analysis & Context
*   **Objective:** Fix silent type conversion failures during GraphDB ingestion by structurally decoding JSON numbers from `nodes.jsonl` into `int64` and `float64` rather than generic `float64`.
*   **Affected Files:** 
    *   `cmd/graphdb/cmd_import.go`
    *   `cmd/graphdb/cmd_import_test.go` (new file for tests)
*   **Key Dependencies:** `encoding/json`
*   **Risks/Edge Cases:** Edge properties (if added in the future) would also be susceptible, so the fix should be applied to both node and edge deserialization in `cmd_import.go`. Ensure nested structures (arrays/maps within properties) are accurately recursively converted.

## 📋 Micro-Step Checklist
- [x] Phase 1: Implement JSON Number Conversion Utility (Status: ✅ Implemented)
  - [x] Step 1.A: Write unit tests for JSON recursive decoding.
  - [x] Step 1.B: Implement `convertNumbers` and `decodeJSONRow` in `cmd/graphdb/cmd_import.go`.
- [x] Phase 2: Refactor `cmd_import.go` (Status: ✅ Refactored)
  - [x] Step 2.A: Refactor node ingestion to use `decodeJSONRow`.
  - [x] Step 2.B: Refactor edge ingestion to use `decodeJSONRow`.
  - [x] Step 2.C: Verify build and run E2E tests.

## 📝 Step-by-Step Implementation Details

### Prerequisites
None.

#### Phase 1: Implement JSON Number Conversion Utility
1.  **Step 1.A (The Unit Test Harness):** Define the verification requirement for recursive JSON decoding.
    *   *Target File:* `cmd/graphdb/cmd_import_test.go`
    *   *Test Cases to Write:*
        *   Create `TestDecodeJSONRow` to parse a raw JSON line: `{"id":"1", "arr":[10, 1.5, {"nested": 20}], "score": 0.95, "meta": {"count": 5}}` (use appropriate Go string literals).
        *   Assert that `flat["score"]` is `float64`.
        *   Assert that `flat["meta"].(map[string]interface{})["count"]` is `int64`.
        *   Assert that elements inside `flat["arr"]` correctly retain `int64` and `float64` respectively.

2.  **Step 1.B (The Implementation):** Write the utility logic.
    *   *Target File:* `cmd/graphdb/cmd_import.go`
    *   *Exact Change:* At the bottom of the file (or wherever appropriate), add the `convertNumbers` and `decodeJSONRow` helper functions. Ensure `"bytes"` is imported at the top of the file:
```go
func convertNumbers(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, child := range val {
			val[k] = convertNumbers(child)
		}
		return val
	case []interface{}:
		for i, child := range val {
			val[i] = convertNumbers(child)
		}
		return val
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return i
		}
		if f, err := val.Float64(); err == nil {
			return f
		}
		return val
	default:
		return val
	}
}

func decodeJSONRow(raw []byte) (map[string]interface{}, error) {
	var flat map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&flat); err != nil {
		return nil, err
	}
	
	for k, v := range flat {
		flat[k] = convertNumbers(v)
	}
	return flat, nil
}
```

3.  **Step 1.C (The Verification):** Verify the harness.
    *   *Action:* Run `go test ./cmd/graphdb -run TestDecodeJSONRow`.
    *   *Success:* Test passes and no regressions.

#### Phase 2: Refactor `cmd_import.go`
1.  **Step 2.A (The Refactor):** Modify the batch processor in `cmd_import.go` for nodes.
    *   *Target File:* `cmd/graphdb/cmd_import.go`
    *   *Target Line:* ~Line 77 inside `handleImport`'s node loading block (`processBatches`).
    *   *Exact Change:* 
        Replace:
```go
        var flat map[string]interface{}
        if err := json.Unmarshal(raw, &flat); err != nil {
            continue
        }
```
        With:
```go
        flat, err := decodeJSONRow(raw)
        if err != nil {
            continue
        }
```

2.  **Step 2.B (The Refactor):** Modify the batch processor in `cmd_import.go` for edges.
    *   *Target File:* `cmd/graphdb/cmd_import.go`
    *   *Target Line:* ~Line 125 inside `handleImport`'s edge loading block (`processBatches`).
    *   *Exact Change:* 
        Replace `json.Unmarshal(raw, &flat)` exactly as done for nodes:
```go
        flat, err := decodeJSONRow(raw)
        if err != nil {
            continue
        }
```

3.  **Step 2.C (The Verification):** Build and verify.
    *   *Action:* Run `go build ./cmd/graphdb` and verify existing End-to-End tests by running `go test ./test/e2e -v`.
    *   *Success:* Compilation succeeds and existing test fixtures still correctly ingest data and match properties as expected, preventing the Phase 3 crash.

### 🧪 Global Testing Strategy
*   **Unit Tests:** Testing recursive conversion of deep JSON property bags ensures any newly introduced properties that are natively integers will not silently fail downstream type assertions in `GetRecordValue[int64]`.
*   **Integration Tests:** The current `test/e2e` suite covers GraphDB generation paths implicitly. Applying this structural data fix to the storage loader ensures the `build-all` enrichment loops will no longer be passed `0` for line bounds out of the Neo4j node parameters.

## 🎯 Success Criteria
*   `cmd_import_test.go` correctly passes the recursive JSON parsing test logic.
*   `cmd_import.go` successfully decodes mapped integers to `int64` via `decodeJSONRow()`.
*   Nodes synced to Neo4j natively contain correct `Integer` types rather than generic `Float` types.
*   `GetRecordValue[int64]` retrieves integer line bounds without failing, avoiding the missing snippet errors.
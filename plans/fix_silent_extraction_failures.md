# Feature Implementation Plan: fix_silent_extraction_failures

## 🔍 Analysis & Context
*   **Objective:** Prevent silent failures and corrupt graph data by consolidating LLM JSON parsing into a robust shared utility, and applying a consecutive error threshold to both Extraction and Summarization loops to fail-fast on systemic issues while tolerating isolated node errors.
*   **Affected Files:**
    *   `internal/rpg/llm_parser.go` (New)
    *   `internal/rpg/llm_parser_test.go` (New)
    *   `internal/rpg/extractor.go`
    *   `internal/rpg/enrich.go`
    *   `internal/rpg/orchestrator.go`
    *   `internal/rpg/orchestrator_test.go`
*   **Key Dependencies:** `encoding/json`, `strings`, `fmt`
*   **Risks/Edge Cases:** LLM returning valid JSON without markdown, LLM returning markdown with extra newlines, extra whitespace breaking suffix trimming, legacy array format (in extraction), summarization aborting on a single unresolvable node.

## 📋 Micro-Step Checklist
- [x] Phase 1: Shared Robust JSON Parsing
  - [x] Step 1.A: Create the Parser Test Harness (Status: ✅ Implemented)
  - [x] Step 1.B: Implement the Parser Logic (Status: ✅ Implemented)
  - [x] Step 1.C: Verify Parser Logic (Status: ✅ Implemented)
- [x] Phase 2: Consolidate Parser Usage
  - [x] Step 2.A: Characterize Extractor & Enricher existing behavior (Status: ✅ Verified)
  - [x] Step 2.B: Refactor Extractor to use shared parser (Status: ✅ Implemented)
  - [x] Step 2.C: Refactor Enricher to use shared parser (Status: ✅ Implemented)
  - [x] Step 2.D: Verify integration (Status: ✅ Verified)
- [ ] Phase 3: Fail-Fast Orchestration
  - [ ] Step 3.A: Characterize Orchestrator Loop Behavior
  - [ ] Step 3.B: Implement Fail-Fast in Extraction Loop
  - [ ] Step 3.C: Implement Fail-Fast in Summarization Loop
  - [ ] Step 3.D: Verify Orchestrator Changes

## 📝 Step-by-Step Implementation Details

### Prerequisites
* Go 1.22+
* Access to `internal/rpg` package

#### Phase 1: Shared Robust JSON Parsing
1.  **Step 1.A (The Unit Test Harness):** Define the verification requirement for robust JSON parsing.
    *   *Target File:* `internal/rpg/llm_parser_test.go` (Create)
    *   *Test Cases to Write:* Create `TestParseLLMJSON(t *testing.T)` to cover:
        *   Standard JSON without markdown (e.g., `{"name": "test"}`).
        *   Standard Markdown block (` ```json\n{"name": "test"}\n``` `).
        *   Markdown block with leading/trailing newlines (` \n\n ```json\n{"name": "test"}\n``` \n\n `).
        *   Markdown block with extra backticks.
        *   Malformed JSON string, asserting it returns an error containing the original text context.
2.  **Step 1.B (The Implementation):** Implement the core parsing utility resilient to whitespace.
    *   *Target File:* `internal/rpg/llm_parser.go` (Create)
    *   *Exact Change:* Create a robust string stripping mechanism:
        ```go
        package rpg

        import (
            "encoding/json"
            "fmt"
            "strings"
        )

        // ParseLLMJSON aggressively strips markdown and whitespace before unmarshaling.
        func ParseLLMJSON(responseText string, target interface{}) error {
            origText := responseText
            
            // 1. Initial trim
            responseText = strings.TrimSpace(responseText)
            
            // 2. Trim optional prefix
            if strings.HasPrefix(responseText, "```json") {
                responseText = strings.TrimPrefix(responseText, "```json")
            } else if strings.HasPrefix(responseText, "```") {
                responseText = strings.TrimPrefix(responseText, "```")
            }
            
            // 3. Re-trim leading whitespace that might have followed the prefix
            responseText = strings.TrimSpace(responseText)
            
            // 4. Trim suffix (which must be exactly "```" at the end of the trimmed string)
            responseText = strings.TrimSuffix(responseText, "```")
            
            // 5. Final trim
            responseText = strings.TrimSpace(responseText)
            
            if err := json.Unmarshal([]byte(responseText), target); err != nil {
                return fmt.Errorf("failed to parse LLM response: %w. Raw: %s", err, origText)
            }
            return nil
        }
        ```
3.  **Step 1.C (The Verification):** Verify the parsing logic in isolation.
    *   *Action:* Run `go test ./internal/rpg -v -run TestParseLLMJSON`.
    *   *Success:* All scenarios pass successfully.

#### Phase 2: Consolidate Parser Usage
1.  **Step 2.A (The Verification Harness):** Ensure existing tests pass before refactoring.
    *   *Action:* Run `go test ./internal/rpg -v`
    *   *Success:* Baseline passes.
2.  **Step 2.B (The Extractor Refactor):** Replace inline string stripping in the Extractor.
    *   *Target File:* `internal/rpg/extractor.go`
    *   *Exact Change:* Inside `LLMFeatureExtractor.Extract`, replace the `strings.TrimPrefix` block and `json.Unmarshal` logic with:
        ```go
        var res extractorResponse
        if err := ParseLLMJSON(cand.Content.Parts[0].Text, &res); err != nil {
            // Legacy LLM fallback: array format
            var descriptors []string
            if err2 := ParseLLMJSON(cand.Content.Parts[0].Text, &descriptors); err2 == nil {
                return descriptors, false, nil
            }
            return nil, false, err
        }
        return res.Descriptors, res.IsVolatile, nil
        ```
3.  **Step 2.C (The Enricher Refactor):** Replace inline string stripping in the Summarizer.
    *   *Target File:* `internal/rpg/enrich.go`
    *   *Exact Change:* Inside `VertexSummarizer.Summarize`, replace the `strings.TrimPrefix` block and `json.Unmarshal` logic with:
        ```go
        var summary struct {
            Name        string `json:"name"`
            Description string `json:"description"`
        }
        if err := ParseLLMJSON(cand.Content.Parts[0].Text, &summary); err != nil {
            return "", "", err
        }
        return summary.Name, summary.Description, nil
        ```
4.  **Step 2.D (The Verification):** Ensure no regressions.
    *   *Action:* Run `go test ./internal/rpg -v`
    *   *Success:* No broken tests.

#### Phase 3: Fail-Fast Orchestration
1.  **Step 3.A (The Unit Test Harness):** Define testing requirement for the fail-fast logic in both loops.
    *   *Target File:* `internal/rpg/orchestrator_test.go`
    *   *Test Cases to Write:*
        *   `TestOrchestratorExtraction_ErrorThreshold_Aborts`: Assert that `RunExtraction` aborts with `"extraction aborted: too many consecutive errors"` after 5 consecutive mock extraction errors.
        *   `TestOrchestratorSummarization_ErrorThreshold_Aborts`: Assert that `RunSummarization` aborts similarly with `"summarization aborted: too many consecutive errors"`.
2.  **Step 3.B (The Implementation - Extraction):** Execute the fail-fast change in the Extraction loop.
    *   *Target File:* `internal/rpg/orchestrator.go`
    *   *Exact Change:* In `RunExtraction`:
        *   Declare `consecutiveErrors := 0` and `const maxConsecutiveErrors = 5` just before the `for {` outer loop.
        *   Replace the `if err != nil` block after `o.Extractor.Extract(code, name)` with:
            ```go
            if err != nil {
                log.Printf("Warning: failed to extract features for %s: %v", node.ID, err)
                _ = o.Provider.UpdateAtomicFeatures(node.ID, []string{"extraction_failed"}, false)
                consecutiveErrors++
                if consecutiveErrors >= maxConsecutiveErrors {
                    return fmt.Errorf("extraction aborted: too many consecutive errors (last error: %w)", err)
                }
                continue
            }
            consecutiveErrors = 0
            ```
3.  **Step 3.C (The Implementation - Summarization):** Execute the fail-fast change in the Summarization loop to tolerate single failures but abort on systemic issues.
    *   *Target File:* `internal/rpg/orchestrator.go`
    *   *Exact Change:* In `RunSummarization`:
        *   Declare `consecutiveErrors := 0` and `const maxConsecutiveErrors = 5` just before the `for {` outer loop.
        *   Replace the `if err != nil` block after `enricher.Enrich(f, memberFuncs, node.Label)` with:
            ```go
            if err != nil {
                log.Printf("Warning: failed to enrich %s: %v", node.ID, err)
                _ = o.Provider.UpdateFeatureSummary(node.ID, "summarization_failed", "Summarization failed due to LLM error")
                consecutiveErrors++
                if consecutiveErrors >= maxConsecutiveErrors {
                    return fmt.Errorf("summarization aborted: too many consecutive errors (last error: %w)", err)
                }
                continue
            }
            consecutiveErrors = 0
            ```
4.  **Step 3.D (The Verification):** Verify orchestration logic changes.
    *   *Action:* Run `go test ./internal/rpg -v -run TestOrchestrator`
    *   *Success:* All threshold and abort conditions act correctly.

### 🧪 Global Testing Strategy
*   **Unit Tests:** Pure string parsing logic will be tested in strict isolation using raw strings without Vertex AI network dependencies. Orchestrator loop state transitions (counter behavior and error returns) will be verified using mocks for `FeatureExtractor` and `Summarizer`.

## 🎯 Success Criteria
*   The `ParseLLMJSON` shared utility correctly identifies, unwraps, and parses JSON regardless of surrounding newlines or Markdown block syntax.
*   Both `LLMFeatureExtractor` and `VertexSummarizer` utilize `ParseLLMJSON` exclusively.
*   The `graphdb-skill` extraction pipeline logs an isolated failure as `"extraction_failed"` in the database (preventing infinite loops) and continues processing.
*   The `graphdb-skill` summarization pipeline logs an isolated failure as `"summarization_failed"` in the database and continues processing.
*   If either pipeline encounters 5 consecutive failures from the LLM/parser, the orchestrator instantly aborts the batch job, preventing widespread cascading data corruption.
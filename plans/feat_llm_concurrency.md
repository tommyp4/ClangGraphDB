# Feature Implementation Plan: feat_llm_concurrency

## 🔍 Analysis & Context
*   **Objective:** Parallelize LLM calls for feature extraction and summarization using a bounded goroutine pool to improve performance, controlled by a new CLI flag.
*   **Affected Files:**
    *   `internal/rpg/orchestrator_test.go`
    *   `internal/rpg/orchestrator.go`
    *   `cmd/graphdb/cmd_enrich.go`
*   **Key Dependencies:**
    *   `sync.WaitGroup` and semaphore channels for goroutine pooling.
    *   `rpg.Orchestrator` struct and methods (`RunExtraction`, `RunSummarization`).
*   **Risks/Edge Cases:**
    *   **Data Races in Tests:** `MockGraphProvider` implementations often write to shared state. Test concurrency must be carefully controlled with `sync.Mutex` or avoided by defaulting `LLMConcurrency` to 1.
    *   **Goroutine Leaks:** Failing to properly read from the semaphore channel or WaitGroup on early returns.
    *   **Early Returns on Error:** We must collect errors in an `errChan` and return the first encountered error after the batch completes.

## 📋 Micro-Step Checklist
- [x] Phase 1: Test Harness (Characterize Behavior)
  - [x] Step 1.A: Add concurrent test for extraction in `internal/rpg/orchestrator_test.go` (Status: ✅ Implemented)
  - [x] Step 1.B: Add concurrent test for summarization in `internal/rpg/orchestrator_test.go` (Status: ✅ Implemented)
- [x] Phase 2: Parallelize Orchestrator Execution
  - [x] Step 2.A: Add `LLMConcurrency` to `rpg.Orchestrator` struct (Status: ✅ Implemented)
  - [x] Step 2.B: Update `RunExtraction` with semaphore and WaitGroup (Status: ✅ Implemented)
  - [x] Step 2.C: Update `RunSummarization` with semaphore and WaitGroup (Status: ✅ Implemented)
- [x] Phase 3: CLI Configuration
  - [x] Step 3.A: Add `--llm-concurrency` flag in `cmd/graphdb/cmd_enrich.go` (Status: ✅ Implemented)
  - [x] Step 3.B: Wire the new flag to the `Orchestrator` instantiation (Status: ✅ Implemented)
- [x] Phase 4: Final Verification
  - [x] Step 4.A: Run the test suite with race detector (Status: ✅ Verified with `go test -race ./internal/rpg/...`)

## 📝 Step-by-Step Implementation Details

### Phase 1: Test Harness (Characterize Behavior)

1.  **Step 1.A (The Unit Test Harness - Extraction):** Define the verification requirement.
    *   *Target File:* `internal/rpg/orchestrator_test.go`
    *   *Test Cases to Write:* Add `TestOrchestrator_RunExtraction_Concurrent(t *testing.T)`.
    *   *Implementation Details:*
        *   Mock `GetUnextractedFunctionsFn` to return 5 mock `*graph.Node` instances on the first call, and `nil` on subsequent calls. Use a `sync.Mutex` to protect the call count.
        *   Mock `UpdateAtomicFeaturesFn` to increment an `updateCount` protected by a `sync.Mutex`.
        *   Initialize `&Orchestrator{LLMConcurrency: 5, ...}`.
        *   Assert that `updateCount == 5` and no race conditions occur.

2.  **Step 1.B (The Unit Test Harness - Summarization):** Define the verification requirement.
    *   *Target File:* `internal/rpg/orchestrator_test.go`
    *   *Test Cases to Write:* Add `TestOrchestrator_RunSummarization_Concurrent(t *testing.T)`.
    *   *Implementation Details:*
        *   Similar to 1.A, mock `GetUnnamedFeaturesFn` to return 5 nodes. Use a `sync.Mutex` to protect the call count.
        *   Mock `ExploreDomainFn` to return a dummy domain with 1 function.
        *   Mock `UpdateFeatureSummaryFn` to increment an `updateCount` protected by a `sync.Mutex`.
        *   Initialize `&Orchestrator{LLMConcurrency: 5, ...}`.
        *   Assert that `updateCount == 5` and no race conditions occur.

### Phase 2: Parallelize Orchestrator Execution

1.  **Step 2.A (Struct Update):** Expose concurrency controls.
    *   *Target File:* `internal/rpg/orchestrator.go`
    *   *Exact Change:* Add `LLMConcurrency int` to the `type Orchestrator struct`.

2.  **Step 2.B (Parallelize Extraction):** Introduce the goroutine pool.
    *   *Target File:* `internal/rpg/orchestrator.go`
    *   *Exact Change:* In `func (o *Orchestrator) RunExtraction(batchSize int) error`:
        *   Determine concurrency at the start: `concurrency := o.LLMConcurrency; if concurrency <= 0 { concurrency = 1 }`
        *   Inside the `for {` batch loop, before iterating over `nodes`, setup a wait group and error channel:
            ```go
            var wg sync.WaitGroup
            sem := make(chan struct{}, concurrency)
            errChan := make(chan error, len(nodes))
            ```
        *   Wrap the `for _, node := range nodes` body in a goroutine:
            ```go
            for _, node := range nodes {
                wg.Add(1)
                sem <- struct{}{}
                go func(n *graph.Node) {
                    defer wg.Done()
                    defer func() { <-sem }()
                    // ... original extraction logic using 'n' instead of 'node' ...
                    // If an error occurs (e.g., err != nil from Extractor.Extract):
                    // errChan <- fmt.Errorf(...)
                    // return // NOTE: replace all original 'continue' statements with 'return' inside this closure
                }(node)
            }
            ```
        *   Wait for completion and handle errors at the end of the batch:
            ```go
            wg.Wait()
            close(errChan)
            for err := range errChan {
                if err != nil {
                    return err // return first encountered error
                }
            }
            ```

3.  **Step 2.C (Parallelize Summarization):** Implement the same pattern.
    *   *Target File:* `internal/rpg/orchestrator.go`
    *   *Exact Change:* In `func (o *Orchestrator) RunSummarization(batchSize int, dir string) error`:
        *   Determine `concurrency := o.LLMConcurrency; if concurrency <= 0 { concurrency = 1 }`
        *   Inside the `for {` batch loop, setup `wg`, `sem`, and `errChan`.
        *   Note that `enricher := &Enricher{...}` is created once per batch. It is thread-safe and can be safely shared across workers.
        *   Wrap the `for _, node := range nodes` body in `go func(n *graph.Node)` just like Step 2.B.
        *   Replace all error returns with `errChan <- fmt.Errorf(...)` and `return`.
        *   Push errors to `errChan`, returning them after `wg.Wait()`.

### Phase 3: CLI Configuration

1.  **Step 3.A (Wire CLI Flag):** Expose concurrency to the user.
    *   *Target File:* `cmd/graphdb/cmd_enrich.go`
    *   *Exact Change:* In `func handleEnrichFeatures(args []string)`, near the other flags, add:
        ```go
        llmConcurrencyPtr := fs.Int("llm-concurrency", 5, "Number of concurrent LLM requests during extraction/summarization")
        ```
2.  **Step 3.B (Inject Flag):** Pass the concurrency value.
    *   *Target File:* `cmd/graphdb/cmd_enrich.go`
    *   *Exact Change:* When initializing `orchestrator := &rpg.Orchestrator{`, set `LLMConcurrency: *llmConcurrencyPtr,`.

### Phase 4: Final Verification

1.  **Step 4.A (Validation):** Prove tests pass with concurrency flags enabled.
    *   *Action:* Run `go test -race ./internal/rpg/...`.
    *   *Success:* All tests pass without data race warnings and demonstrate full coverage.

## 🧪 Global Testing Strategy
*   **Unit Tests:** Two new concurrent test harnesses (`TestOrchestrator_RunExtraction_Concurrent` and `TestOrchestrator_RunSummarization_Concurrent`) will explicitly verify thread-safety and pool logic. Existing tests implicitly verify sequential fallback logic (concurrency = 1).
*   **Integration Tests:** Ensure `go test -race` cleanly passes across the `internal/rpg` package to eliminate hidden mutex violations or memory corruption.

## 🎯 Success Criteria
*   Extraction and Summarization perform concurrent requests utilizing the `--llm-concurrency` limit.
*   Default concurrency is set to `5`.
*   Batch failure bubbles up correctly if any concurrent worker fails.
*   No deadlocks or leaked goroutines when early returns are triggered inside workers.
*   All tests, including new concurrent harnesses, pass cleanly with the `-race` detector.

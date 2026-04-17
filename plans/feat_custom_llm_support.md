# Feature Implementation Plan: Custom LLM Backend & Domain Context Support

## 🔍 Analysis & Context
*   **Objective:** Introduce support for custom LLM backends (like local Gemma models) by overriding BaseURLs and API Keys, improve the resiliency of JSON parsing against custom models that ignore schema rules, and introduce an application-level context pre-amble to improve feature summarization accuracy.
*   **Affected Files:**
    *   `internal/config/loader.go`
    *   `internal/embedding/vertex.go`
    *   `internal/rpg/extractor.go`
    *   `internal/rpg/enrich.go`
    *   `internal/rpg/llm_parser.go`
    *   `internal/rpg/llm_parser_test.go`
    *   `cmd/graphdb/setup_prod.go`
    *   `cmd/graphdb/setup_mock_mode.go`
    *   `cmd/graphdb/cmd_enrich.go`
*   **Key Dependencies:** `google.golang.org/genai`
*   **Risks/Edge Cases:** 
    *   Custom local LLMs may ignore `ResponseSchema` and inject conversational prefixes (e.g., "Sure, here is your JSON").
    *   Failing to correctly plumb the GenAI `HTTPOptions` into all clients (Embedder, Extractor, Summarizer) will lead to API routing failures.

## 📋 Micro-Step Checklist
- [x] Phase 1: Environment Configuration Expansion
  - [x] Step 1.A: Add backend settings to `config.Config`. (Status: ✅ Implemented)
- [x] Phase 2: GenAI Client Overrides
  - [x] Step 2.A: Refactor initialization in `vertex.go` (Embedder). (Status: ✅ Implemented)
  - [x] Step 2.B: Refactor initialization in `extractor.go` (Feature Extractor). (Status: ✅ Implemented)
  - [x] Step 2.C: Refactor initialization in `enrich.go` (Summarizer). (Status: ✅ Implemented)
  - [x] Step 2.D: Update setup helpers in `setup_prod.go`. (Status: ✅ Implemented)
- [x] Phase 3: Robust JSON Parsing
  - [x] Step 3.A: Add edge-case test cases to `llm_parser_test.go`. (Status: ✅ Implemented)
  - [x] Step 3.B: Re-implement `ParseLLMJSON` to scan for brace/bracket boundaries. (Status: ✅ Implemented)
- [x] Phase 4: Application Context Injection
  - [x] Step 4.A: Add `-app-context` flag to CLI and load content. (Status: ✅ Implemented)
  - [x] Step 4.B: Plumb `appContext` into prompt generation. (Status: ✅ Implemented)

## 📝 Step-by-Step Implementation Details

### Phase 1: Environment Configuration Expansion
1.  **Step 1.A (The Configuration Data Model):** Define new environment variable bindings for custom GenAI targets.
    *   *Target File:* `internal/config/loader.go`
    *   *Exact Change:* 
        *   Add `GenAIBackend`, `GenAIBaseURL`, `GenAIAPIKey`, `GenAIAPIVersion` as `string` fields to the `Config` struct.
        *   Update `LoadConfig()` to populate these fields using `os.Getenv("GENAI_BACKEND")` etc.

### Phase 2: GenAI Client Overrides
1.  **Step 2.A (The Embedder):** Enable the Embedder client to target custom backends.
    *   *Target File:* `internal/embedding/vertex.go`
    *   *Exact Change:* Update `NewVertexEmbedder` to accept a `config.Config` struct instead of string parameters. Modify the `&genai.ClientConfig{}` block to check `cfg.GenAIBackend`. If "geminiapi" or if `GenAIBaseURL` is non-empty, set `Backend: genai.BackendGeminiAPI` and populate `APIKey`, and conditionally add `HTTPOptions` (with `BaseURL` and `APIVersion`). Otherwise, default to `Backend: genai.BackendVertexAI`.
2.  **Step 2.B (The Extractor):** Enable the Extractor client to target custom backends.
    *   *Target File:* `internal/rpg/extractor.go`
    *   *Exact Change:* Update `NewLLMFeatureExtractor` and `LLMFeatureExtractor` struct to accept `config.Config` and an `appContext string`. Apply the same `genai.ClientConfig` logic as Step 2.A. Add `AppContext string` to the `LLMFeatureExtractor` struct.
3.  **Step 2.C (The Summarizer):** Enable the Summarizer client to target custom backends.
    *   *Target File:* `internal/rpg/enrich.go`
    *   *Exact Change:* Update `NewVertexSummarizer` and `VertexSummarizer` struct to accept `config.Config` and an `appContext string`. Apply the same `genai.ClientConfig` logic as Step 2.A. Add `AppContext string` to the `VertexSummarizer` struct.
4.  **Step 2.D (The Wiring):** Update the CLI constructors to propagate the Config object.
    *   *Target File:* `cmd/graphdb/setup_prod.go` & `cmd/graphdb/setup_mock_mode.go`
    *   *Exact Change:* Change `setupEmbedder`, `setupExtractor`, and `setupSummarizer` signatures to take `(cfg config.Config, appContext string)` and update all callers inside `cmd_enrich.go`, `cmd_query.go`, `cmd_serve.go`.

### Phase 3: Robust JSON Parsing
1.  **Step 3.A (The Unit Test Harness):** Define the verification requirement for messy JSON.
    *   *Target File:* `internal/rpg/llm_parser_test.go`
    *   *Test Cases to Write:* Add `{"name": "conversational prefix", responseText: "Sure, here is your JSON:\n{\"name\": \"test\"}\nHope this helps!", wantErr: false}` and `{"name": "markdown wrapper prefix", responseText: "Here is your JSON:\n```json\n{\"name\": \"test\"}\n```\n", wantErr: false}`.
2.  **Step 3.B (The Implementation):** Implement boundary scanning.
    *   *Target File:* `internal/rpg/llm_parser.go`
    *   *Exact Change:* Rewrite `ParseLLMJSON`. Use `strings.IndexByte(responseText, '{')` and `strings.IndexByte(responseText, '[')` to find the start index of the JSON payload. Find the closing boundary using `strings.LastIndexByte`. Slice the `responseText` using `responseText[startIdx:endIdx+1]` before calling `json.Unmarshal`.
3.  **Step 3.C (The Verification):** 
    *   *Action:* Run `make test`.
    *   *Success:* The `TestParseLLMJSON` passes successfully, correctly stripping out the conversational preambles.

### Phase 4: Application Context Injection
1.  **Step 4.A (The CLI Flag):** Parse and load the context file.
    *   *Target File:* `cmd/graphdb/cmd_enrich.go`
    *   *Exact Change:* Add `appContextPtr := fs.String("app-context", "", "Optional path to an OVERVIEW.md or context preamble file")`. Read the file if provided, or silently fall back to `os.ReadFile(filepath.Join(*dirPtr, "OVERVIEW.md"))` if the flag is empty but the default file exists. Pass this resulting `appContext` string down to `setupExtractor` and `setupSummarizer`.
2.  **Step 4.B (The Prompt Generation):** Inject context into the LLM prompts.
    *   *Target File:* `internal/rpg/extractor.go`
    *   *Exact Change:* Prepend the prompt with `fmt.Sprintf("APPLICATION CONTEXT:\n%s\n\n", e.AppContext)` if `e.AppContext` is not empty.
    *   *Target File:* `internal/rpg/enrich.go`
    *   *Exact Change:* Prepend the prompts in `Summarize` with `fmt.Sprintf("APPLICATION CONTEXT:\n%s\n\n", s.AppContext)` if `s.AppContext` is not empty.

### 🧪 Global Testing Strategy
*   **Unit Tests:** Verify `llm_parser_test.go` correctly slices arbitrary text wrappers surrounding valid JSON objects/arrays.
*   **Integration Tests:** Verify `enrich-features` successfully runs end-to-end when an `OVERVIEW.md` is present, confirming that the prompt structure remains valid.

## 🎯 Success Criteria
*   The Go SDK `ClientInitialization` correctly respects `GENAI_BASE_URL` when provided, configuring `HTTPOptions` internally.
*   Conversational prefixes and markdown wrappers surrounding JSON payloads are stripped unconditionally by `ParseLLMJSON`.
*   Feature enrichment (both `Extractor` and `Summarizer`) dynamically embeds application domain context into the prompts to improve classification accuracy.
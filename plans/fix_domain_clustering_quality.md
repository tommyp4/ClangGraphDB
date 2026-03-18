# Implementation Plan: Fix Domain Clustering Quality

## Todo Checklist
- [x] Step 1: Fix `line`/`start_line` property mismatch across codebase
- [x] Step 2: Enrich `NodeToText` with structural context (file path + function name) (Status: ✅ Implemented and verified with tests)
- [x] Step 3: Split Summarizer into Domain vs Feature prompts with DDD naming guidance (Status: ✅ Implemented and verified with tests)
- [x] Step 4: Improve extraction prompt to produce domain-friendly descriptors (Status: ✅ Implemented and verified with tests)

## Research & Root Cause Analysis

### The Problem
Running `build-all` on an example codebase with concepts like payment/settlement produced 6 domains: **async, create, delete, payment, get, settlement**. Only 2 (payment, settlement) represent real business domains. The others (async, create, delete, get) are verb-based clusters that group unrelated functions by operation type rather than business concern.

### Why This Matters (Feathers Methodology Context)
This tool exists to support legacy code analysis in the style of Michael Feathers' "Working Effectively with Legacy Code." The core workflows are:

- **Finding seams**: Places where you can insert tests or break dependencies between modules
- **Identifying pinch points**: Chokepoints between stable business logic and volatile external dependencies
- **Understanding what code does**: Grouping scattered code by business capability so practitioners can reason about impact
- **Refactoring planning**: Using `what-if` and `impact` queries to simulate extraction of bounded contexts

For all of these, domains must represent **business capabilities** (e.g. payment processing, settlement, user management) — the things you'd draw on a whiteboard as bounded contexts. A domain called "create" that mixes payment creation, user creation, and config creation is useless for seam detection because those functions have nothing in common architecturally.

### Root Cause Chain

The failure occurs across four stages of the pipeline, each compounding the previous:

#### Stage 1: Atomic Feature Extraction (`internal/rpg/extractor.go`)
The LLM extraction prompt asks for "Verb-Object" descriptors: `"validate email"`, `"create payment"`, `"delete record"`. This format places the **verb first**, which gives it dominant positional weight in downstream embedding.

Functions across completely different business domains share the same verbs:
- `create payment` (payment domain) + `create user` (user domain) + `create session` (auth domain)
- `delete record` (data domain) + `delete session` (auth domain) + `delete payment` (payment domain)

#### Stage 2: NodeToText (`internal/rpg/text.go`)
`NodeToText` joins atomic features into a comma-separated string: `"create payment, validate amount"`. It discards **all structural context**:
- No file path (`internal/payment/processor.go` would strongly signal "payment")
- No class/container name (`PaymentService` would signal "payment")
- No function name (when `atomic_features` exist, the raw name like `ProcessPayment` is discarded entirely at `text.go:12`)

This means the embedding model receives ONLY the verb-object phrases with zero domain context.

#### Stage 3: Embedding + K-Means Clustering (`internal/rpg/cluster_semantic.go`)
The embedding model encodes these short verb-first phrases. In semantic embedding space, `"create payment"` is closer to `"create user"` than to `"process payment"` because the shared verb `"create"` dominates the sparse, varied object nouns. K-Means then groups by vector similarity, producing verb-centric clusters.

#### Stage 4: Domain Naming (`internal/rpg/enrich.go` + `cluster_global.go`)
Two additional bugs degrade naming quality:

1. **Property mismatch bug**: `cluster_global.go:160` reads `n.Properties["line"]` for snippet loading, which happens to work. BUT `GetUnextractedFunctions` (`neo4j_batch.go:20`) queries `n.start_line IS NOT NULL` while all parsers emit `"line"` — this means extraction may silently skip all nodes or produce degraded results depending on the code path.

2. **Single prompt for two levels**: The same `Summarize()` prompt is used for both Domain naming (`cluster_global.go:53`) and Feature naming (`enrich.go:60` via `RunSummarization`). The prompt says "name this Feature" even when naming a Domain. There's no guidance to use business-domain language vs. capability-specific language, and no instruction to avoid verb-centric names.

### The `line` vs `start_line` Property Mismatch (Full Audit)

This is a pre-existing schema inconsistency that affects multiple code paths:

**Producers (parsers) — all currently emit `"line"` (WRONG — should be `"start_line"`):**
- `java.go:177, 233, 323` → `"line": int(c.Node.StartPoint().Row + 1)`
- `typescript.go:234, 277, 311` → `"line": int(c.Node.StartPoint().Row + 1)`
- `csharp.go:315` → `"line": c.Node.StartPoint().Row + 1`
- `cpp.go:164, 292` → `"line": int(c.Node.StartPoint().Row + 1)`
- `sql.go:82` → `"line": c.Node.StartPoint().Row + 1`
- `vbnet.go:89, 168` → `"line": lineNumber`
- `asp.go:59-60` → reads/modifies `n.Properties["line"]`

**Consumers — mixed expectations (after fix, canonical name is `start_line`):**
- `neo4j_batch.go:20` `GetUnextractedFunctions` → queries `n.start_line IS NOT NULL` (CORRECT)
- `neo4j_batch.go:154` `GetFunctionMetadata` → queries `n.line` (WRONG — change to `n.start_line`)
- `neo4j.go:623` `FetchSource` → queries `n.start_line` (CORRECT)
- `neo4j.go:660` `LocateUsage` → queries `source.start_line` (CORRECT)
- `orchestrator.go:39` `RunExtraction` → reads `node.Properties["start_line"]` (CORRECT)
- `cluster_global.go:160` `collectSnippets` → reads `n.Properties["line"]` (WRONG — change to `"start_line"`)
- `enrich.go:39` `Enricher.Enrich` → reads `fn.Properties["line"]` (WRONG — change to `"start_line"`)

**Impact**: The `GetUnextractedFunctions` query filters `WHERE n.start_line IS NOT NULL`. Since no parser emits `start_line`, this filter returns 0 nodes in a fresh database. The `RunExtraction` loop then reads `node.Properties["start_line"]` which is nil, causing `startLine == 0`, which triggers the fallback path writing `atomic_features: ["unknown"]` for every function. This means all functions get embedded as the text `"unknown"` — producing nearly identical vectors — which would cluster into a single blob that the LLM then names by whatever random signal it finds.

**Correction**: The canonical property names should be `start_line` and `end_line` (a logical, symmetric pair). The parsers (writers) must be updated to emit `start_line` instead of `line`. The consumers that already use `start_line` are correct by intent — only the parsers and the few consumers that read `"line"` need to change.

### Why File Paths Can't Be the Primary Signal

In legacy codebases (the target use case), code is often poorly organized. A `PaymentValidator` might live in `utils/helpers.go` or `legacy/MonolithService.java`. If file-path seeding were the primary clustering mechanism, it would reinforce the broken organization the tool is supposed to help escape.

The semantic clustering must remain the primary mechanism. File paths should be included as **one signal among several** in the embedding text. When the path is meaningful (well-organized code), it reinforces the domain signal. When it's noise (`utils/`, `common/`), it gets outweighed by the function name and atomic features — both of which typically carry the object/domain in their name (`ProcessPayment`, `validate payment amount`).

### DDD Naming Philosophy

Domain-Driven Design emphasizes **Ubiquitous Language**: names should use the language of the business domain, not technical implementation patterns. In DDD terms:

- **Domains** map to **Bounded Contexts** — they represent a coherent area of the business model with its own language. "Payment Processing" is a bounded context. "Create Operations" is not.
- **Features** map to **Aggregates** or **Domain Services** within a bounded context — they represent specific capabilities. "Payment Validation" is a capability within the "Payment Processing" context.

The naming prompts should explicitly guide the LLM toward this thinking:
- Domain names should answer: "What area of the business does this code serve?"
- Feature names should answer: "What specific capability does this group provide within its parent domain?"
- Both should avoid implementation verbs (create, delete, get, process) as the primary name.

---

## Implementation Plan

### Step 1: Fix `line`/`start_line` Property Mismatch

**Priority**: Critical — this may cause extraction to silently fail, making all downstream steps operate on garbage data.

**Canonical names**: `start_line` and `end_line` (a symmetric, self-documenting pair).

**Direction of fix**: Update the **writers** (parsers) to emit `start_line` instead of `line`. Then fix the 3 consumers that still read `"line"`. No legacy data migration — we rebuild from scratch.

#### Step 1.A: Fix all parsers to emit `start_line` instead of `line`
Each parser currently emits `"line": <StartPoint>`. Change to `"start_line": <StartPoint>`.

- **`internal/analysis/java.go`**: Lines 177, 233, 323 — change `"line":` to `"start_line":`
- **`internal/analysis/typescript.go`**: Lines 234, 277, 311 — change `"line":` to `"start_line":`
- **`internal/analysis/csharp.go`**: Line 315 — change `"line":` to `"start_line":`
- **`internal/analysis/cpp.go`**: Lines 164, 292 — change `"line":` to `"start_line":`
- **`internal/analysis/sql.go`**: Line 82 — change `"line":` to `"start_line":`
- **`internal/analysis/vbnet.go`**: Lines 89, 168 — change `"line":` to `"start_line":`
- **`internal/analysis/asp.go`**: Lines 59-60 — change `"line"` references to `"start_line"`

#### Step 1.B: Fix `GetFunctionMetadata` query (consumer reading `"line"`)
- **File**: `internal/query/neo4j_batch.go:154`
- **Change**: Replace `n.line as line` with `n.start_line as start_line` in the Cypher query.
- **Downstream**: Line 167 reads `neo4j.GetRecordValue[int64](record, "line")` — change to `"start_line"`.
- Line 187 maps to `Properties["line"]` — change to `Properties["start_line"]`.

#### Step 1.C: Fix `collectSnippets` property read (consumer reading `"line"`)
- **File**: `internal/rpg/cluster_global.go:160`
- **Change**: Replace `n.Properties["line"]` with `n.Properties["start_line"]`

#### Step 1.D: Fix `Enricher.Enrich` property read (consumer reading `"line"`)
- **File**: `internal/rpg/enrich.go:39`
- **Change**: Replace `fn.Properties["line"]` with `fn.Properties["start_line"]`

#### Step 1.E: Update all tests

These tests serve as the **schema contract** between writers and readers. Since Neo4j is schemaless, there is no DDL enforcing property names — these tests are the only validation that producer and consumer agree on the same "magic strings." Every test fixture must use the canonical property names (`start_line`, `end_line`, `file`, `name`, `atomic_features`, `embedding`, `is_volatile`, `id`) consistently.

**Test audit and required changes:**

##### `internal/analysis/asp_test.go`
- Lines 55, 61, 99, 105: Reads `node.Properties["line"]` — change to `"start_line"`.
- These tests validate parser output, so they must match the updated parser property name.

##### `internal/query/neo4j_batch_test.go`
- **`TestNeo4jBatchOperations`** (line 30-32): Setup Cypher creates nodes with BOTH `line` AND `start_line` as a workaround for the mismatch. After the fix, remove the redundant `line` property — only `start_line` and `end_line` should exist.
- Lines 189-190: Asserts `n.Properties["line"] != int64(21)` — change to assert `n.Properties["start_line"]`.
- **`TestGetUnextractedFunctions_SchemaMismatch`** (lines 258-319): This test was written to **document the bug** — it creates a node with `line` (no `start_line`) and expects `GetUnextractedFunctions` to fail. After the fix, this test is obsolete because parsers now emit `start_line`. **Rewrite it** as a positive test: create a node with `start_line`, call `GetUnextractedFunctions`, assert the node IS found and properties are correctly mapped. Remove the schema mismatch commentary.

##### `internal/query/neo4j_test.go`
- **`TestNeo4jProvider_FetchSource_SchemaMismatch`** (lines 396-449): Another bug-documenting test. Creates nodes with `line` and expects `FetchSource` to fail because it queries `start_line`. **Rewrite** as a positive test: create nodes with `start_line`, call `FetchSource`, assert it succeeds. Remove mismatch commentary.
- **`TestLocateUsage_SchemaMismatch`** (lines 452-510): Same pattern. **Rewrite** as a positive test with `start_line`.

##### `internal/rpg/orchestrator_test.go`
- **`TestOrchestratorExtraction`** (line 160): Mock returns `"start_line": 1` — this is already correct. No change needed.
- **`TestOrchestratorExtraction_SchemaMismatch`** (lines 298-351): Bug-documenting test. Returns `"line": 10` and expects fallback to `"unknown"`. **Rewrite** as a positive test: return `"start_line": 10`, assert extraction proceeds normally (NOT falling back to "unknown"). Remove the explicit `t.Errorf` that flags the schema mismatch. The test should now **pass** cleanly, confirming the orchestrator reads `start_line` and gets the correct value.

##### `internal/rpg/cluster_global_test.go`
- **`TestGlobalEmbeddingClusterer_Cluster`** (lines 13-15): Nodes use `"line": 10` — change to `"start_line": 10` (and similar for end_line which is already correct).
- **`TestGlobalEmbeddingClusterer_SnippetPropertyMismatch`** (lines 107-163): Bug-documenting test. Uses `"line": 10` and checks if loader is called. After the fix, `collectSnippets` reads `"start_line"`, so this test should use `"start_line": 10` and assert the loader IS called. **Rewrite** as a positive test. Remove the mismatch commentary at lines 108-109.

##### `internal/rpg/enrich_test.go`
- **`TestEnricher_Enrich`** (lines 76, 81): Uses `"line": 10` and `"line": 30` — change to `"start_line": 10` and `"start_line": 30`.
- **`TestEnricher_Enrich_NilEmbedder`** (line 122): Uses `"line": 10` — change to `"start_line": 10`.
- **`TestEnricher_Enrich_Float64Props`** (line 178): Uses `"line": float64(10)` — change to `"start_line": float64(10)`. This test validates that `getInt()` handles `float64` type coercion, which is still important.
- **`TestEnricher_Enrich_SchemaMismatch`** (lines 193-237): Bug-documenting test. Uses `"line": 10` and checks if loader is called. After fix, `Enricher.Enrich` reads `"start_line"`, so use `"start_line": 10` and assert loader IS called. **Rewrite** as a positive test. Remove mismatch commentary.
- **`MockSummarizer`** (lines 10-38): The mock's `Summarize` signature will need to be updated in Step 3 when we add the `level` parameter. For Step 1, no change needed.
- **`mockLoader`** (lines 51-59): No property names here — it accepts `(path, start, end int)`. No change needed.

##### `internal/rpg/builder_test.go`
- **`TestBuilder_Build`** (line 54): Uses `"file": "src/auth/login.go"` — correct, no change needed.
- **`TestBuilder_Build_SchemaMismatch_File`** (lines 135-190): Tests that `file` property is read correctly for LCA calculation. No `line`/`start_line` involved. **No change needed** for Step 1, but this test is a good example of the schema contract pattern.

##### `internal/rpg/naming_test.go`
- Uses `"name"` property throughout. No `line`/`start_line` involved. **No change needed** for Step 1.

**General principle for rewriting bug-documenting tests**: These tests were valuable for flagging the mismatch, but once fixed, they should become **positive schema contract tests** that verify the happy path works correctly with the canonical property names. They should not assert failure — they should assert success.

#### Step 1.F: Verify
- Run `go test ./internal/...`
- Confirm all tests pass with the unified `start_line`/`end_line` property names.
- Confirm no test contains `"line":` as a property name in fixtures (except `"end_line"` and `"start_line"`).

---

### Step 2: Enrich `NodeToText` with Structural Context

**Priority**: High — this is the core fix for verb-dominated clustering.

**Design principle**: Include multiple signals so the embedding model can triangulate domain membership. The function name is the most reliable signal (developers almost always name things by what they do, even when they put them in the wrong directory). The file path helps when it's meaningful and is ignored when it's not.

#### Step 2.A: Modify `NodeToText`
- **File**: `internal/rpg/text.go`
- **Change**: Restructure to include file path, function name, AND atomic features:

```go
func NodeToText(n graph.Node) string {
    var parts []string

    // File path provides structural domain context
    if file, ok := n.Properties["file"].(string); ok && file != "" {
        parts = append(parts, file)
    }

    // Function name provides behavioral context
    if name, ok := n.Properties["name"].(string); ok && name != "" {
        parts = append(parts, name)
    }

    // Atomic features provide semantic depth
    if af := getAtomicFeatures(n); len(af) > 0 {
        parts = append(parts, strings.Join(af, ", "))
    }

    if len(parts) > 0 {
        return strings.Join(parts, " | ")
    }

    return n.ID
}

func getAtomicFeatures(n graph.Node) []string {
    if af, ok := n.Properties["atomic_features"].([]string); ok && len(af) > 0 {
        return af
    }
    if afAny, ok := n.Properties["atomic_features"].([]interface{}); ok && len(afAny) > 0 {
        parts := make([]string, len(afAny))
        for j, v := range afAny {
            parts[j] = fmt.Sprintf("%v", v)
        }
        return parts
    }
    return nil
}
```

**Example outputs**:
- Well-organized code: `"internal/payment/processor.go | ProcessPayment | create payment, validate amount"` — three signals pointing to "payment"
- Legacy scattered code: `"utils/helpers.go | ProcessPayment | create payment, validate amount"` — path is noise, but name + descriptors still point to "payment"
- Minimal metadata: `"ProcessPayment"` — graceful fallback

#### Step 2.B: Update `NodeToText` tests
- **File**: `internal/rpg/text_test.go`
- **Change**: Update test cases to reflect the new format. Add test cases for:
  - Node with all three signals (file + name + atomic_features)
  - Node with file + name but no atomic_features
  - Node with only atomic_features (no file, no name)
  - Node with only ID (fallback)
  - Node with meaningless file path (`utils/common.go`) — verify it still produces reasonable text

#### Step 2.C: Verify downstream consumers
- **Consumers of `NodeToText`**:
  - `orchestrator.go:107` (`RunEmbedding`) — generates embeddings. No change needed; the richer text improves embedding quality automatically.
  - `cluster_semantic.go:45` (fallback embedding) — same, no change needed.
- **Impact on existing embeddings**: Any previously embedded nodes will have stale vectors. A full `build-all -clean` re-run is required after this change.

#### Step 2.D: Run tests
- `go test ./internal/rpg/...`

---

### Step 3: Split Summarizer for Domain vs Feature with DDD Naming

**Priority**: High — even with correct clustering, verb-centric naming makes domains unusable.

**Design**: Add a `level` parameter to `Summarize` so the prompt can differentiate between Domain and Feature naming. Use DDD-informed language in both prompts.

#### Step 3.A: Update Summarizer interface
- **File**: `internal/rpg/enrich.go:18-20`
- **Change**: Add a `level` parameter:

```go
type Summarizer interface {
    Summarize(snippets []string, level string) (string, string, error)
}
```

- `level` values: `"domain"` or `"feature"`

#### Step 3.B: Update `VertexSummarizer.Summarize`
- **File**: `internal/rpg/enrich.go:118+`
- **Change**: Use `level` to select the appropriate prompt.

**Domain prompt** (when `level == "domain"`):
```
You are a software architect performing Domain-Driven Design (DDD) analysis.
Below are representative code snippets from a cluster of related functions.

Your task is to identify the Bounded Context (business domain) these functions belong to.

1. Provide a concise name for this domain using the Ubiquitous Language of the business.
   - GOOD examples: "Payment Processing", "User Authentication", "Order Fulfillment", "Inventory Management"
   - BAD examples: "Create Operations", "Data Retrieval", "Async Handlers", "Delete Functions"
   - The name should answer: "What area of the business does this code serve?"
   - Use nouns and noun phrases that describe the business capability, NOT verbs that describe implementation operations.
   - If the code spans multiple concerns, pick the dominant business theme.

2. Provide a 2-3 sentence description of this domain's responsibility and boundaries.
   Describe what business problems it solves, not how it implements them.

Return JSON ONLY: {"name": "...", "description": "..."}
```

**Feature prompt** (when `level == "feature"`):
```
You are a software architect performing Domain-Driven Design (DDD) analysis.
Below are code snippets from a group of closely related functions within a larger domain.

Your task is to name the specific capability or service these functions provide.

1. Provide a concise name for this feature.
   - GOOD examples: "Payment Validation", "Session Token Management", "Invoice Generation", "Refund Processing"
   - BAD examples: "Helper Functions", "Utility Methods", "Data Access", "CRUD Operations"
   - The name should answer: "What specific capability does this group provide?"
   - Be more specific than the parent domain, but still use business language.

2. Provide a 1-2 sentence description of what this feature does.

Return JSON ONLY: {"name": "...", "description": "..."}
```

#### Step 3.C: Update all `Summarize` call sites
- **`cluster_global.go:53`** (domain naming): Change to `c.Summarizer.Summarize(snippets, "domain")`
- **`enrich.go:60`** (feature/domain naming in `Enricher.Enrich`): This is called from `RunSummarization` which processes both unnamed Features and Domains. Need to determine the level from the node's label.

#### Step 3.D: Pass node label context to `RunSummarization`
- **File**: `internal/rpg/orchestrator.go` in `RunSummarization`
- **Change**: When calling `enricher.Enrich`, pass the node's label so it can select the right prompt level. The `GetUnnamedFeatures` query should return the node label (Domain vs Feature).
- Check `GetUnnamedFeatures` query to see if it returns label info. If not, add it.

#### Step 3.E: Update `Enricher.Enrich` to accept and pass level
- **File**: `internal/rpg/enrich.go:28`
- **Change**: Add `level string` parameter to `Enrich`, pass through to `e.Client.Summarize(snippets, level)`.

#### Step 3.F: Update mocks and tests
The `Summarize` signature change ripples through every mock and test that uses it.

- **`internal/rpg/enrich_test.go`**:
  - `MockSummarizer` (line 10-38): Update `SummarizeFunc` field type from `func(snippets []string)` to `func(snippets []string, level string)`. Update `Summarize` method signature to accept `level string`. Pass `level` through to `SummarizeFunc`.
  - `TestGlobalEmbeddingClusterer_SnippetPropertyMismatch` in `cluster_global_test.go` (line 142): The `SummarizeFunc` mock uses `func(snippets []string)` — update signature.
  - `TestGlobalEmbeddingClusterer_Cluster` in `cluster_global_test.go` (line 40): Same — update mock signature.
  - All `TestEnricher_*` tests: `Enricher.Enrich` now takes `level` — update call sites.

- **`cmd/graphdb/mocks.go`** (line 24-26): `MockSummarizer.Summarize` — update signature to accept `level string`.

- **`test/e2e/campaign_3_7_integration_test.go`** (line 14-21):
  - `MockSummarizer.Summarize` — update signature to accept `level string`.
  - Line 40: Node uses `"line": 3` — change to `"start_line": 3` (from Step 1).
  - Line 57: `enricher.Enrich(feature, []graph.Node{fn})` — update to pass `level` parameter.

#### Step 3.G: Run tests
- `go test ./internal/rpg/... ./cmd/graphdb/...`

---

### Step 4: Improve Extraction Prompt for Domain-Friendly Descriptors

**Priority**: Medium — this improves embedding quality at the source, complementing Step 2.

**Design**: The current prompt asks for "Verb-Object" descriptors (`"create payment"`). This biases embeddings toward verb clustering. We should ask for descriptors that emphasize the **domain object** while still capturing the action.

#### Step 4.A: Update extraction prompt
- **File**: `internal/rpg/extractor.go:58-77`
- **Change**: Modify the prompt to request **Object-Action** descriptors and include explicit guidance:

```
You are analyzing source code to extract atomic feature descriptors.

For the function below, generate descriptors that capture what business entity
or concept this function operates on and what it does.

Format each descriptor as "object-action" (noun first, then verb):
  GOOD: "payment validation", "user authentication", "order fulfillment", "session cleanup"
  BAD:  "validate payment", "create user", "process data", "handle request"

The object/noun should reflect the business domain concept, not technical implementation.
  GOOD: "invoice generation" (business concept)
  BAD:  "string parsing" (implementation detail)

Additionally, evaluate if the function is 'volatile' [... rest of volatility instructions unchanged ...]

Rules:
- Each descriptor should be 2-4 words: a domain noun followed by the action
- Generate 1-5 descriptors depending on function complexity
- Focus on the business purpose, not implementation mechanics
- If the function is purely technical (e.g., a utility), use the most specific
  domain noun available (e.g., "configuration loading" not "file reading")

Return ONLY a JSON object with this schema:
{
  "descriptors": ["descriptor1", "descriptor2"],
  "is_volatile": true
}
```

#### Step 4.B: Verify backward compatibility
- The change is prompt-only; the response schema (`descriptors` + `is_volatile`) is unchanged.
- Existing `atomic_features` in the DB will be stale after this change. A full `build-all -clean` is required.
- `NodeToText` and all downstream consumers don't care about descriptor format — they join with commas regardless.

#### Step 4.C: Update mock descriptors in tests
- **File**: `internal/rpg/extractor.go:122` (`MockFeatureExtractor`)
- **Change**: Update default mock descriptors from `"process data", "validate input"` to `"data processing", "input validation"` to match the new format.

#### Step 4.D: Run tests
- `go test ./internal/rpg/...`

---

## Testing Strategy

### Unit Tests
- All existing tests pass after each step (run `go test ./...` after each step).
- New test cases in `text_test.go` verify the enriched `NodeToText` format.
- Mock summarizer tests verify correct prompt selection by level.

### Integration Verification
After all 4 steps, run `build-all -clean` on a test codebase and verify:
1. **Extraction**: `atomic_features` are populated (not `["unknown"]`) — confirms Step 1 fix.
2. **Embedding text**: Spot-check that embedded text includes file path + name + descriptors.
3. **Domain names**: Verify domains represent business concepts, not operation verbs.
4. **Feature names**: Verify features are more specific than their parent domains.
5. **Hierarchy**: Verify Feature nodes are nested under semantically appropriate Domain nodes.

### Regression Checks
- `semantic-seams` query still works (uses same embeddings, now with richer text).
- `search-similar` query still works (richer embedding text should improve search quality).
- `explore-domain` returns coherent business groupings.

## Migration Notes
- Existing graphs built before this change will have stale `atomic_features` and `embedding` values.
- A full `build-all -clean` is required to regenerate everything from scratch.
- There is no in-place migration path — the extraction prompt change means all atomic features must be re-extracted.

## Success Criteria
- Domains represent business capabilities (e.g., "Payment Processing", "Settlement Management"), not operation types (e.g., "Create", "Delete", "Get").
- Features within each domain are specific capabilities (e.g., "Payment Validation", "Refund Processing").
- The `line`/`start_line` mismatch is fully resolved — one canonical property name across the entire codebase.
- All tests pass. No regressions in structural queries.

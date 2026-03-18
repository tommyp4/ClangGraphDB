# Plan: Fix Missing Content via Disk-Based Retrieval

**Status:** Active
**Trigger:** User feedback: "Don't store content in DB, read from disk."
**Objective:** Resolve "Unknown Feature" nodes by enabling the `Enricher` to read function source code directly from disk on-demand, rather than storing it in the Graph/JSONL.

## 1. Architecture Update
*   **Ingestion (Parsers):** Will capture `start_line` and `end_line` (and optionally byte offsets) in `graph.Node.Properties`.
*   **Enrichment (`Enricher`):** Will use `internal/tools/snippet.SliceFile` to read the function body when generating prompts for the LLM.

## 2. Implementation Steps

### Step 1: Update Language Parsers
Modify `internal/analysis/` parsers (`.ts`, `.cs`, `.java`, `.cpp`, `.vb`) to extract `end_line`.
*   Current: `line` (int) -> Start Line.
*   New: `end_line` (int).
*   *Note:* Tree-sitter nodes have `StartPoint().Row` and `EndPoint().Row`. (0-based, convert to 1-based).
*   **CRITICAL (VB.NET):** Remove `"content": string(content)` from `internal/analysis/vbnet.go`. This is the only parser populating it today, and it must be stopped to ensure consistency and reduce JSONL size.

### Step 2: Update Enricher Logic
Modify `internal/rpg/enrich.go`.
*   **Import:** `graphdb/internal/tools/snippet`.
*   **Definition:** Define `type SourceLoader func(path string, start, end int) (string, error)`.
*   **Field:** Add `Loader SourceLoader` to `Enricher` struct.
*   **Default:** In constructor (or initialization), set `Loader = snippet.SliceFile`.
*   **Usage:** In `Enrich(feature, functions)`:
    *   Iterate functions.
    *   Extract `file`, `line` (start), `end_line` from properties.
    *   Call `e.Loader(file, line, end_line)`.
    *   Use this content for the LLM prompt.

### Step 3: Global Cleanup
*   Remove any code in `cmd/graphdb/main.go` that attempts to read `fn.Properties["content"]`.
*   Replace these reads with `snippet.SliceFile(file, start, end)`.
*   Verify `nodes.jsonl` no longer contains massive code blocks.

## 3. Verification
*   **Unit Test:** Mock the file system or use a real fixture to ensure `Enricher` reads the file.
*   **E2E:** Run `build-all` on a sample project. Check logs/graph to ensure Features have names/descriptions (not "Unknown Feature").

## 4. Execution Order
1.  **Refactor Parsers** (Add `end_line`).
2.  **Refactor Enricher** (Use `snippet.SliceFile`).

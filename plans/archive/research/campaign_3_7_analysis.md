# Research Report: Campaign 3.7 System Architecture & Gap Analysis

## 1. Executive Summary
The goal of Campaign 3.7 is "RPG Realization" and remediation. Current analysis of the `graphdb-skill` codebase reveals a **critical disconnection** in the data pipeline that prevents the RPG (Repository Planning Graph) from being correctly enriched with semantic features. Specifically, the source code content of functions is never extracted during ingestion, causing the Feature Enrichment phase to fail silently or produce empty summaries.

## 2. System Architecture Map (Campaign 3.7)

The system operates in three distinct phases: **Ingest**, **Enrich**, and **Import**.

### Phase 1: Ingest (`cmd/graphdb/main.go` -> `ingest/walker.go`)
*   **Walker:** Traverses the file system, respecting `.gitignore`.
*   **WorkerPool:** Distributes files to workers.
*   **Parsers (`internal/analysis/`):** Parse files using `tree-sitter`.
    *   *Output:* Graph Nodes (Files, Functions, Classes) and Edges (DEFINES, CALLS, etc.).
    *   *Defect:* Parsers extract `name`, `file`, and `start_line`, but **omit `end_line` and function body `content`**.
*   **Emitter:** Writes raw nodes/edges to `graph.jsonl`.

### Phase 2: Enrich (`cmd/graphdb/main.go` -> `rpg/enrich.go`)
*   **Loader:** Reads `graph.jsonl`.
*   **Feature Extractor (`rpg/extractor.go`):** Intended to use LLMs to extract "atomic features" (verb-object pairs) from function code.
    *   *Failure Point:* It attempts to read `fn.Properties["content"]`. Since this property is empty (never populated in Phase 1), it returns `nil`/empty.
*   **Builder (`rpg/builder.go`):** Clusters functions into Domains and Features based on embeddings and directory structure.
*   **Enricher (`rpg/enrich.go`):** Generates summaries for features.
    *   *Failure Point:* It also relies on `content` to generate summaries. Without content, it returns "Unknown Feature".

### Phase 3: Import (`cmd/graphdb/main.go` -> `loader/neo4j_loader.go`)
*   **Neo4jLoader:** Loads the enriched `rpg.jsonl` into the graph database.

## 3. Deep Dive: Language Parsers & Property Handling
The following parsers in `internal/analysis/` were examined:

*   **Interface:** `LanguageParser` (`Parse(path, content) -> ([]Node, []Edge, error)`)
*   **Implementations:**
    *   `csharp.go`: Extracts Classes, Methods, Fields. Properties: `name`, `file`, `line`.
    *   `java.go`: Extracts Classes, Interfaces, Methods. Properties: `name`, `file`, `line`.
    *   `typescript.go`: Extracts Classes, Functions. Properties: `name`, `file`, `line`.
    *   *(Others likely similar: cpp, sql, vbnet, asp)*

**Critical Gap:** No parser populates `Properties["end_line"]` or `Properties["content"]`.
*   `StartPoint()` is called on tree-sitter nodes, but `EndPoint()` is ignored.
*   The raw content byte slice is available during parsing, but not sliced and attached to nodes.

## 4. Integration Analysis
### `internal/rpg/enrich.go`
*   **Dependency:** Strictly depends on `Properties["content"]` being a non-empty string.
*   **Status:** **BROKEN**. Functional logic exists but is starved of data.

### `internal/tools/snippet/snippet.go`
*   **Capability:** Can slice files given `startLine` and `endLine`.
*   **Integration:** Currently **UNUSED** in the ingestion pipeline.
*   **Potential:** This tool is the key to solving the content extraction gap, provided we have `endLine`.

## 5. Recommendations for Architect

### 1. Refactor Parsers to Extract `EndLine`
Modify the `LanguageParser` implementations to include the end line number in node properties.
*   *Action:* Update `csharp.go`, `java.go`, `typescript.go`, etc. to capture `node.EndPoint().Row + 1`.

### 2. Implement Content Extraction in Worker
Update `internal/ingest/worker.go` to populate the `content` property.
*   *Option A (Preferred):* Use `snippet.SliceFile` using the `file`, `start_line`, and `end_line` (once available).
*   *Option B (Optimization):* Extract content directly within the Parsers (since they already hold the file buffer) and attach it to the Node immediately. This avoids re-reading the file.

### 3. Verify Feature Extractor
Once `content` is flowing, verify `rpg/extractor.go` correctly calls Vertex AI and that `rpg/enrich.go` generates valid summaries.

### 4. Seam Identification
The `internal/ingest` package is the primary "Cut Point". Isolating the `WorkerPool` logic to support content extraction will fix the downstream RPG generation without requiring changes to the RPG logic itself.

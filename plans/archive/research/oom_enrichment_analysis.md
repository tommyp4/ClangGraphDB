# Research Report: Resolving Out of Memory (OOM) Exceptions in GraphDB Enrichment

**Status:** Completed
**Date:** February 25, 2026
**Target:** `cmd/graphdb/main.go` and `internal/rpg/enrich.go`
**Context:** The `graphdb` skill crashes with an Out of Memory (OOM) exception during the "[Phase 2/3] Enriching Features" step when analyzing a 31,000-file C++ application.

## 1. Executive Summary

A deep analysis of the Go `graphdb` ingest pipeline reveals that the application is built on a **purely in-memory architecture** that scales linearly with the size of the repository. For a 31,000-file application (translating to ~1.5 to 3 million functions), the memory required to hold the structural graph, the raw source code strings, and the precomputed embedding vectors simultaneously exceeds the physical RAM of most workstations (estimated 20-30+ GB).

The OOM crash specifically triggers during the transition between the **Clustering Phase** and the **Enrichment Phase**, exacerbated by file I/O operations and string concatenations performed to build LLM prompts.

To solve this, the architecture must transition from an **In-Memory Batch Pipeline** to a **Disk-Backed Streaming Pipeline**, utilizing Neo4j as the active working memory for intermediate states.

## 2. Root Cause Analysis (Memory Profiling)

The memory bloat is concentrated in three specific areas during the `build-all` sequence:

### 2.1 The `functions` Slice (The "Content" Trap)
In `cmd/graphdb/main.go`, the `loadFunctions()` method reads `graph.jsonl` and loads every single function into a massive slice: `[]graph.Node`.
*   **The Issue:** Each node is backed by a `map[string]interface{}` (unmarshaled JSON). The `content` property of these nodes holds the raw source code string of the function.
*   **The Cost:** For 3 million functions, holding the raw source strings plus the Go map overhead consumes an estimated **5 GB to 15 GB of RAM**.

### 2.2 The `precomputed` Embeddings Map
Before K-Means clustering begins, the system generates embeddings for all functions and stores them in memory: `precomputed := make(map[string][]float32)`.
*   **The Issue:** A 768-dimensional `float32` vector is approximately 3 KB.
*   **The Cost:** Storing 3 million of these vectors in a map requires **~9.5 GB of RAM**.

### 2.3 The Enrichment Allocation Spike
Once `builder.Build()` finishes clustering, the `Enriching features` phase begins.
*   **The Issue:** The `Enricher` (in `internal/rpg/enrich.go`) uses `snippet.SliceFile()` to lazy-load source code from disk to provide context to the LLM.
*   **The Cost:** Because the baseline memory is already near the physical limits (15-25 GB), the rapid allocation of temporary strings during file scanning (`bufio.Scanner.Text()`) and prompt concatenation pushes the Go heap over the edge, causing the OS to kill the process via OOM.

## 3. Short-Term Remediation (Eager Memory Reclaim)

Without changing the fundamental architecture, the memory footprint can be cut significantly by aggressively clearing data that is no longer needed in the pipeline.

1.  **Drop the `content` Property Early:** The raw source code is only needed during `extractor.Extract`. By actively deleting the `content` key from the node properties (`delete(fn.Properties, "content")`) immediately after extraction, gigabytes of string memory become eligible for Garbage Collection *before* embedding begins.
2.  **Drop the `precomputed` Map:** The embeddings are only required for K-Means clustering (`builder.Build()`). Once the feature hierarchy is constructed, this 10 GB map is dead weight. Setting `precomputed = nil` and forcing a manual garbage collection (`runtime.GC()`) right before `enrichAll()` would free up massive amounts of RAM for the final phase.
3.  **Struct Packing:** Refactor `loadFunctions` to unmarshal into a strictly typed Go struct instead of a `map[string]interface{}` to reduce pointer overhead and heap fragmentation.

## 4. Long-Term Architectural Solution (The "Neo4j-Centric" Pipeline)

To handle massive enterprise applications reliably, the pipeline must shift from an in-memory batch script to an out-of-core streaming architecture. The most efficient approach is to use the GraphDB (Neo4j) as the stateful engine for intermediate steps.

### 4.1 The "Pointer to Blob Storage" Pattern
Crucially, **source code should never be stored in Neo4j or intermediate JSONL files.** The application correctly implements `snippet.SliceFile` in the later stages. This pattern must be enforced throughout the entire pipeline:
*   Neo4j stores only topology and metadata (`id`, `name`, `file`, `line`, `end_line`, `atomic_features`, `embedding`).
*   The local file system acts as the immutable "Blob Storage."
*   Go workers read from disk on-demand based on the `line` pointers.

### 4.2 The Streaming Workflow

The pipeline transforms into an iterative, database-driven workflow, solving both memory and reliability issues:

1.  **Phase 1 (Ingest - Topology Only):** Tree-sitter parses the code, extracts relationships and bounding lines (`file`, `line`, `end_line`), actively discards the `content` string, and streams this lightweight graph directly into Neo4j.
2.  **Phase 2 (Extract - Resumable):** Go queries Neo4j for batches of unenriched functions (e.g., `LIMIT 500`). It uses `snippet.SliceFile` to read the context, calls the LLM, and writes the short `atomic_features` strings back to Neo4j. *Memory is bounded to the batch size (O(1)).*
3.  **Phase 3 (Embed - Resumable):** Go queries Neo4j for batches of functions missing vectors, calls the Embedding API, and writes `embedding` properties back to Neo4j.
4.  **Phase 4 (Cluster):** Go queries Neo4j for *only* the vectors (`id`, `embedding`). This loads ~100MB of floats into RAM instead of 20GB of node data. Go runs K-Means incredibly fast in memory and writes the hierarchical `Feature` nodes back to Neo4j.
5.  **Phase 5 (Summarize - Resumable):** Go queries Neo4j for unnamed `Feature` nodes, lazy-loads child function context via `snippet.SliceFile`, calls the LLM, and updates the `Feature` node.

### 4.3 Benefits of the Neo4j-Centric Architecture
*   **O(1) Application Memory:** The Go process memory footprint drops to a flat, constant size (just the size of the current batch). A 10-million-file codebase could be processed on an 8GB RAM machine.
*   **Checkpointing & Fault Tolerance:** If the Vertex AI API times out or rate-limits after 5 hours of processing, the entire in-memory state is currently lost. By using Neo4j as the intermediate store, the process becomes a robust, resumable worker queue. If it crashes, it picks up exactly where it left off on restart.
*   **Richer LLM Context:** With data already in Neo4j during the final Enrichment phase, the LLM prompt generation can use graph queries to provide *architectural context* (e.g., "What global variables do these functions share?"), drastically improving the semantic accuracy of the generated Feature names.

## 5. Conclusion

The OOM exception is a symptom of an architecture that treats massive datasets as in-memory arrays. Adopting the "Pointer to Blob Storage" pattern and utilizing Neo4j for intermediate state tracking will not only resolve the memory limitations but also transform the tool into a highly resilient, enterprise-grade analysis platform.
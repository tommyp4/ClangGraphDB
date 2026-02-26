---
name: graphdb
description: Expert in analyzing project architecture using a Neo4j Code Property Graph (CPG) enhanced with Vector Search. Answers questions about dependencies, seams, testing contexts, implicit links, and risks.
---

# Graph Database Skill (Go-Powered)

You are an expert in analyzing the project's architecture using a high-performance Code Property Graph (CPG) built with Go and Neo4j.
Your goal is to answer questions about dependencies, seams, testing contexts, and architectural risks using both structural analysis and the RPG (Repository Planning Graph) Intent Layer.

## Tool Usage
You will use the `graphdb` Go binary directly. Always execute commands from the **project root directory**.

**Binary Location Selection:**
*   **Linux/macOS:** `.gemini/skills/graphdb/scripts/graphdb`
*   **Windows:** `.gemini/skills/graphdb/scripts/graphdb-win.exe`

**Variable Definition:**
Define `${graphdb_bin}` as the path to the binary appropriate for the current operating system.

**Base Command:** `${graphdb_bin} <command> [options]`

## Setup & Infrastructure

### Installation
The skill relies on a pre-compiled Go binary (`${graphdb_bin}`).
If it does not exist, build it from the project root: `make build` (Linux/macOS) or use the cross-compilation script for Windows.

### Environment Variables
The tool automatically inherits the following environment variables. Assume they are already configured correctly via the `.env` file or host system. 
*   `NEO4J_URI`, `NEO4J_USER`, `NEO4J_PASSWORD` (Required for `import` and `query`)
*   `GOOGLE_CLOUD_PROJECT` (Required for Vertex AI embeddings)
*   `GOOGLE_CLOUD_LOCATION` (Default: `us-central1`)

**CRITICAL RULES FOR CREDENTIALS:**
1. You must **NEVER** explicitly set, export, or pass environment variables (like `NEO4J_PASSWORD=...`) in your bash commands. 
2. You must rely purely on the Go binary's internal `.env` loading. 
3. If a command fails due to an `Unauthorized` or authentication error, **STOP**. Do not try to guess or brute-force the password. Report the failure directly to the user and state that the credentials in their environment or `.env` file appear to be invalid or missing.

## Workflows

### 1. The "One-Shot" Build (Recommended)
To rebuild the entire graph from scratch (Ingest -> Import -> Enrich), use the `build-all` command. This handles all phases sequentially and ensures the database is synchronized with the latest code state.
```bash
${graphdb_bin} build-all -dir .
```
*   *Options:*
    *   `-clean`: Wipe the database before importing (default: true).

### 2. Manual Pipeline
If you need granular control over each step, follow this sequence:

**Step 0: Check Sync Status**
1. Get local commit: `git rev-parse HEAD`
2. Get graph commit: `${graphdb_bin} query -type status`
3. **Decision:** If the commit hashes match, you can **skip** the ingestion pipeline and proceed directly to "Analysis & Querying".

**Step 1: Ingest (Parse & Generate Graph):**
Scans code and generates structural graph JSONL files. We output nodes and edges separately to avoid double-parsing penalties during the import phase.
```bash
${graphdb_bin} ingest -dir . -nodes nodes.jsonl -edges edges.jsonl
```
*   *Options:* `-workers` (concurrency), `-file-list` (specific files).

**Step 2: Import (Load Structural Graph to Neo4j):**
Loads the structural graph into the active Neo4j database. This must be done **before** enrichment in the new streaming pipeline. Using separate nodes and edges files prevents a massive CPU penalty from scanning a combined file multiple times.
```bash
${graphdb_bin} import -nodes nodes.jsonl -edges edges.jsonl -clean
```
*   *Options:* `-clean` (wipe DB first), `-batch-size`.

**Step 3: Enrich (Build Intent Layer - In-Database):**
Performs **Global Semantic Clustering** directly against the live database. Identifies latent functional domains, extracts features, and generates summaries. Memory usage is bounded by batch sizes.
```bash
${graphdb_bin} enrich-features -dir .
```
*   *Options:*
    *   `-batch-size`: Number of nodes to process per LLM/Batch request (default: 20).
    *   `-embed-batch-size`: Batch size for embedding generation (default: 100).

### 3. Analysis & Querying
The primary way to interact with the graph is via the `query` command.

**Base Syntax:**
```bash
${graphdb_bin} query -type <type> -target "<search_term>" [options]
```

#### Supported Languages & FQN Formats
Structural queries require function names in their fully qualified format:

*   **C# / .NET / VB.NET:** `Namespace.Class.Method`
*   **C / C++:** `Namespace::Class::Method` (or `Class::Method`)
*   **Java:** `Package.Class.Method`
*   **TypeScript:** `FilePath:Class.Method` or `FilePath:Function` (e.g., `src/app.ts:MyClass.myMethod`)
*   **SQL:** `Schema.ObjectName` or `ObjectName` (no file path)

#### Query Types Reference

| Type | Description | Target | Options |
| :--- | :--- | :--- | :--- |
| `search-features` | **Intent Search.** Find features/concepts using vector search. | Natural language query | `-limit` |
| `search-similar` | **Code Search.** Find functions semantically similar to a query. | Natural language or code snippet | `-limit` |
| `neighbors` / `test-context` | **Dependency Analysis.** Find immediate callers and callees. | Function Name (exact) | `-depth` |
| `hybrid-context` | **Combined.** Structural neighbors + semantic similarities. Great for refactoring. | Function Name | `-depth`, `-limit` |
| `impact` | **Risk Analysis.** What other parts of the system behave differently if I change this? | Function Name | `-depth` |
| `globals` | **State Analysis.** Find global variables used by a function. | Function Name | |
| `seams` | **Architecture.** Identify testing seams in a module. | (Ignored) | `-module <regex>` |
| `locate-usage` | **Trace.** Find path/usage between two functions. | Function 1 | `-target2 <Function 2>` |
| `fetch-source` | **Read.** Fetch the source code of a function by ID/Name. | Function Name | |
| `explore-domain` | **Discovery.** Explore the domain model around a concept. | Concept/Entity Name | |
| `traverse` | **Raw Traversal.** Explore graph relationships directly. | Node ID / Name | `-edge-types`, `-direction`, `-depth` |
| `status` | **Verification.** Check the git commit hash stored in the graph. | (None) | |

## Operational Guidelines
*   **Output Parsing:** The tool returns JSON. Parse it and present a concise summary (bullet points, mermaid diagrams, or tables).
*   **Exact Names:** Structural queries (`neighbors`, `impact`) require exact function names in their fully qualified format (e.g., `Namespace.Class.Method`, `src/app.ts:Class.Method`). 
    *   **CRITICAL RULE:** If you only have a partial or unqualified name from the user, **DO NOT use `grep` or text search** to find the fully qualified name. Instead, you MUST use `search-similar` (semantic search) first to locate the exact node `ID`/`Name` before running structural queries.
*   **Context:** Always mention the source file and line number when discussing a function.
*   **Missing Data:** If a query returns empty, verify the spelling of the function/module name or try a semantic search.

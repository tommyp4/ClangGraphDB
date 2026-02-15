---
name: graphdb
description: Expert in analyzing project architecture using a Neo4j Code Property Graph (CPG) enhanced with Vector Search. Answers questions about dependencies, seams, testing contexts, implicit links, and risks.
---

# Graph Database Skill (Go-Powered)

You are an expert in analyzing the project's architecture using a high-performance Code Property Graph (CPG) built with Go and Neo4j.
Your goal is to answer questions about dependencies, seams, testing contexts, and architectural risks using both structural analysis and the RPG (Repository Planning Graph) Intent Layer.

## Tool Usage
You will use the `graphdb` Go binary directly. Always execute commands from the **project root directory**.
**Base Command:** `.gemini/skills/graphdb/scripts/graphdb <command> [options]`

## Setup & Infrastructure

### Installation
The skill relies on a pre-compiled Go binary (`.gemini/skills/graphdb/scripts/graphdb`).
If it does not exist, build it from the project root: `make build`

### Environment Variables
The tool automatically inherits the following environment variables. Assume they are already configured correctly. Do not manually verify, echo, or debug these variables unless the tool explicitly fails with a configuration error.
*   `NEO4J_URI`, `NEO4J_USER`, `NEO4J_PASSWORD` (Required for `import` and `query`)
*   `GOOGLE_CLOUD_PROJECT` (Required for Vertex AI embeddings)
*   `GOOGLE_CLOUD_LOCATION` (Default: `us-central1`)

## Workflows

### 1. The "One-Shot" Build (Recommended)
To rebuild the entire graph from scratch (Ingest -> Enrich -> Import), use the `build-all` command. This handles all phases sequentially.
```bash
.gemini/skills/graphdb/scripts/graphdb build-all -dir .
```
*   *Options:*
    *   `-clean`: Wipe the database before importing (default: true).

### 2. Manual Pipeline
If you need granular control over each step, follow this sequence:

**Step 0: Check Sync Status**
1. Get local commit: `git rev-parse HEAD`
2. Get graph commit: `.gemini/skills/graphdb/scripts/graphdb query -type status`
3. **Decision:** If hashes match, skip to "Analysis & Querying".

**Step 1: Ingest (Parse & Generate Graph):**
Scans code and generates a graph JSONL file.
```bash
.gemini/skills/graphdb/scripts/graphdb ingest -dir . -output graph.jsonl
```
*   *Options:* `-workers` (concurrency), `-file-list` (specific files).

**Step 2: Enrich (Build Intent Layer):**
Performs **Global Semantic Clustering** to identify latent functional domains across the entire codebase, independent of directory structure. Grounds these domains using Lowest Common Ancestor (LCA) logic.
```bash
.gemini/skills/graphdb/scripts/graphdb enrich-features -input graph.jsonl -output rpg.jsonl
```
*   *Options:*
    *   `-embed-batch-size`: Batch size for embedding generation (default: 100).

**Step 3: Import (Load to Neo4j):**
Loads the generated JSONL files into the active Neo4j database.
```bash
.gemini/skills/graphdb/scripts/graphdb import -input rpg.jsonl -clean
```
*   *Options:* `-clean` (wipe DB first), `-batch-size`.

### 3. Analysis & Querying
The primary way to interact with the graph is via the `query` command.

**Base Syntax:**
```bash
.gemini/skills/graphdb/scripts/graphdb query -type <type> -target "<search_term>" [options]
```

#### Supported Languages
*   **C# / .NET:** `.cs`, `.vb`, `.asp`, `.aspx`, `.ascx`
*   **C / C++:** `.c`, `.cpp`, `.cc`, `.h`, `.hpp`
*   **Java:** `.java`
*   **TypeScript:** `.ts`
*   **SQL:** `.sql`

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
*   **Exact Names:** Structural queries (`neighbors`, `impact`) require exact function names. Use `search-similar` first if you are unsure of the name.
*   **Context:** Always mention the source file and line number when discussing a function.
*   **Missing Data:** If a query returns empty, verify the spelling of the function/module name or try a semantic search.

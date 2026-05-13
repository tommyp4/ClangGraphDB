# Building & Running ClangGraphDB

## Prerequisites

### Required

| Tool | Version | Purpose | Install |
|------|---------|---------|---------|
| Go | 1.21+ | Build the CLI | `winget install GoLang.Go` |
| LLVM/Clang | 18+ | AST extraction via `clang-cl.exe` | `winget install LLVM.LLVM` |
| Visual Studio | 2022 | MFC/ATL/Windows SDK headers | VS Installer with "Desktop development with C++" |
| Git | 2.x | Incremental diff, repo operations | `winget install Git.Git` |
| MinGW | 13+ | CGo compilation (Tree-sitter parsers) | Download from [winlibs.com](https://winlibs.com/) |

### Optional

| Tool | Purpose | Install |
|------|---------|---------|
| Neo4j 5.x | Graph database for import/query | See [Neo4j Setup](#neo4j-setup) below |
| Node.js 20+ | Neo4j manager skill scripts | `winget install OpenJS.NodeJS` |

## Verify Prerequisites

```powershell
go version                    # go1.21+
clang-cl --version            # LLVM 18+
git --version                 # git 2.x
gcc --version                 # MinGW gcc 13+
```

Clang-cl must be able to find MSVC and Windows SDK headers. Verify with:
```powershell
# Should list the MSVC include path
& "C:\Program Files\LLVM\bin\clang-cl.exe" /v 2>&1 | Select-String "include"
```

Visual Studio install paths are detected automatically via `vswhere.exe`.

## Build

```powershell
cd C:\Repos\ClangGraphDB
$env:PATH = "C:\Program Files\Go\bin;C:\Tools\mingw64\bin;" + $env:PATH
go build -o graphdb.exe ./cmd/graphdb/
```

## Environment Variables

Create a `.env` file in the project root:

```
NEO4J_URI=bolt://localhost:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=<your-password>
```

The CLI loads `.env` automatically from the current or parent directories.

## Neo4j Setup

Start Neo4j via Podman/Docker in WSL:

```bash
CONTAINER_NAME="neo4j-graphdb"
NEO4J_PASSWORD="your-password-here"
BASE_PATH="$HOME/neo4j-graphdb"

mkdir -p "$BASE_PATH/data" "$BASE_PATH/logs"

podman run -d \
    --name "$CONTAINER_NAME" \
    -p 7474:7474 \
    -p 7687:7687 \
    -v "$BASE_PATH/data:/data" \
    -v "$BASE_PATH/logs:/logs" \
    -e NEO4J_AUTH="neo4j/$NEO4J_PASSWORD" \
    -e NEO4J_PLUGINS='["apoc"]' \
    docker.io/library/neo4j:5.26.0
```

Verify: open http://localhost:7474 in a browser.

The existing Neo4j manager skill is also available at `.gemini/skills/neo4j-manager/`.

## Usage

### Full extraction (single project)

```powershell
go run ./cmd/graphdb/ clang-ingest `
    -sln "C:\Repos\VIEW\ais\FullBuild.sln" `
    -output output `
    -project LoadingModule `
    -verbose
```

### Full extraction (entire solution)

```powershell
go run ./cmd/graphdb/ clang-ingest `
    -sln "C:\Repos\VIEW\ais\FullBuild.sln" `
    -output output `
    -workers 16
```

### Import into Neo4j

```powershell
go run ./cmd/graphdb/ import `
    -nodes output/nodes.jsonl `
    -edges output/edges.jsonl `
    -batch-size 2000
```

### Incremental update (after code changes)

```powershell
go run ./cmd/graphdb/ clang-incremental `
    -sln "C:\Repos\VIEW\ais\FullBuild.sln" `
    -since HEAD~1 `
    -verbose
```

Use `--dry-run` to preview affected files without making changes.

### Query the graph

```powershell
# Node counts by label
go run ./cmd/graphdb/ query -type cypher -cypher "MATCH (n) RETURN labels(n)[0] AS label, count(n) ORDER BY count DESC"

# Who calls a function
go run ./cmd/graphdb/ query -type cypher -cypher "MATCH (caller)-[:CALLS]->(callee) WHERE callee.fqn CONTAINS 'AutoLoadCases' RETURN caller.fqn, callee.fqn"

# Class hierarchy
go run ./cmd/graphdb/ query -type cypher -cypher "MATCH (child:Class)-[:INHERITS]->(parent:Class) RETURN child.name, parent.name LIMIT 20"
```

### Full pipeline (extract + import + enrich)

```powershell
go run ./cmd/graphdb/ clang-build-all `
    -sln "C:\Repos\VIEW\ais\FullBuild.sln" `
    -output output
```

## CLI Reference

| Command | Description |
|---------|-------------|
| `clang-ingest` | Parse .sln/.vcxproj, extract C++ graph via Clang AST |
| `clang-incremental` | Re-extract only changed files + transitive includers |
| `clang-build-all` | Full pipeline: ingest + import + enrich |
| `import` | Import nodes.jsonl/edges.jsonl into Neo4j |
| `query` | Query the graph (Cypher, search, neighbors, impact, etc.) |
| `serve` | Start HTTP server with D3 visualizer |
| `ingest` | Original Tree-sitter based ingestion (non-C++) |
| `enrich-features` | Build RPG Intent Layer |
| `enrich-history` | Analyze git history for hotspots |
| `enrich-contamination` | Propagate contamination layers |
| `enrich-tests` | Link tests to production functions |

## Architecture

```
FullBuild.sln
    |
    v
[.sln/.vcxproj parser] --> compile_commands.json
    |                              |
    | Project nodes/edges          v
    |                     [clang-cl -ast-dump=json]
    |                              |
    v                              v
nodes.jsonl / edges.jsonl  <-- [streaming JSON parser]
    |
    v
[Neo4j import] --> Neo4j graph database
    |
    v
[query / serve / enrich]
```

## Node & Edge Types

### Nodes

| Type | Description |
|------|-------------|
| File | Source file (.cpp, .h) |
| Function | Free function or class method |
| Constructor | C++ constructor |
| Class | Class or struct definition |
| Field | Class member variable |
| Global | File-scope variable |
| Project | .vcxproj project |

### Edges

| Type | Description |
|------|-------------|
| CALLS | Function calls another function |
| HAS_METHOD | Class has a method |
| DEFINES | Class defines a field |
| DEFINED_IN | Declaration is defined in a file |
| USES_GLOBAL | Function references a global variable |
| INHERITS | Class inherits from another class |
| DEPENDS_ON | Class depends on another class (via field types) |
| INCLUDES | File includes another file |
| PROJECT_CONTAINS | Project contains a source file |
| PROJECT_DEPENDS_ON | Project depends on another project |

## Troubleshooting

**clang-cl not found**: Ensure LLVM is installed and `C:\Program Files\LLVM\bin` is on PATH, or the CLI will search there automatically.

**MFC headers not found**: Visual Studio 2022 with "Desktop development with C++" workload must be installed. The vcxproj parser detects the install path via `vswhere.exe`.

**Large output files**: The streaming JSON parser handles 400+ MB AST dumps per file. If output is still too large, use `-project` to test with a single project first.

**Neo4j connection refused**: Ensure the container is running (`podman ps`) and ports 7474/7687 are forwarded. Check `.env` has the correct credentials.

**MinGW errors during build**: The existing Tree-sitter parsers require CGo. Ensure MinGW's `gcc` is on PATH before running `go build`.

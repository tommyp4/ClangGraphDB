# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## [1.1.307-beta] - 2026-03-25 [Pre-release]
### Changed
- **UI:** Suppressed query logging in `neo4j.go` when progress bars are active to prevent UI flickering.
- **Observability:** Updated `executeQuery` to extract concise query descriptions from Cypher comments, improving log clarity.
- **Observability:** Added `// description` comments to all major Cypher queries.

## [1.1.305-beta] - 2026-03-25 [Pre-release]
### Added
- **RPG:** Decoupled topology generation from LLM-based semantic naming to ensure clustering progress is persisted even if summarization fails later.
- **Scout:** Modernized the Scout agent with the Feathers Workflow and established a 'graceful fallback' protocol for tool usage.
- **Observability:** Centralized Neo4j query logging with parameter sanitization for better transparency across all providers.

### Changed
- **RPG:** Refined domain discovery with diverse sampling (edge-aware) and hierarchical context in LLM prompts.
- **Retry Logic:** Implemented exponential backoff for 429 errors in Summarization, Extraction, and Embedding with a 5-minute safety cap.
- **CLI:** Renamed `--log-file` to `--log` and updated `GRAPHDB_LOG` environment variable for consistency.
- **Agent Tuning:** Increased Scout agent turn limit to 120 and timeout to 60 minutes for deeper research.

### Fixed
- **RPG:** Removed soft-failing fallbacks in favor of hard failures for non-retryable errors to ensure process integrity.
- **CI:** Added `.gemini/graph_data/.gitignore` to the release bundle to prevent tracking of local graph data.


## [1.0.1] - 2026-03-22
### Fixed
- **UI:** Synchronized physical and semantic layer toggle states to prevent UI desyncs on initialization.
- **UI:** Corrected node coloring logic to correctly identify semantic types ("Domain" and "Feature") from nested node properties.
- **UI:** Fixed layout glitches in the layer toggle buttons.

### Added
- **UI:** Added a "Collapse All" button to easily reset the graph exploration state.
- **Web Server:** Disabled client-side caching for embedded static web assets to ensure fresh UI loads.

## [1.0.0] - 2026-03-21
### Added
- **Documentation:** Mapped Michael Feathers legacy modernization workflows to GraphDB and detailed RPG granularity for complex God functions.

## [0.3.297-beta] - 2026-03-21 [Pre-release]
### Added
- **RPG Enrichment:** Implemented standardized progress reporting, replacing log-based output with interactive spinners and progress bars during LLM operations.
- **Analysis:** Added comprehensive research notes and metrics on RPG clustering behaviour and performance scaling.

### Changed
- **Testing:** Isolated database-dependent internal queries into an explicit `integration` build tag, allowing standard unit tests (`make test`) to run consistently without a local Neo4j container.

### Fixed
- **C++ Parser:** Corrected function call edge tagging and signature extraction logic to fix missing relationship data in the graph.
- **Testing Dependencies:** Cleaned up unused environment variables and Testcontainers imports across the testing suite.

## [0.3.290-beta] - 2026-03-19 [Pre-release]
### Added
- **UI:** Added password visibility toggle to settings modal.
- **UI:** Added configuration modal and live database stats.

### Changed
- **UI:** Repositioned physical and semantic layer controls into an overlay panel for better UX.

### Fixed
- **Parsing:** Resolved parser line span boundary issues for C++, C#, Java, SQL, and TypeScript.
- **Orchestrator:** Implemented retry and abort logic to handle errors robustly.
- **Testing:** Fixed flakiness in integration tests and removed breaking scratchpad scripts.
- **Release Manager:** Updated beta versioning format rules.

## [0.3.0-beta.2] - 2026-03-19 [Pre-release]
### Changed
- **Database Management:** Removed `-clean` flag from `build-all` and `import` CLI commands and removed `Wipe`, `RecreateDatabase`, `DropSchema` from `neo4j_loader`.
- **Documentation:** Added `make test` target and updated documentation in `GEMINI.md`. Archived completed feature plans and audit reports.

### Fixed
- **Container Cleanup:** Fixed data wipe permissions in `delete_neo4j_container.sh` using `podman unshare rm -rf`.
## [0.3.0-beta.1] - 2026-03-19 [Pre-release]
### Added
- **RPG Extraction:** Added progress tracking and robust reporting for batch operations. Integrated `CountUnextractedFunctions` for precise sizing, UI progress bars into `RunExtraction`, and periodic progress logging in `RunEmbedding`.

## [0.2.0-beta.7] - 2026-03-18 [Pre-release]
### Added
- **CLI Commands:** Added global `--log-file` option (and `GRAPHDB_LOG_FILE` env var) for persistent debugging, dual-writing standard logs with `Lshortfile` tracing.

### Fixed
- **Ingest:** Properly decode JSON numbers as `int64` and `float64` to prevent silent type assertion failures during the node and edge ingestion processes.

## [0.2.0-beta.6] - 2026-03-18 [Pre-release]
### Fixed
- **CLI Commands:** Removed unsupported `-module` flag from the `enrich-contamination` command, fixing crashes during the `build-all` orchestrator sequence.

## [0.2.0-beta.5] - 2026-03-18 [Pre-release]
### Added
- **UI/UX Improvements:** Stabilized D3 physics and improved neighborhood expansion UX for a smoother graph interaction.
- **UI Tweaks:** Cleaned up layer toggles with matching heights, simplified text, equalized spacing, and removed the unused profile avatar.

### Changed
- **Install Instructions:** Simplified installation commands by auto-generating `.gitignore` files for bundled skills during the release process.
- **Documentation:** Updated Campaign 16 Phase 2 and 3 as completed in the implementation plan.

## [0.2.0-beta.4] - 2026-03-18 [Pre-release]
### Added
- **Fail-Fast Orchestration:** Implemented error thresholds in the RPG orchestration pipelines (Extraction and Summarization) to prevent silent cascading failures during LLM processing.
- **Robust JSON Parsing:** Introduced `ParseLLMJSON` to better handle and strip markdown syntax, extra backticks, and whitespace from unstructured LLM responses.

### Changed
- **Documentation:** Archived inactive plans and renamed design documentation to `UX_DESIGN_OVERVIEW.md`.
- **Install Instructions:** Added specific instructions to README for pre-release version installation.

### Fixed
- Addressed silent feature extraction and summarization failures by failing fast and surfacing underlying context errors.


## [0.2.0-beta.3] - 2026-03-18 [Pre-release]
### Changed
- **Domain Clustering Quality:** Improved atomic feature extraction prompts to output domain-friendly, "Object-Action" (noun-first) descriptors rather than verb-centric descriptions.
- **DDD Naming Prompts:** Split Summarizer prompts into distinct "Domain" and "Feature" levels with Domain-Driven Design (DDD) naming guidance to avoid implementation-based names.
- **Enriched Node Context:** Enriched `NodeToText` with file path and function name for stronger structural and behavioral context during embedding generation.

### Fixed
- **Schema Mismatch:** Fixed `line` vs `start_line` schema property mismatch across all parsers and consumers, completely unblocking the feature extraction pipeline.

### Documentation
- Refactored `README.md` for Gemini CLI skill focus with simplified 1-line installation commands.
- Updated Scout agent and GraphDB architecture overview documentation.

## [0.2.0-beta.2] - 2026-03-17 [Pre-release]
### Changed
- **Pipeline Architecture:** Decoupled `Embedder` from the initial ingest phase, moving all high-fidelity embedding generation to Phase 3 (`enrich-features`). This speeds up ingestion, significantly reduces `nodes.jsonl` file bloat, and solves the overwrite bug.
- **Documentation:** Updated `GRAPHDB_OVERVIEW.md` and `README.md` to reflect the 6-phase pipeline architecture in detail.

### Fixed
- Fixed E2E test failures caused by un-mocked environment variable checks for `GEMINI_GENERATIVE_MODEL`.

## [0.2.0-beta.1] - 2026-03-17 [Pre-release]
### Added
- **LLM-Driven Volatility:** Extractor now seeds volatility flags using LLM heuristics during extraction.
- **Pre-flight Checks:** New safety guards for `Seams` and `Hotspots` queries that fail fast with explicit instructions when data is missing.
- **Release Manager:** Extracted release process documentation into a new dedicated skill.
- **Automated Pre-releases:** Added support to the GitHub Action workflow to automatically publish beta releases.

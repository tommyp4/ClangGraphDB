# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

### Changed
- **Domain Architecture:** Refactored `Clusterer` to return structured metadata (`ClusterGroup`) and implemented `ClearFeatureTopology` for idempotent clustering.
- **Property Alignment:** Renamed the `summary` property to `description` to perfectly align the graph provider and the frontend UI.

### Fixed
- `GetSeams` error message now correctly directs users to run `enrich-contamination` instead of `extract`.
- Added missing tests asserting domain hierarchy properties.

### Removed
- Dead code related to the experimental 3-level hierarchy (`CategoryClusterer`) removed.

## [0.1.0] - 2026-03-16
### Added
- GitHub Action (`release.yml`) for automated cross-platform binary builds (Linux and Windows via Zig).
- Documentation in `GEMINI.md` detailing the automated release process.

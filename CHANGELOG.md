# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

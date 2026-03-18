# Remediation Plan for Item 4 - Missing Tests and Risk Score Incorporation

## 1. Unit Tests for `cmd/graphdb/cmd_enrich_history.go`
- [x] ~~Create `cmd/graphdb/cmd_enrich_history_test.go`.~~ ✅ Implemented
- [x] ~~Write unit test for `analyzeGitHistory` (via `parseGitLog`) with mocked git output.~~ ✅ Implemented

## 2. Unit Tests for `internal/query/neo4j_history.go`
- [x] ~~Create `internal/query/neo4j_history_test.go`.~~ ✅ Implemented
- [x] ~~Write unit tests for `GetHotspots` and `UpdateFileHistory`.~~ ✅ Implemented

## 3. Unit Tests for `internal/storage/neo4j_emitter.go`
- [x] ~~Create `internal/storage/neo4j_emitter_test.go`.~~ ✅ Implemented
- [x] ~~Write unit tests for `Neo4jEmitter`.~~ ✅ Implemented

## 4. Update `CalculateRiskScores` and Test
- [x] ~~Create `internal/query/neo4j_contamination_test.go`.~~ ✅ Implemented
- [x] ~~Write characterization tests for current `CalculateRiskScores`.~~ ✅ Implemented
- [x] ~~Update `CalculateRiskScores` in `internal/query/neo4j_contamination.go` to incorporate `file.change_frequency`.~~ ✅ Implemented
- [x] ~~Update tests to verify `change_frequency` is used correctly.~~ ✅ Implemented

## 5. Invoke Risk Score Calculation in CLI
- [x] ~~Update `cmd/graphdb/cmd_enrich_history.go` to call `CalculateRiskScores` after updating file history.~~ ✅ Implemented

# Feature Implementation Plan: Flexible Domain Discovery

## 📋 Todo Checklist
- [ ] Phase 1: Test Harness & Reproduction
- [ ] Phase 2: Refactor Domain Discovery Logic
- [ ] Phase 3: Strict Path Matching in Builder
- [ ] Phase 4: CLI Configuration
- [ ] Final Review and Verification

## 🔍 Analysis & Investigation
The current `graphdb` skill has hardcoded assumptions about project structure:
1.  **Hardcoded Base Dirs:** It only looks in `internal`, `pkg`, `cmd`, `src`.
2.  **Domain Collisions:** It uses the leaf directory name as the domain identifier. `internal/api` and `cmd/api` both become domain "api", causing one to overwrite the other.
3.  **Loose Matching:** It uses `strings.Contains` for path matching, leading to false positives (e.g., domain "auth" claiming "authentication" files).
4.  **No Root Support:** It doesn't support discovering top-level directories (e.g., `client/`, `server/`) as domains.

## 📝 Implementation Plan

### Prerequisites
- Go environment set up.
- Access to `graphdb-skill` repository.

### Step-by-Step Implementation

#### Phase 1: Test Harness & Reproduction
1.  **Step 1.A (The Harness):** Create a reproduction test case.
    *   *Action:* Create `internal/rpg/discovery_repro_test.go`.
    *   *Goal:* Setup a test with colliding directory names (`cmd/api`, `internal/api`) and top-level directories (`client`, `server`). Assert that current logic fails to distinguish them or find top-level dirs.

#### Phase 2: Refactor Domain Discovery Logic
1.  **Step 2.A (Discovery Logic):** Update `DirectoryDomainDiscoverer`.
    *   *Action:* Modify `internal/rpg/discovery.go`.
    *   *Detail:*
        *   Change `DiscoverDomains` to use the **relative path** (e.g., `internal/api`) as the domain key instead of the leaf name.
        *   **Normalization:** Ensure the path used for the key and value is normalized to forward slashes (`filepath.ToSlash`) to match the graph data format (files in `graph.jsonl` are usually slash-separated).
        *   Implement support for `.` in `BaseDirs`.
        *   Logic:
            *   Iterate over `BaseDirs`.
            *   If `dir` is `.` (or clean path is `.`):
                *   Scan immediate subdirectories of the project root.
                *   Skip hidden directories (starting with `.`) and skip directories that match other configured `BaseDirs` (optimization/deduplication).
                *   Key/Path = `filepath.ToSlash(subdirName)`.
            *   Else:
                *   Scan `filepath.Join(root, dir)`.
                *   Key/Path = `filepath.ToSlash(filepath.Join(dir, subdirName))`.
2.  **Step 2.B (Verify Discovery):** Update tests.
    *   *Action:* Update `internal/rpg/discovery_test.go` and the new repro test.
    *   *Success:* Tests pass, collisions are resolved (keys are unique paths), and top-level dirs are found.

#### Phase 3: Strict Path Matching in Builder
1.  **Step 3.A (Builder Logic):** Update `Builder.Build`.
    *   *Action:* Modify `internal/rpg/builder.go`.
    *   *Detail:*
        *   Replace `strings.Contains(p, pathPrefix)` with strict prefix matching.
        *   Logic: `strings.HasPrefix(p, pathPrefix)`.
        *   **Boundary Check:** Ensure we match directory boundaries to avoid partial name matching (e.g., `auth` vs `authentication`).
        *   Refined Logic:
            ```go
            // Ensure pathPrefix has no trailing slash for consistency
            cleanPrefix := strings.TrimRight(pathPrefix, "/")
            // Check if p is exactly the prefix (file in dir) OR p starts with prefix + /
            match := p == cleanPrefix || strings.HasPrefix(p, cleanPrefix + "/")
            ```
2.  **Step 3.B (Verify Builder):** Update `internal/rpg/builder_test.go`.
    *   *Action:* Add test case for partial name overlap.
    *   *Success:* "auth" domain does NOT claim "authentication" files.

#### Phase 4: CLI Configuration
1.  **Step 4.A (CLI Flag):** Add `--base-dirs` to `enrich-features`.
    *   *Action:* Modify `cmd/graphdb/main.go`.
    *   *Detail:*
        *   Add `baseDirsPtr := fs.String("base-dirs", "internal,pkg,cmd,src", "Comma-separated list of base directories to scan for domains (use '.' for top-level)")`.
        *   Parse the string into a slice: `strings.Split(*baseDirsPtr, ",")`.
        *   Trim spaces from each entry.
        *   Pass this slice to `DirectoryDomainDiscoverer`.
2.  **Step 4.B (Verify CLI):** Manual verification.
    *   *Action:* Build the binary and run `graphdb enrich-features --help` to see the flag.

### Testing Strategy
*   **Unit Tests:** `internal/rpg/discovery_test.go` will be the primary verification ground.
*   **Integration Test:** Create a temporary directory structure mimicking `trucks-v2` and run the `DirectoryDomainDiscoverer` against it.

## 🎯 Success Criteria
1.  `trucks-v2` structure (`client`, `server`, `web`) can be correctly clustered by running with `--base-dirs=.`.
2.  `cmd/api` and `internal/api` are distinct domains `cmd/api` and `internal/api`.
3.  `auth` domain does not incorrectly include `authentication` code.

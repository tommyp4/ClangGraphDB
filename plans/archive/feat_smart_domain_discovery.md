# Feature Implementation Plan: Smart Domain Discovery & Recursive GitIgnore

## 📋 Todo Checklist
- [ ] **Step 1:** Create Verification Harness (Nested `.gitignore` & Discovery Scenario)
- [ ] **Step 2:** Refactor `walker.go` for Recursive `.gitignore` Support
- [ ] **Step 3:** Implement Smart Domain Discovery in `discovery.go`
- [ ] **Step 4:** Enforce Strict Path Matching in `builder.go`
- [ ] **Step 5:** Final Verification

## 🔍 Analysis & Investigation
**Current State:**
1.  **Ingest Walker (`walker.go`):** Uses `filepath.WalkDir` and checks `.gitignore` *only* at the root. It fails to respect nested `.gitignore` files (e.g., in `client/node_modules` inside a monorepo).
2.  **Domain Discovery (`discovery.go`):** Relies on explicit `BaseDirs` configuration. Uses `os.ReadDir` without checking `.gitignore`, potentially identifying ignored directories (like `dist` or `bin`) as domains.
3.  **Graph Builder (`builder.go`):** Uses `strings.Contains` for domain membership, causing false positives (e.g., domain `auth` claiming `authentication/file.go`).

**Requirements:**
-   **Zero Config:** Automatically scan root for domains.
-   **Recursive Ignoring:** Respect `.gitignore` at all levels during ingestion.
-   **Precision:** Strict path matching for domain attribution.

## 📝 Implementation Plan

### Prerequisites
-   Ensure `github.com/monochromegane/go-gitignore` is available (already in `go.mod`).

### Step-by-Step Implementation

#### Phase 1: The Verification Harness
1.  **Step 1.A: Create Test Fixture**
    *   *Action:* Create `test/fixtures/smart_discovery/` with:
        *   `.gitignore` (ignores `ignored_root/`)
        *   `ignored_root/file.txt`
        *   `domain_a/file.go`
        *   `domain_b/.gitignore` (ignores `nested_secret/`)
        *   `domain_b/nested_secret/secret.txt`
        *   `domain_b/public/ok.txt`
        *   `auth/user.go`
        *   `authentication/service.go`
    *   *Goal:* Establish a baseline to prove current failures (Walker finding secrets, Discovery finding `ignored_root`, Builder confusing `auth`).

2.  **Step 1.B: Create Test Suite**
    *   *Action:* Create `internal/ingest/walker_recursive_test.go` and `internal/rpg/discovery_smart_test.go`.
    *   *Goal:* Assert that:
        1. Walker finds `domain_b/public/ok.txt` but NOT `domain_b/nested_secret/secret.txt`.
        2. Discoverer finds `domain_a`, `domain_b`, `auth`, `authentication`. Skips `ignored_root`.
        3. Builder correctly assigns `authentication/service.go` to `authentication` domain, not `auth`.

#### Phase 2: Recursive Walker (Ingest)
1.  **Step 2.A: Implement Recursive Walk Logic**
    *   *Action:* Refactor `internal/ingest/walker.go`.
    *   *Detail:*
        *   Replace `filepath.WalkDir` with a custom recursive function `walkRecursive(dir string, parentMatchers []gitignore.IgnoreMatcher)`.
        *   In each directory:
            *   Check for local `.gitignore`.
            *   If found, parse and append to `parentMatchers` to create `currentMatchers`.
            *   Iterate directory entries.
            *   Check `d.Name()` against all matchers in `currentMatchers`.
            *   Recurse for directories, visit for files.
    *   *Constraint:* Ensure performance is acceptable (avoid re-parsing ignores if possible, though caching might be premature optimization).

2.  **Step 2.B: Verify Walker**
    *   *Action:* Run `go test ./internal/ingest/...`
    *   *Success:* Walker tests pass, specifically the nested ignore case.

#### Phase 3: Smart Discovery (RPG)
1.  **Step 3.A: Implement Auto-Discovery**
    *   *Action:* Modify `internal/rpg/discovery.go`.
    *   *Detail:*
        *   Update `DiscoverDomains`.
        *   If `BaseDirs` is empty (or by default):
            *   Load root `.gitignore` (if exists).
            *   List all top-level directories of project root.
            *   Filter out:
                *   Hidden dirs (`.*`).
                *   Ignored dirs (using root matcher).
                *   `BaseDirs` duplicates (if mixed mode).
            *   Return remaining dirs as domains.
2.  **Step 3.B: Verify Discovery**
    *   *Action:* Run `go test ./internal/rpg/...`
    *   *Success:* Discovery finds top-level folders but respects root ignore.

#### Phase 4: Strict Builder Matching (RPG)
1.  **Step 4.A: Fix Path Matching**
    *   *Action:* Modify `internal/rpg/builder.go`.
    *   *Detail:*
        *   Locate the loop assigning functions to domains.
        *   Change `strings.Contains(p, pathPrefix)` to strict prefix logic:
            ```go
            // Ensure strict directory matching (e.g. "auth/" vs "authentication/")
            relPath, err := filepath.Rel(pathPrefix, p)
            if err == nil && !strings.HasPrefix(relPath, "..") {
                // It is inside
            }
            // OR simple string manipulation
            cleanPrefix := filepath.Clean(pathPrefix) + string(os.PathSeparator)
            if strings.HasPrefix(p, cleanPrefix) || p == filepath.Clean(pathPrefix) { ... }
            ```
2.  **Step 4.B: Verify Builder**
    *   *Action:* Run existing and new tests.
    *   *Success:* `auth` domain does not claim `authentication` files.

## 🎯 Success Criteria
1.  **Zero Config:** User can run tool in `trucks-v2` root and it finds `client`, `server`, `web` as domains automatically.
2.  **Privacy/Cleanup:** `node_modules`, `bin`, `obj` folders are ignored during ingestion even if deep in the tree.
3.  **Correctness:** No overlapping domain claims.

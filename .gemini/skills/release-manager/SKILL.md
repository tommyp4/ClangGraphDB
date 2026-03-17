---
name: release-manager
description: Automates the versioning, changelog generation, and GitHub release process for the project. Use when the user wants to bump the version, prepare a new release, or release new binaries.
---

# Release Manager Skill

This skill automates the creation of cross-platform releases and changelog generation for the GraphDB project.

## Workflow

When the user requests to bump the version, prepare a release, or publish a pre-release (e.g., "Prepare a new release v1.x.x", "Publish a beta"):

1.  **Verify State:** Ensure the working directory is clean (`git status`). Ensure you are on the correct branch (pre-releases can happen on feature branches; stable releases typically on `main`).
2.  **Gather History:** Run `git log $(git describe --tags --abbrev=0)..HEAD` to get a list of all raw commits since the last release.
3.  **Summarize & Update Changelog:** 
    *   Synthesize those commits into human-readable bullet points (e.g., Added, Changed, Fixed).
    *   Prepend these notes into the `CHANGELOG.md` file under the new version header, following the existing format. If it is a pre-release (e.g., `-beta`, `-rc`), clearly mark it as `[Pre-release]` in the header.
4.  **Commit & Tag:**
    *   Commit the updated changelog: `git commit -am "chore: release <version>"`
    *   Create a new Git tag: `git tag <version>` (e.g., `v1.0.0`, `v1.1.0-beta.1`) on the *current* branch.
5.  **Push:** Push the commit and the tag to GitHub: `git push origin HEAD && git push origin <version>`.
6.  **Confirm Execution:** Inform the user that pushing the tag triggered the `.github/workflows/release.yml` GitHub Actions workflow.

### Background Information

*   **GitHub Action:** Pushing a `v*` tag triggers the remote Action which compiles binaries for Linux and Windows using Go and Zig.
*   **Version Injection:** The `Makefile` automatically captures the git tag using `git describe` and passes it to the Go compiler. When running `graphdb version`, it displays the official release tag instead of "dev".
*   **Release Notes:** GitHub automatically generates web-based release notes based on PRs via the Action, but `CHANGELOG.md` serves as the permanent, in-repo history.

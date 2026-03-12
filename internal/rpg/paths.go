package rpg

import (
	"path/filepath"
	"strings"
)

// FindLowestCommonAncestor returns the longest common directory prefix of the given paths.
// Returns "" if paths is empty or if there is no common root (e.g. different drives on Windows, mixed absolute/relative paths).
// For disjoint relative paths (e.g. "a/b", "c/d"), it returns "." (current directory).
func FindLowestCommonAncestor(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	// Start with the first path as the candidate common ancestor
	common := filepath.Clean(paths[0])

	for _, p := range paths[1:] {
		p = filepath.Clean(p)

		// If one is abs and other is rel, they have no common ancestor
		if filepath.IsAbs(common) != filepath.IsAbs(p) {
			return ""
		}

		rel, err := filepath.Rel(common, p)
		if err != nil {
			// e.g. different drives on Windows
			return ""
		}

		// Move common up until p is inside common (rel doesn't start with "..")
		for strings.HasPrefix(rel, "..") {
			parent := filepath.Dir(common)

			// If we hit the root/current dir limit and we still need to go up..
			if parent == common {
				// We can't go higher.
				// This handles cases where we are at root "/" or current "."
				// and the target path requires going up (e.g. ".." relative path).
				// We consider this "no common ancestor" within the scope of the paths provided.
				return ""
			}

			common = parent
			rel, err = filepath.Rel(common, p)
			if err != nil {
				return ""
			}
		}
	}

	return common
}

package rpg

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/monochromegane/go-gitignore"
)

type DirectoryDomainDiscoverer struct {
	BaseDirs []string
}

func (d *DirectoryDomainDiscoverer) DiscoverDomains(rootPath string) (map[string]string, error) {
	domains := make(map[string]string)
	
	// Normalize BaseDirs
	// If empty, default to "." (Smart Discovery)
	targets := d.BaseDirs
	if len(targets) == 0 {
		targets = []string{"."}
	}

	baseDirsSet := make(map[string]bool)
	for _, base := range targets {
		clean := filepath.Clean(base)
		if clean == "." {
			clean = ""
		}
		baseDirsSet[clean] = true
	}

	// Load root .gitignore if it exists
	var ignoreMatcher gitignore.IgnoreMatcher
	gitignorePath := filepath.Join(rootPath, ".gitignore")
	if content, err := os.ReadFile(gitignorePath); err == nil {
		ignoreMatcher = gitignore.NewGitIgnoreFromReader(rootPath, strings.NewReader(string(content)))
	}

	// Process unique base directories
	for base := range baseDirsSet {
		// Determine directory to scan
		scanDir := filepath.Join(rootPath, base)
		
		entries, err := os.ReadDir(scanDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue // Skip hidden directories
			}

			// Construct relative path for the domain (e.g. "client" or "internal/api")
			relPath := filepath.Join(base, name)
			
			// Check if ignored
			if ignoreMatcher != nil {
				// Match against the full path or relative to root?
				// Our matcher is rooted at rootPath.
				fullPath := filepath.Join(rootPath, relPath)
				ignored := ignoreMatcher.Match(fullPath, true)
				if ignored {
					continue
				}
			}

			// Skip if this directory is itself a BaseDir (to avoid double scanning)
			if baseDirsSet[relPath] {
				continue
			}

			// Use the relative path as the unique key
			key := filepath.ToSlash(relPath)
			domains[key] = key
		}
	}

	// If no domains found, fall back to root
	if len(domains) == 0 {
		domains["root"] = ""
	}

	return domains, nil
}

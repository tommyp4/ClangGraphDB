package ingest

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestWalker_RecursiveGitIgnore validates that .gitignore files in subdirectories
// correctly override or extend the ignore rules from parent directories.
func TestWalker_RecursiveGitIgnore(t *testing.T) {
	// Structure:
	// .
	// ├── .gitignore          (ignores *.log)
	// ├── root.log            (IGNORED)
	// ├── root.txt            (KEPT)
	// ├── level1/
	// │   ├── .gitignore      (unignores !important.log)
	// │   ├── normal.log      (IGNORED inherited)
	// │   ├── important.log   (KEPT by unignore)
	// │   ├── level1.txt      (KEPT)
	// │   └── level2/
	// │       ├── .gitignore  (ignores *.txt)
	// │       ├── deep.txt    (IGNORED by new rule)
	// │       └── deep.log    (IGNORED inherited)
	
	tempDir := t.TempDir()

	// 1. Create Directories
	dirs := []string{
		"level1",
		"level1/level2",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(tempDir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// 2. Create Files
	files := map[string]string{
		"root.log":               "ignore me",
		"root.txt":               "keep me",
		"level1/normal.log":      "ignore me too",
		"level1/important.log":   "keep me please",
		"level1/level1.txt":      "keep me",
		"level1/level2/deep.txt": "ignore me now",
		"level1/level2/deep.log": "ignore me still",
	}
	for path, content := range files {
		fullPath := filepath.Join(tempDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 3. Create .gitignores
	ignores := map[string]string{
		".gitignore":               "*.log\n",
		"level1/.gitignore":        "!important.log\n",
		"level1/level2/.gitignore": "*.txt\n",
	}
	for path, content := range ignores {
		fullPath := filepath.Join(tempDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 4. Run Walker
	// Use a mock emitter or just use Count/Walk directly.
	// We'll use Walk to collect paths.
	walker := NewWalker(1, &MockEmitter{})
	
	collectedPaths := []string{}
	err := walker.Walk(context.Background(), tempDir, func(path string, d os.DirEntry) error {
		if !d.IsDir() {
			rel, _ := filepath.Rel(tempDir, path)
			collectedPaths = append(collectedPaths, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Walker failed: %v", err)
	}

	// 5. Verify Expectations
	expected := []string{
		"root.txt",
		"level1/important.log",
		"level1/level1.txt",
		// level1/level2/deep.txt is ignored by level2/.gitignore
		// level1/level2/deep.log is ignored by root .gitignore
		".gitignore",
		"level1/.gitignore",
		"level1/level2/.gitignore",
	}
	
	// Normalize and sort for comparison
	sort.Strings(collectedPaths)
	sort.Strings(expected)

	if len(collectedPaths) != len(expected) {
		t.Errorf("Mismatch in file count.\nExpected (%d): %v\nGot (%d): %v", len(expected), expected, len(collectedPaths), collectedPaths)
		return
	}

	for i := range expected {
		if collectedPaths[i] != expected[i] {
			t.Errorf("Mismatch at index %d: expected %s, got %s", i, expected[i], collectedPaths[i])
		}
	}
}

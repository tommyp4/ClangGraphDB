package ingest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWalker_Count_RespectsGitIgnore(t *testing.T) {
	// Setup temporary directory structure
	// .
	// ├── .gitignore
	// ├── file1.txt
	// ├── file2.txt
	// ├── ignored.log
	// ├── subdir/
	// │   ├── file3.txt
	// │   └── ignored_subdir.log
	// └── ignored_dir/
	//     └── file4.txt

	tempDir, err := os.MkdirTemp("", "walker_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create files
	files := []string{
		"file1.txt",
		"file2.txt",
		"ignored.log",
		"subdir/file3.txt",
		"subdir/ignored_subdir.log",
		"ignored_dir/file4.txt",
	}

	for _, f := range files {
		path := filepath.Join(tempDir, f)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Create .gitignore
	gitignoreContent := `
*.log
ignored_dir/
`
	if err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte(gitignoreContent), 0644); err != nil {
		t.Fatalf("Failed to write .gitignore: %v", err)
	}

	// Initialize Walker
	// We don't need real emitter for Count
	walker := NewWalker(1, &MockEmitter{})

	// Test Count
	count, err := walker.Count(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("Walker.Count failed: %v", err)
	}

	// Expected count:
	// file1.txt (included)
	// file2.txt (included)
	// ignored.log (excluded by *.log)
	// subdir/file3.txt (included)
	// subdir/ignored_subdir.log (excluded by *.log)
	// ignored_dir/file4.txt (excluded by ignored_dir/)
	// .gitignore (included? usually yes unless ignored)
	
	// So 3 txt files + .gitignore = 4 files.
	// Wait, does walker include .gitignore itself?
	// The walker iterates over all files. If .gitignore is not ignored, it is included.
	
	expectedCount := int64(4) 
	
	if count != expectedCount {
		t.Errorf("Expected count %d, got %d", expectedCount, count)
	}
}

func TestWalker_Count_NoGitIgnore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "walker_test_no_ignore")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	files := []string{
		"file1.txt",
		"subdir/file2.txt",
	}

	for _, f := range files {
		path := filepath.Join(tempDir, f)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	walker := NewWalker(1, &MockEmitter{})

	count, err := walker.Count(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("Walker.Count failed: %v", err)
	}

	expectedCount := int64(2)
	if count != expectedCount {
		t.Errorf("Expected count %d, got %d", expectedCount, count)
	}
}

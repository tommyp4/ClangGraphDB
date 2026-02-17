package e2e

import (
	"context"
	"encoding/json"
	"graphdb/internal/ingest"
	"graphdb/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MockEmbedder for testing
type MockEmbedder struct{}

func (m *MockEmbedder) Embed(text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *MockEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = []float32{0.1, 0.2, 0.3}
	}
	return result, nil
}

func TestIngestPaths(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "graphdb_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a dummy source file
	srcFile := filepath.Join(tmpDir, "Test.cs")
	content := "namespace TestNamespace { public class TestClass {} }"
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Output file
	outFile := filepath.Join(tmpDir, "graph.jsonl")
	f, err := os.Create(outFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// Run Ingest
	emitter := storage.NewJSONLEmitter(f)
	
	walker := ingest.NewWalker(1, &MockEmbedder{}, emitter)
	if err := walker.Run(context.Background(), tmpDir); err != nil {
		t.Fatal(err)
	}
	// Emitter doesn't need explicit Close() as we close the file, but good practice if it buffered.
	// JSONLEmitter wraps json.Encoder which writes directly.

	// Verify Output
	bytes, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(string(bytes), "\n")
	foundFileNode := false
	for _, line := range lines {
		if line == "" {
			continue
		}
		var node map[string]interface{}
		if err := json.Unmarshal([]byte(line), &node); err != nil {
			continue // Might be edge or partial
		}

		if label, ok := node["type"].(string); ok && label == "File" {
			foundFileNode = true
			id, _ := node["id"].(string)
			// Check if ID is absolute
			if filepath.IsAbs(id) {
				t.Fatalf("File ID is absolute: %s", id)
			} else {
				t.Logf("File ID is relative: %s", id)
			}
		}
	}

	if !foundFileNode {
		// It might be possible that the walker runs in background and we check too early?
		// No, walker.Run waits for pool to stop.
		t.Logf("Output content: %s", string(bytes))
		t.Fatal("No File node found")
	}
}

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

func TestCSharpFQN(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "graphdb_fqn_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a C# source file
	srcFile := filepath.Join(tmpDir, "Controller.cs")
	content := `
namespace Trucks.Server {
    public class PaymentHistoryController {
        public void Get() {}
    }
}
`
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
	
	walker := ingest.NewWalker(1, emitter)
	if err := walker.Run(context.Background(), tmpDir); err != nil {
		t.Fatal(err)
	}

	// Verify Output
	bytes, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(string(bytes), "\n")
	foundClassNode := false
	for _, line := range lines {
		if line == "" {
			continue
		}
		var node map[string]interface{}
		if err := json.Unmarshal([]byte(line), &node); err != nil {
			continue // Might be edge or partial
		}

		if label, ok := node["type"].(string); ok && label == "Class" {
			foundClassNode = true
			id, _ := node["id"].(string)
			// Check if ID is FQN (no file path) using new standard Label:FQN:Signature
			expectedID := "Class:Trucks.Server.PaymentHistoryController:"
			
			// Currently it should fail because it has file path prefix
			if strings.Contains(id, "Controller.cs:") {
				t.Fatalf("Class ID contains file path: %s", id)
			}
			
			if id != expectedID {
				t.Fatalf("Expected ID %s, got %s", expectedID, id)
			}
		}
	}

	if !foundClassNode {
		t.Fatal("No Class node found")
	}
}

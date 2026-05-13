package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"clang-graphdb/internal/graph"
	"clang-graphdb/internal/ingest"
)

func TestIngest_Ignore(t *testing.T) {
	// 1. Setup temporary directory
	tempDir, err := os.MkdirTemp("", "ingest_ignore_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// 2. Create file structure
	files := map[string]string{
		".gitignore":                "node_modules/\n",
		"src/main.ts":               "console.log('hello');",
		"node_modules/lib/index.ts": "export const x = 1;",
		".git/test.ts":              "console.log('git');",
	}

	for path, content := range files {
		fullPath := filepath.Join(tempDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 3. Run Walker
	emitter := &IgnoreTestEmitter{}

	walker := ingest.NewWalker(2, emitter)

	err = walker.Run(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("Walker.Run failed: %v", err)
	}

	// 4. Assertions
	var ingestedFiles []string
	for _, node := range emitter.Nodes {
		ingestedFiles = append(ingestedFiles, node.ID)
	}

	foundNodeModules := false
	foundSrc := false
    foundGit := false

	for _, f := range ingestedFiles {
		if filepath.Base(f) == "index.ts" {
			foundNodeModules = true
		}
		if filepath.Base(f) == "main.ts" {
			foundSrc = true
		}
        		if filepath.Base(f) == "test.ts" {
        			foundGit = true
        		}	}

    // Expect FAILURE initially (Red)
    if foundNodeModules {
        t.Errorf("node_modules should be ignored, but was ingested")
    }
    
    if foundGit {
        t.Errorf(".git should be ignored, but was ingested")
    }

	if !foundSrc {
		t.Errorf("src/main.ts should be ingested, but was not")
	}
}

// IgnoreTestEmitter implements storage.Emitter
type IgnoreTestEmitter struct {
	Nodes []*graph.Node
}

func (m *IgnoreTestEmitter) EmitNode(node *graph.Node) error {
	m.Nodes = append(m.Nodes, node)
	return nil
}

func (m *IgnoreTestEmitter) EmitEdge(edge *graph.Edge) error {
	return nil
}

func (m *IgnoreTestEmitter) Close() error {
	return nil
}

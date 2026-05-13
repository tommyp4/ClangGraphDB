package ingest

import (
	"clang-graphdb/internal/analysis"
	"clang-graphdb/internal/graph"
	"testing"
	"sync"
	"path/filepath"
	"os"
)

// MockEmitter collects items
type MockEmitter struct {
	Nodes []*graph.Node
	Edges []*graph.Edge
	mu    sync.Mutex
}

func (m *MockEmitter) EmitNode(node *graph.Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Nodes = append(m.Nodes, node)
	return nil
}

func (m *MockEmitter) EmitEdge(edge *graph.Edge) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Edges = append(m.Edges, edge)
	return nil
}

func (m *MockEmitter) Close() error {
	return nil
}

func TestWorkerPool_EmitsFileAndDefinedInEdges(t *testing.T) {
	// 1. Setup
	emitter := &MockEmitter{}
	workerPool := NewWorkerPool(1, emitter)
	
	workerPool.Start()
	
	// 2. Submit a file
	wd, _ := os.Getwd()
	repoRoot := filepath.Dir(filepath.Dir(wd))
	fixturePath := filepath.Join(wd, "../../test/fixtures/typescript/sample.ts")
	workerPool.Submit(repoRoot, fixturePath)
	
	// 3. Stop and wait
	workerPool.Stop()
	
	// 4. Assert
	emitter.mu.Lock()
	defer emitter.mu.Unlock()
	
	foundFileNode := false
	var fileNodeID string
	
	for _, node := range emitter.Nodes {
		if node.Label == "File" {
			foundFileNode = true
			fileNodeID = node.ID
			
			if _, ok := node.Properties["file"]; !ok {
				t.Error("File node missing 'file' property")
			}
			if _, ok := node.Properties["name"]; !ok {
				t.Error("File node missing 'name' property")
			}
			break
		}
	}
	
	if !foundFileNode {
		t.Error("Expected to find a Node with Label 'File', but none found")
	}
	
	foundDefinedInEdge := false
	for _, edge := range emitter.Edges {
		if edge.Type == "DEFINED_IN" {
			foundDefinedInEdge = true
			if edge.TargetID != fileNodeID {
				t.Errorf("DEFINED_IN edge target %s does not match File node ID %s", edge.TargetID, fileNodeID)
			}
			break
		}
	}
	
	if !foundDefinedInEdge {
		t.Error("Expected to find an Edge with Type 'DEFINED_IN', but none found")
	}
}

// MockParser for testing
type MockParser struct{}
func (m *MockParser) Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error) {
	return []*graph.Node{
		{ID: "node1", Label: "Function", Properties: map[string]interface{}{"name": "testFunc"}},
	}, nil, nil
}

func TestWorkerPool_TagsTestFiles(t *testing.T) {
	// 1. Setup
	analysis.RegisterParser(".go", &MockParser{})
	emitter := &MockEmitter{}
	workerPool := NewWorkerPool(1, emitter)
	workerPool.Start()

	// 2. Submit a test file
	wd, _ := os.Getwd()
	repoRoot := filepath.Dir(filepath.Dir(wd))
	realPath := filepath.Join(wd, "worker_test.go") // current file is a test file
	workerPool.Submit(repoRoot, realPath)

	// 3. Stop and wait
	workerPool.Stop()

	// 4. Assert
	emitter.mu.Lock()
	defer emitter.mu.Unlock()

	foundTestFile := false
	foundTestFunction := false

	t.Logf("Emitted nodes: %d", len(emitter.Nodes))
	for _, node := range emitter.Nodes {
		t.Logf("Node: ID=%s, Label=%s, Properties=%v", node.ID, node.Label, node.Properties)
		if node.Label == "File" {
			if val, ok := node.Properties["is_test"].(bool); ok && val {
				foundTestFile = true
			}
		}
		if node.Label == "Function" || node.Label == "Method" {
			if val, ok := node.Properties["is_test"].(bool); ok && val {
				foundTestFunction = true
			}
		}
	}

	if !foundTestFile {
		t.Error("Expected File node to be tagged with is_test: true")
	}
	if !foundTestFunction {
		t.Error("Expected Function/Method nodes to be tagged with is_test: true")
	}
}

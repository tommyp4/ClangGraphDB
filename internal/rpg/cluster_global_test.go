package rpg

import (
	"graphdb/internal/graph"
	"sort"
	"strings"
	"testing"
)

func TestGlobalEmbeddingClusterer_Cluster(t *testing.T) {
	// Setup nodes
	nodes := []graph.Node{
		{ID: "fn1", Properties: map[string]interface{}{"file": "auth/login.go", "line": 10, "end_line": 20, "content": "func Login() {}"}},
		{ID: "fn2", Properties: map[string]interface{}{"file": "auth/logout.go", "line": 15, "end_line": 25, "content": "func Logout() {}"}},
		{ID: "fn3", Properties: map[string]interface{}{"file": "payment/process.go", "line": 5, "end_line": 15, "content": "func Process() {}"}},
	}

	// Setup PrecomputedEmbeddings
	embeddings := map[string][]float32{
		"fn1": {1.0, 0.0},
		"fn2": {0.9, 0.1}, // Close to fn1
		"fn3": {0.0, 1.0}, // Far from fn1/fn2
	}

	// Setup Inner Clusterer (Mocking the K-Means result)
	mockInner := &MockClusterer{
		ClusterFunc: func(n []graph.Node, d string) (map[string][]graph.Node, error) {
			// Simulate K-Means returning 2 clusters based on our knowledge of embeddings
			// Cluster 1: fn1, fn2
			// Cluster 2: fn3
			return map[string][]graph.Node{
				"root-cluster-0": {nodes[0], nodes[1]},
				"root-cluster-1": {nodes[2]},
			}, nil
		},
	}

	// Setup Summarizer
	mockSummarizer := &MockSummarizer{
		SummarizeFunc: func(snippets []string) (string, string, error) {
			// Simple logic to name based on content
			content := strings.Join(snippets, " ")
			if strings.Contains(content, "Login") || strings.Contains(content, "Logout") {
				return "Authentication System", "Handles user auth", nil
			}
			if strings.Contains(content, "Process") {
				return "Payment System", "Handles payments", nil
			}
			return "Unknown System", "Desc", nil
		},
	}

	// Setup Loader (Mocking file read)
	mockLoader := func(path string, start, end int) (string, error) {
		// Return content based on file path for simplicity
		if strings.Contains(path, "login") {
			return "func Login() {}", nil
		}
		if strings.Contains(path, "logout") {
			return "func Logout() {}", nil
		}
		if strings.Contains(path, "process") {
			return "func Process() {}", nil
		}
		return "", nil
	}

	// Create GlobalEmbeddingClusterer
	gc := &GlobalEmbeddingClusterer{
		Inner:                 mockInner,
		Summarizer:            mockSummarizer,
		Loader:                mockLoader,
		PrecomputedEmbeddings: embeddings,
	}

	// Execute Cluster
	result, err := gc.Cluster(nodes, "root")
	if err != nil {
		t.Fatalf("Cluster failed: %v", err)
	}

	// Verify Results
	if len(result) != 2 {
		t.Errorf("Expected 2 global domains, got %d", len(result))
	}

	// Check Domain Names
	// We expect "Authentication System" and "Payment System"
	var keys []string
	for k := range result {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if keys[0] != "Authentication System" {
		t.Errorf("Expected 'Authentication System', got '%s'", keys[0])
	}
	if keys[1] != "Payment System" {
		t.Errorf("Expected 'Payment System', got '%s'", keys[1])
	}

	// Check content of Authentication System
	authNodes := result["Authentication System"]
	if len(authNodes) != 2 {
		t.Errorf("Expected 2 nodes in Auth System, got %d", len(authNodes))
	}
}

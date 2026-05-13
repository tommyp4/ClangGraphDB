package rpg

import (
	"clang-graphdb/internal/graph"
	"sort"
	"strings"
	"testing"
)

func TestGlobalEmbeddingClusterer_Cluster(t *testing.T) {
	// Setup nodes
	nodes := []graph.Node{
		{ID: "fn1", Properties: map[string]interface{}{"file": "auth/login.go", "start_line": 10, "end_line": 20, "content": "func Login() {}"}},
		{ID: "fn2", Properties: map[string]interface{}{"file": "auth/logout.go", "start_line": 15, "end_line": 25, "content": "func Logout() {}"}},
		{ID: "fn3", Properties: map[string]interface{}{"file": "payment/process.go", "start_line": 5, "end_line": 15, "content": "func Process() {}"}},
	}

	// Setup PrecomputedEmbeddings
	embeddings := map[string][]float32{
		"fn1": {1.0, 0.0},
		"fn2": {0.9, 0.1}, // Close to fn1
		"fn3": {0.0, 1.0}, // Far from fn1/fn2
	}

	// Setup Inner Clusterer (Mocking the K-Means result)
	mockInner := &MockClusterer{
		ClusterFunc: func(n []graph.Node, d string) ([]ClusterGroup, error) {
			// Simulate K-Means returning 2 clusters based on our knowledge of embeddings
			// Cluster 1: fn1, fn2
			// Cluster 2: fn3
			return []ClusterGroup{
				{Name: "root-cluster-0", Nodes: []graph.Node{nodes[0], nodes[1]}},
				{Name: "root-cluster-1", Nodes: []graph.Node{nodes[2]}},
			}, nil
		},
	}

	// Setup Summarizer
	mockSummarizer := &MockSummarizer{
		SummarizeFunc: func(snippets []string, level string) (string, string, error) {
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
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	if result[0].Name != "Authentication System" {
		t.Errorf("Expected 'Authentication System', got '%s'", result[0].Name)
	}
	if result[1].Name != "Payment System" {
		t.Errorf("Expected 'Payment System', got '%s'", result[1].Name)
	}

	// Check content of Authentication System
	authGroup := result[0]
	if len(authGroup.Nodes) != 2 {
		t.Errorf("Expected 2 nodes in Auth System, got %d", len(authGroup.Nodes))
	}
}

func TestGlobalEmbeddingClusterer_SnippetPropertyMismatch(t *testing.T) {
	// Setup nodes using "start_line" as it would be returned if the DB schema used it instead of "start_line"
	// This simulates the gap between orchestrator.go using "start_line" and cluster_global.go expecting "start_line"
	nodes := []graph.Node{
		{
			ID: "fn1",
			Properties: map[string]interface{}{
				"file":            "auth/login.go",
				"start_line":            10,
				"end_line":        20,
				"atomic_features": []string{"fallback-feature"},
			},
		},
	}

	embeddings := map[string][]float32{
		"fn1": {1.0, 0.0},
	}

	mockInner := &MockClusterer{
		ClusterFunc: func(n []graph.Node, d string) ([]ClusterGroup, error) {
			return []ClusterGroup{
				{Name: "root-cluster-0", Nodes: []graph.Node{nodes[0]}},
			}, nil
		},
	}

	var loaderCalled bool
	mockLoader := func(path string, start, end int) (string, error) {
		loaderCalled = true
		return "func Login() {}", nil
	}

	var passedSnippets []string
	mockSummarizer := &MockSummarizer{
		SummarizeFunc: func(snippets []string, level string) (string, string, error) {
			passedSnippets = append(passedSnippets, snippets...)
			return "Test System", "Desc", nil
		},
	}

	gc := &GlobalEmbeddingClusterer{
		Inner:                 mockInner,
		Summarizer:            mockSummarizer,
		Loader:                mockLoader,
		PrecomputedEmbeddings: embeddings,
	}

	_, err := gc.Cluster(nodes, "root")
	if err != nil {
		t.Fatalf("Cluster failed: %v", err)
	}

	if !loaderCalled {
		t.Errorf("Expected snippet loader to be called, but it was skipped. Ensure 'start_line' is used.")
	}
}

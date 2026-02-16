package rpg

import (
	"graphdb/internal/graph"
	"strings"
	"testing"
)

// MockClusterer implements Clusterer for testing
type MockClusterer struct {
	ClusterFunc func(nodes []graph.Node, domain string) (map[string][]graph.Node, error)
}

func (m *MockClusterer) Cluster(nodes []graph.Node, domain string) (map[string][]graph.Node, error) {
	if m.ClusterFunc != nil {
		return m.ClusterFunc(nodes, domain)
	}
	// Default behavior: return empty
	return make(map[string][]graph.Node), nil
}

func TestBuilder_Build(t *testing.T) {
	// Setup Global Clusterer to act as the primary discovery mechanism
	mockGlobal := &MockClusterer{
		ClusterFunc: func(nodes []graph.Node, domain string) (map[string][]graph.Node, error) {
			clusters := make(map[string][]graph.Node)
			// Simulate finding 2 domains
			clusters["Auth"] = []graph.Node{nodes[0]}
			clusters["Payment"] = []graph.Node{nodes[1]}
			return clusters, nil
		},
	}

	// Setup Feature Clusterer (2nd level)
	mockFeatureClusterer := &MockClusterer{
		ClusterFunc: func(nodes []graph.Node, domain string) (map[string][]graph.Node, error) {
			clusters := make(map[string][]graph.Node)
			// One feature per domain for simplicity
			clusters[domain+"-Feature"] = nodes
			return clusters, nil
		},
	}

	builder := &Builder{
		GlobalClusterer: mockGlobal,
		Clusterer:       mockFeatureClusterer,
	}

	// Input: A mix of functions
	functions := []graph.Node{
		{ID: "func1", Properties: map[string]interface{}{"file": "src/auth/login.go"}},
		{ID: "func2", Properties: map[string]interface{}{"file": "src/payment/charge.go"}},
	}

	// Execute
	features, edges, err := builder.Build("src/", functions)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	nodes, allEdges := Flatten(features, edges)

	// Verify Structure
	// 2 Domains (Auth, Payment)
	if len(features) != 2 {
		t.Errorf("Expected 2 domain features, got %d", len(features))
	}

	// Verify Nodes
	// 2 Domains + 2 Features (one per domain) = 4 Feature nodes
	if len(nodes) != 4 {
		t.Errorf("Expected 4 feature nodes, got %d", len(nodes))
	}

	// Verify Edges
	// 2 PARENT_OF (Domain -> Feature)
	// 2 IMPLEMENTS (Function -> Feature)
	// Total 4 edges
	if len(allEdges) != 4 {
		t.Errorf("Expected 4 edges, got %d", len(allEdges))
	}

	foundImplements := 0
	foundParentOf := 0
	for _, e := range allEdges {
		if e.Type == "IMPLEMENTS" {
			foundImplements++
			if e.SourceID != "func1" && e.SourceID != "func2" {
				t.Errorf("IMPLEMENTS edge SourceID should be a function ID, got %s", e.SourceID)
			}
			if !strings.HasPrefix(e.TargetID, "feat-") {
				t.Errorf("IMPLEMENTS edge TargetID should be a feature ID (feat-*), got %s", e.TargetID)
			}
		}
		if e.Type == "PARENT_OF" {
			foundParentOf++
		}
	}

	if foundImplements != 2 {
		t.Errorf("Expected 2 IMPLEMENTS edges, got %d", foundImplements)
	}
	if foundParentOf != 2 {
		t.Errorf("Expected 2 PARENT_OF edges, got %d", foundParentOf)
	}
}

func TestBuilder_BuildThreeLevel(t *testing.T) {
	// Setup Global Clusterer
	mockGlobal := &MockClusterer{
		ClusterFunc: func(nodes []graph.Node, domain string) (map[string][]graph.Node, error) {
			clusters := make(map[string][]graph.Node)
			clusters["Auth"] = []graph.Node{nodes[0]}
			clusters["Payment"] = []graph.Node{nodes[1]}
			return clusters, nil
		},
	}

	// Setup Category Clusterer
	mockCategoryClusterer := &MockClusterer{
		ClusterFunc: func(nodes []graph.Node, domain string) (map[string][]graph.Node, error) {
			clusters := make(map[string][]graph.Node)
			clusters[domain+"-Cat"] = nodes
			return clusters, nil
		},
	}

	// Setup Feature Clusterer
	mockFeatureClusterer := &MockClusterer{
		ClusterFunc: func(nodes []graph.Node, domain string) (map[string][]graph.Node, error) {
			clusters := make(map[string][]graph.Node)
			clusters[domain+"-Feat"] = nodes
			return clusters, nil
		},
	}

	builder := &Builder{
		GlobalClusterer:   mockGlobal,
		CategoryClusterer: mockCategoryClusterer,
		Clusterer:         mockFeatureClusterer,
	}

	functions := []graph.Node{
		{ID: "func1", Properties: map[string]interface{}{"file": "src/auth/login.go"}},
		{ID: "func2", Properties: map[string]interface{}{"file": "src/payment/charge.go"}},
	}

	features, edges, err := builder.Build("src/", functions)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	nodes, _ := Flatten(features, edges)

	// 2 Domains + 2 Categories + 2 Features = 6 nodes
	if len(nodes) != 6 {
		t.Errorf("Expected 6 feature nodes in 3-level hierarchy, got %d", len(nodes))
	}

	// Verify edge types
	foundParentOf := 0
	foundImplements := 0
	for _, e := range edges {
		switch e.Type {
		case "PARENT_OF":
			foundParentOf++
		case "IMPLEMENTS":
			foundImplements++
		}
	}

	// 2 Domain->Category + 2 Category->Feature = 4 PARENT_OF
	if foundParentOf != 4 {
		t.Errorf("Expected 4 PARENT_OF edges in 3-level hierarchy, got %d", foundParentOf)
	}
	if foundImplements != 2 {
		t.Errorf("Expected 2 IMPLEMENTS edges, got %d", foundImplements)
	}
}

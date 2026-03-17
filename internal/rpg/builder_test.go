package rpg

import (
	"fmt"
	"graphdb/internal/graph"
	"strings"
	"testing"
)

// MockClusterer implements Clusterer for testing
type MockClusterer struct {
	ClusterFunc func(nodes []graph.Node, domain string) ([]ClusterGroup, error)
}

func (m *MockClusterer) Cluster(nodes []graph.Node, domain string) ([]ClusterGroup, error) {
	if m.ClusterFunc != nil {
		return m.ClusterFunc(nodes, domain)
	}
	// Default behavior: return empty
	return nil, nil
}

func TestBuilder_Build(t *testing.T) {
	// Setup Global Clusterer to act as the primary discovery mechanism
	mockGlobal := &MockClusterer{
		ClusterFunc: func(nodes []graph.Node, domain string) ([]ClusterGroup, error) {
			// Simulate finding 2 domains
			clusters := []ClusterGroup{
				{Name: "Auth", Nodes: []graph.Node{nodes[0]}},
				{Name: "Payment", Nodes: []graph.Node{nodes[1]}},
			}
			return clusters, nil
		},
	}

	// Setup Feature Clusterer (2nd level)
	mockFeatureClusterer := &MockClusterer{
		ClusterFunc: func(nodes []graph.Node, domain string) ([]ClusterGroup, error) {
			// One feature per domain for simplicity
			clusters := []ClusterGroup{
				{Name: domain + "-Feature", Nodes: nodes},
			}
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
			if !strings.HasPrefix(e.TargetID, "feature-") {
				t.Errorf("IMPLEMENTS edge TargetID should be a feature ID (feature-*), got %s", e.TargetID)
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


func TestBuilder_ErrorPropagation(t *testing.T) {
	mockGlobal := &MockClusterer{
		ClusterFunc: func(nodes []graph.Node, domain string) ([]ClusterGroup, error) {
			return nil, fmt.Errorf("simulated global clusterer error")
		},
	}
	
	builder := &Builder{
		GlobalClusterer: mockGlobal,
		Clusterer:       &MockClusterer{},
	}
	
	functions := []graph.Node{{ID: "f1"}}
	_, _, err := builder.Build("src/", functions)
	
	if err == nil {
		t.Fatal("Expected error to propagate from builder, got nil")
	}
	if !strings.Contains(err.Error(), "simulated global clusterer error") {
		t.Errorf("Expected error to contain 'simulated global clusterer error', got '%v'", err)
	}
}

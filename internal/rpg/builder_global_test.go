package rpg

import (
	"graphdb/internal/graph"
	"strings"
	"testing"
)

// MockGlobalClusterer for testing Global Discovery Mode
type MockGlobalClusterer struct{}

func (m *MockGlobalClusterer) Cluster(nodes []graph.Node, domain string) ([]ClusterGroup, error) {
	// Returns two clusters with SEMANTIC NAMES: "Auth System" and "Payment Service"
	var clusters []ClusterGroup

	// Filter nodes for AuthGroup
	var authNodes []graph.Node
	for _, n := range nodes {
		if p, ok := n.Properties["file"].(string); ok && (strings.Contains(p, "auth/")) {
			authNodes = append(authNodes, n)
		}
	}
	clusters = append(clusters, ClusterGroup{Name: "Auth System", Nodes: authNodes})

	// Filter nodes for PaymentGroup
	var payNodes []graph.Node
	for _, n := range nodes {
		if p, ok := n.Properties["file"].(string); ok && (strings.Contains(p, "payment/")) {
			payNodes = append(payNodes, n)
		}
	}
	clusters = append(clusters, ClusterGroup{Name: "Payment Service", Nodes: payNodes})

	return clusters, nil
}

func TestBuilder_GlobalBuild(t *testing.T) {
	// Setup
	builder := &Builder{
		GlobalClusterer: &MockGlobalClusterer{},
		// We reuse MockClusterer from builder_test.go (same package)
		// Assuming MockClusterer is defined there.
		// If not, we need to redefine it here or make sure they share the package scope.
		// Since both are package 'rpg', they share it.
		Clusterer: &MockClusterer{},
	}

	// Input: Functions
	functions := []graph.Node{
		{ID: "func1", Properties: map[string]interface{}{"file": "src/auth/login.go", "name": "Login"}},
		{ID: "func2", Properties: map[string]interface{}{"file": "src/auth/logout.go", "name": "Logout"}},
		{ID: "func3", Properties: map[string]interface{}{"file": "src/payment/charge.go", "name": "Charge"}},
		{ID: "func4", Properties: map[string]interface{}{"file": "src/payment/refund.go", "name": "Refund"}},
	}

	// Execute
	// rootPath "src/" is just context for LCA fallback if needed
	features, _, err := builder.Build("src/", functions)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify Structure
	// We expect 2 domains with the SEMANTIC NAMES provided by the clusterer.
	// 1. "Auth System" -> LCA "src/auth"
	// 2. "Payment Service" -> LCA "src/payment"

	if len(features) != 2 {
		t.Errorf("Expected 2 domain features, got %d", len(features))
	}

	foundAuth := false
	foundPayment := false

	for _, f := range features {
		t.Logf("Found feature: ID=%s Name=%s ScopePath=%s", f.ID, f.Name, f.ScopePath)

		// Check for Auth System
		if f.Name == "Auth System" {
			foundAuth = true
			if !strings.HasPrefix(f.ID, "domain-") {
				t.Errorf("Expected ID to start with 'domain-', got '%s'", f.ID)
			}
			if f.ScopePath != "src/auth" {
				t.Errorf("Expected Auth domain ScopePath 'src/auth', got '%s'", f.ScopePath)
			}
		}

		// Check for Payment Service
		if f.Name == "Payment Service" {
			foundPayment = true
			if !strings.HasPrefix(f.ID, "domain-") {
				t.Errorf("Expected ID to start with 'domain-', got '%s'", f.ID)
			}
			if f.ScopePath != "src/payment" {
				t.Errorf("Expected Payment domain ScopePath 'src/payment', got '%s'", f.ScopePath)
			}
		}
	}

	if !foundAuth {
		t.Errorf("Did not find 'Auth System' domain")
	}
	if !foundPayment {
		t.Errorf("Did not find 'Payment Service' domain")
	}
}

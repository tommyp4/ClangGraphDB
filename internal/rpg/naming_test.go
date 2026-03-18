package rpg

import (
	"graphdb/internal/graph"
	"testing"
)

func TestGenerateDomainName(t *testing.T) {
	tests := []struct {
		name     string
		lca      string
		nodes    []graph.Node
		expected string
	}{
		{
			name:     "Specific LCA",
			lca:      "internal/auth",
			nodes:    []graph.Node{},
			expected: "Auth",
		},
		{
			name:     "Specific LCA Deep",
			lca:      "internal/services/payment",
			nodes:    []graph.Node{},
			expected: "Payment",
		},
		{
			name: "Root LCA with Dominant Term",
			lca:  ".",
			nodes: []graph.Node{
				{Properties: map[string]interface{}{"name": "LoginUser"}},
				{Properties: map[string]interface{}{"name": "LogoutUser"}},
				{Properties: map[string]interface{}{"name": "RegisterUser"}},
			},
			expected: "User",
		},
		{
			name: "Root LCA with Dominant Term (Case Insensitive)",
			lca:  ".",
			nodes: []graph.Node{
				{Properties: map[string]interface{}{"name": "processData"}},
				{Properties: map[string]interface{}{"name": "LoadData"}},
				{Properties: map[string]interface{}{"name": "save_data"}},
			},
			expected: "Data",
		},
		{
			name: "Generic LCA with Dominant Term",
			lca:  "internal",
			nodes: []graph.Node{
				{Properties: map[string]interface{}{"name": "ApiHandler"}},
				{Properties: map[string]interface{}{"name": "ApiMiddleware"}},
			},
			expected: "Api",
		},
		{
			name: "Generic LCA with Dominant Term (pkg)",
			lca:  "pkg",
			nodes: []graph.Node{
				{Properties: map[string]interface{}{"name": "Logger"}},
				{Properties: map[string]interface{}{"name": "LogFormatter"}},
				{Properties: map[string]interface{}{"name": "LogWriter"}},
			},
			expected: "Log",
		},
		{
			name: "Fallback Generic UUID",
			lca:  ".",
			nodes: []graph.Node{
				{Properties: map[string]interface{}{"name": "A"}},
				{Properties: map[string]interface{}{"name": "B"}},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateDomainName(tt.lca, tt.nodes)
			if got != tt.expected {
				t.Errorf("GenerateDomainName() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFindDominantTerm_SchemaMismatch_Name(t *testing.T) {
	// Naming fallback uses the 'name' property to find dominant terms if LLM fails.
	// If the schema uses 'func_name' instead, it silently fails and returns nothing.

	nodes := []graph.Node{
		{Properties: map[string]interface{}{"name": "GetUser"}},
		{Properties: map[string]interface{}{"name": "GetAccount"}},
	}

	term := findDominantTerm(nodes)
	if term == "" {
		t.Errorf("Expected 'Get', got empty string. Check name property reading.")
	} else if term != "Get" {
		t.Errorf("Expected 'Get', got '%s'", term)
	}
}

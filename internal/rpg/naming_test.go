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
			expected: "auth",
		},
		{
			name:     "Specific LCA Deep",
			lca:      "internal/services/payment",
			nodes:    []graph.Node{},
			expected: "payment",
		},
		{
			name: "Root LCA with Dominant Term",
			lca:  ".",
			nodes: []graph.Node{
				{Properties: map[string]interface{}{"name": "LoginUser"}},
				{Properties: map[string]interface{}{"name": "LogoutUser"}},
				{Properties: map[string]interface{}{"name": "RegisterUser"}},
			},
			expected: "user",
		},
		{
			name: "Root LCA with Dominant Term (Case Insensitive)",
			lca:  ".",
			nodes: []graph.Node{
				{Properties: map[string]interface{}{"name": "processData"}},
				{Properties: map[string]interface{}{"name": "LoadData"}},
				{Properties: map[string]interface{}{"name": "save_data"}},
			},
			expected: "data",
		},
		{
			name: "Generic LCA with Dominant Term",
			lca:  "internal",
			nodes: []graph.Node{
				{Properties: map[string]interface{}{"name": "ApiHandler"}},
				{Properties: map[string]interface{}{"name": "ApiMiddleware"}},
			},
			expected: "api",
		},
		{
			name: "Generic LCA with Dominant Term (pkg)",
			lca:  "pkg",
			nodes: []graph.Node{
				{Properties: map[string]interface{}{"name": "Logger"}},
				{Properties: map[string]interface{}{"name": "LogFormatter"}},
				{Properties: map[string]interface{}{"name": "LogWriter"}},
			},
			expected: "log",
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

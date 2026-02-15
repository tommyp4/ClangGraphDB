package rpg

import (
	"graphdb/internal/graph"
	"strings"
	"testing"
)

func TestGenerateDomainName(t *testing.T) {
	tests := []struct {
		name     string
		lca      string
		nodes    []graph.Node
		expected string // We might need partial matching for UUIDs
	}{
		{
			name:     "Specific LCA",
			lca:      "internal/auth",
			nodes:    []graph.Node{},
			expected: "domain-auth",
		},
		{
			name:     "Specific LCA Deep",
			lca:      "internal/services/payment",
			nodes:    []graph.Node{},
			expected: "domain-payment",
		},
		{
			name: "Root LCA with Dominant Term",
			lca:  ".",
			nodes: []graph.Node{
				{Properties: map[string]interface{}{"name": "LoginUser"}},
				{Properties: map[string]interface{}{"name": "LogoutUser"}},
				{Properties: map[string]interface{}{"name": "RegisterUser"}},
			},
			expected: "domain-user",
		},
		{
			name: "Root LCA with Dominant Term (Case Insensitive)",
			lca:  ".",
			nodes: []graph.Node{
				{Properties: map[string]interface{}{"name": "processData"}},
				{Properties: map[string]interface{}{"name": "LoadData"}},
				{Properties: map[string]interface{}{"name": "save_data"}},
			},
			expected: "domain-data",
		},
		{
			name: "Generic LCA with Dominant Term",
			lca:  "internal",
			nodes: []graph.Node{
				{Properties: map[string]interface{}{"name": "ApiHandler"}},
				{Properties: map[string]interface{}{"name": "ApiMiddleware"}},
			},
			expected: "domain-api",
		},
		{
			name: "Generic LCA with Dominant Term (pkg)",
			lca:  "pkg",
			nodes: []graph.Node{
				{Properties: map[string]interface{}{"name": "Logger"}},
				{Properties: map[string]interface{}{"name": "LogFormatter"}},
				{Properties: map[string]interface{}{"name": "LogWriter"}},
			},
			expected: "domain-log", // "logger" might be split or stemmed? sticking to simple for now. 
            // "Logger" -> "Logger"
            // "LogFormatter" -> "Log", "Formatter"
			// "LogWriter" -> "Log", "Writer"
			// Log appears twice.
		},
        {
			name: "Fallback Generic UUID",
			lca:  ".",
			nodes: []graph.Node{
				{Properties: map[string]interface{}{"name": "A"}},
				{Properties: map[string]interface{}{"name": "B"}},
			},
			expected: "domain-generic-", // Prefix match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateDomainName(tt.lca, tt.nodes)
			if strings.HasPrefix(tt.expected, "domain-generic-") {
				if !strings.HasPrefix(got, "domain-generic-") {
					t.Errorf("GenerateDomainName() = %v, expected prefix %v", got, tt.expected)
				}
			} else {
				if got != tt.expected {
					t.Errorf("GenerateDomainName() = %v, want %v", got, tt.expected)
				}
			}
		})
	}
}

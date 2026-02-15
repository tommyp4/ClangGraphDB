package rpg

import (
	"graphdb/internal/graph"
	"testing"
)

func TestNodeToText(t *testing.T) {
	tests := []struct {
		name     string
		node     graph.Node
		expected string
	}{
		{
			name: "With Atomic Features (Slice)",
			node: graph.Node{
				ID: "test-1",
				Properties: map[string]interface{}{
					"atomic_features": []string{"feature1", "feature2"},
				},
			},
			expected: "feature1, feature2",
		},
		{
			name: "With Atomic Features (Interface Slice)",
			node: graph.Node{
				ID: "test-2",
				Properties: map[string]interface{}{
					"atomic_features": []interface{}{"feature1", "feature2"},
				},
			},
			expected: "feature1, feature2",
		},
		{
			name: "Fallback to Name",
			node: graph.Node{
				ID: "test-3",
				Properties: map[string]interface{}{
					"name": "MyFunction",
				},
			},
			expected: "MyFunction",
		},
		{
			name: "Fallback to ID",
			node: graph.Node{
				ID: "test-4",
			},
			expected: "test-4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NodeToText(tt.node); got != tt.expected {
				t.Errorf("NodeToText() = %v, want %v", got, tt.expected)
			}
		})
	}
}

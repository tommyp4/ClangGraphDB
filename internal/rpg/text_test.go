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
			name: "All Signals Present",
			node: graph.Node{
				ID: "test-1",
				Properties: map[string]interface{}{
					"file":            "src/payment/processor.go",
					"name":            "ProcessPayment",
					"atomic_features": []string{"create payment", "validate amount"},
				},
			},
			expected: "src/payment/processor.go | ProcessPayment | create payment, validate amount",
		},
		{
			name: "File and Name Only",
			node: graph.Node{
				ID: "test-2",
				Properties: map[string]interface{}{
					"file": "src/user/service.go",
					"name": "CreateUser",
				},
			},
			expected: "src/user/service.go | CreateUser",
		},
		{
			name: "Atomic Features (Slice) Only",
			node: graph.Node{
				ID: "test-3",
				Properties: map[string]interface{}{
					"atomic_features": []string{"feature1", "feature2"},
				},
			},
			expected: "feature1, feature2",
		},
		{
			name: "Atomic Features (Interface Slice) Only",
			node: graph.Node{
				ID: "test-4",
				Properties: map[string]interface{}{
					"atomic_features": []interface{}{"feature1", "feature2"},
				},
			},
			expected: "feature1, feature2",
		},
		{
			name: "Name Only",
			node: graph.Node{
				ID: "test-5",
				Properties: map[string]interface{}{
					"name": "MyFunction",
				},
			},
			expected: "MyFunction",
		},
		{
			name: "File and Atomic Features",
			node: graph.Node{
				ID: "test-6",
				Properties: map[string]interface{}{
					"file":            "utils/helpers.go",
					"atomic_features": []string{"helper_func"},
				},
			},
			expected: "utils/helpers.go | helper_func",
		},
		{
			name: "Meaningless File Path",
			node: graph.Node{
				ID: "test-7",
				Properties: map[string]interface{}{
					"file":            "utils/common.go",
					"name":            "ProcessPayment",
					"atomic_features": []string{"create payment", "validate amount"},
				},
			},
			expected: "utils/common.go | ProcessPayment | create payment, validate amount",
		},
		{
			name: "ID Fallback",
			node: graph.Node{
				ID: "test-8",
			},
			expected: "test-8",
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

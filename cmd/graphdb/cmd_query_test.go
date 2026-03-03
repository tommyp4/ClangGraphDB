//go:build test_mocks

package main

import (
	"context"
	"os"
	"testing"
)

// Tests for CLI queries would go here.
// Semantic Seams CLI tests removed and will be added back in Task 4.3 when the CLI is wired.

func TestHandleQuery_Basic(t *testing.T) {
	// 1. Setup Environment for Mocking
	os.Setenv("GRAPHDB_MOCK_ENABLED", "true")
	os.Setenv("NEO4J_URI", "bolt://localhost:7687") // Just to pass the check
	defer os.Unsetenv("GRAPHDB_MOCK_ENABLED")
	defer os.Unsetenv("NEO4J_URI")

	// 2. Call handleQuery with status
	args := []string{"-type", "status"}
	
	// This should not panic or exit if mocking is working
	handleQuery(args)
}

func TestMockProvider_GetSemanticSeams(t *testing.T) {
	// Task 4.1 requirement: verify the mock provider can execute and return results for semantic seam detection.
	mock := &MockProvider{}
	ctx := context.Background()
	threshold := 0.5

	results, err := mock.GetSemanticSeams(ctx, threshold)
	if err != nil {
		t.Fatalf("Expected no error from mock, got %v", err)
	}

	if !mock.GetSemanticSeamsCalled {
		t.Errorf("Expected GetSemanticSeamsCalled to be true")
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result from mock, got %d", len(results))
	}

	if results[0].Container != "mock_file.go" {
		t.Errorf("Expected container 'mock_file.go', got '%s'", results[0].Container)
	}

	if results[0].Similarity != 0.1 {
		t.Errorf("Expected similarity 0.1, got %f", results[0].Similarity)
	}
}

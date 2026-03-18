package rpg

import (
	"testing"
)

func TestMockFeatureExtractor_Extract(t *testing.T) {
	extractor := &MockFeatureExtractor{}

	descriptors, isVolatile, err := extractor.Extract("func login() { validate(); hash(); }", "login")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(descriptors) != 2 {
		t.Fatalf("Expected 2 descriptors, got %d", len(descriptors))
	}
	if descriptors[0] != "data processing" {
		t.Errorf("Expected 'data processing', got '%s'", descriptors[0])
	}
	if descriptors[1] != "input validation" {
		t.Errorf("Expected 'input validation', got '%s'", descriptors[1])
	}
	if isVolatile {
		t.Errorf("Expected isVolatile to be false")
	}
}

func TestMockFeatureExtractor_EmptyCode(t *testing.T) {
	extractor := &MockFeatureExtractor{}

	descriptors, isVolatile, err := extractor.Extract("", "empty")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// MockFeatureExtractor returns fixed values regardless of input
	if len(descriptors) != 2 {
		t.Fatalf("Expected 2 descriptors from mock, got %d", len(descriptors))
	}
	if isVolatile {
		t.Errorf("Expected isVolatile to be false")
	}
}

func TestFeatureExtractorInterface(t *testing.T) {
	// Verify both implementations satisfy the interface
	var _ FeatureExtractor = &MockFeatureExtractor{}
	var _ FeatureExtractor = &LLMFeatureExtractor{}
}

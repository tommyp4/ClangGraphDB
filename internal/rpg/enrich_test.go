package rpg

import (
	"fmt"
	"graphdb/internal/graph"
	"strings"
	"testing"
)

type MockSummarizer struct {
	SummarizeFunc func(snippets []string, level string) (string, string, error)
}

func (m *MockSummarizer) Summarize(snippets []string, level string, extraContext string) (string, string, error) {
	if m.SummarizeFunc != nil {
		return m.SummarizeFunc(snippets, level)
	}

	if len(snippets) == 0 {
		return "Feature-" + GenerateShortUUID(), "No code snippets provided for analysis.", nil
	}
	// Verify that we got the content we expected
	foundLogin := false
	foundVerify := false
	for _, s := range snippets {
		if s == "func login() { ... }" {
			foundLogin = true
		}
		if s == "func verify() { ... }" {
			foundVerify = true
		}
	}
	if !foundLogin && !foundVerify {
		return "Unknown", "Content missing", nil
	}

	return "User Login", "Handles authentication verification", nil
}

type MockEmbedder struct{}

func (m *MockEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	res := make([][]float32, len(texts))
	for i := range texts {
		res[i] = make([]float32, 768)
		res[i][0] = 0.42 // Sentinel value for testing
	}
	return res, nil
}

func mockLoader(path string, start, end int) (string, error) {
	if path == "auth.go" && start == 10 && end == 20 {
		return "func login() { ... }", nil
	}
	if path == "auth.go" && start == 30 && end == 40 {
		return "func verify() { ... }", nil
	}
	return "", fmt.Errorf("file not found or range invalid: %s %d-%d", path, start, end)
}

func TestEnricher_Enrich(t *testing.T) {
	enricher := &Enricher{
		Client:   &MockSummarizer{},
		Embedder: &MockEmbedder{},
		Loader:   mockLoader,
	}

	feature := &Feature{
		ID:   "feat-temp",
		Name: "Cluster-001",
	}

	functions := []graph.Node{
		{Properties: map[string]interface{}{
			"file":     "auth.go",
			"start_line":     10,
			"end_line": 20,
		}},
		{Properties: map[string]interface{}{
			"file":     "auth.go",
			"start_line":     30,
			"end_line": 40,
		}},
	}

	err := enricher.Enrich(feature, functions, "Feature")
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if feature.Name != "User Login" {
		t.Errorf("Expected name 'User Login', got '%s'", feature.Name)
	}
	if feature.Description != "Handles authentication verification" {
		t.Errorf("Expected description to match mock, got '%s'", feature.Description)
	}
	if feature.Embedding == nil {
		t.Fatal("Expected Embedding to be non-nil after Enrich")
	}
	if len(feature.Embedding) != 768 {
		t.Errorf("Expected 768-dim embedding, got %d", len(feature.Embedding))
	}
	if feature.Embedding[0] != 0.42 {
		t.Errorf("Expected sentinel value 0.42 in embedding[0], got %f", feature.Embedding[0])
	}
}

func TestEnricher_Enrich_NilEmbedder(t *testing.T) {
	enricher := &Enricher{
		Client: &MockSummarizer{},
		Loader: mockLoader,
		// Embedder is nil -- should still work, just no embedding
	}

	feature := &Feature{
		ID:   "feat-temp",
		Name: "Cluster-001",
	}

	functions := []graph.Node{
		{Properties: map[string]interface{}{
			"file":     "auth.go",
			"start_line":     10,
			"end_line": 20,
		}},
	}

	err := enricher.Enrich(feature, functions, "Feature")
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if feature.Name != "User Login" {
		t.Errorf("Expected name 'User Login', got '%s'", feature.Name)
	}
	if feature.Embedding != nil {
		t.Errorf("Expected nil embedding when embedder is nil, got %v", feature.Embedding)
	}
}

func TestEnricher_Enrich_MissingProps(t *testing.T) {
	enricher := &Enricher{
		Client: &MockSummarizer{},
		Loader: mockLoader,
	}
	feature := &Feature{ID: "feat-temp", Name: "Cluster-001"}

	// Missing file/line props should result in empty snippets, handled gracefully
	functions := []graph.Node{
		{Properties: map[string]interface{}{"other": "val"}},
	}

	err := enricher.Enrich(feature, functions, "Feature")
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}
	// Should be unknown feature because snippets are empty
	if !strings.HasPrefix(feature.Name, "Feature-") {
		t.Errorf("Expected name starting with 'Feature-', got '%s'", feature.Name)
	}
}

func TestEnricher_Enrich_Float64Props(t *testing.T) {
	enricher := &Enricher{
		Client:   &MockSummarizer{},
		Embedder: &MockEmbedder{},
		Loader:   mockLoader,
	}

	feature := &Feature{
		ID:   "feat-temp",
		Name: "Cluster-001",
	}

	functions := []graph.Node{
		{Properties: map[string]interface{}{
			"file":     "auth.go",
			"start_line":     float64(10), // As if from JSON
			"end_line": float64(20),
		}},
	}

	err := enricher.Enrich(feature, functions, "Feature")
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}
	// If loader was called, name will be User Login. If not, Unknown.
	if feature.Name != "User Login" {
		t.Errorf("Expected name 'User Login', got '%s'", feature.Name)
	}
}

func TestEnricher_Enrich_SchemaMismatch(t *testing.T) {
	// Tests that the enricher fails gracefully (or falls back to atomic features)
	// when the node has 'start_line' instead of the expected 'line' (schema mismatch).

	var loaderCalled bool
	var passedSnippets []string

	mockLoader := func(path string, start, end int) (string, error) {
		loaderCalled = true
		return "func login() { ... }", nil
	}

	summarizer := &MockSummarizer{
		SummarizeFunc: func(snippets []string, level string) (string, string, error) {
			passedSnippets = append(passedSnippets, snippets...)
			return "Named Feature", "Desc", nil
		},
	}

	enricher := &Enricher{
		Client: summarizer,
		Loader: mockLoader,
	}

	feature := &Feature{ID: "feat-temp", Name: "Cluster-001"}

	// Note 'start_line' instead of 'line'
	functions := []graph.Node{
		{Properties: map[string]interface{}{
			"file":            "auth.go",
			"start_line":            10, 
			"end_line":        20,
			"atomic_features": []string{"fallback-feature"},
		}},
	}

	err := enricher.Enrich(feature, functions, "Feature")
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if !loaderCalled {
		t.Errorf("Expected snippet loader to be called using 'start_line' property, but it was skipped.")
	}
}

package e2e

import (
	"graphdb/internal/graph"
	"graphdb/internal/rpg"
	"graphdb/internal/tools/snippet"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MockSummarizer captures snippets and returns fixed output
type MockSummarizer struct {
	CapturedSnippets []string
}

func (m *MockSummarizer) Summarize(snippets []string, level string, extraContext string) (string, string, error) {
	m.CapturedSnippets = snippets
	return "Mock Feature", "This is a mock description.", nil
}

func TestEnricher_Integration_RealFile(t *testing.T) {
	// 1. Create a temporary file to read
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	content := "package main\n\nfunc HelloWorld() {\n\tprintln(\"Hello\")\n}\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Create a Function Node pointing to this file
	// Line 3 is "func HelloWorld() {", Line 5 is "}"
	fn := graph.Node{
		ID:    "test:HelloWorld",
		Label: "Function",
		Properties: map[string]interface{}{
			"name":     "HelloWorld",
			"file":     filePath,
			"start_line":     3,
			"end_line": 5,
		},
	}

	// 3. Setup Enricher with Real Loader and Mock Summarizer
	m := &MockSummarizer{}
	enricher := &rpg.Enricher{
		Client: m,
		Loader: snippet.SliceFile, // USE REAL LOADER
	}

	feature := &rpg.Feature{
		Name: "Unknown",
	}

	// 4. Run Enrich
	if err := enricher.Enrich(feature, []graph.Node{fn}, "Feature"); err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// 5. Verify Results
	// Check feature name/desc
	if feature.Name != "Mock Feature" {
		t.Errorf("Expected 'Mock Feature', got '%s'", feature.Name)
	}

	// Check that snippets were loaded!
	if len(m.CapturedSnippets) == 0 {
		t.Fatal("Summarizer received no snippets! Loader failed.")
	}

	// Verify snippet content
	snip := m.CapturedSnippets[0]
	expected := "func HelloWorld() {\n\tprintln(\"Hello\")\n}"
	if !strings.Contains(snip, expected) {
		t.Errorf("Snippet content mismatch.\nGot:\n%s\nExpected to contain:\n%s", snip, expected)
	}
}

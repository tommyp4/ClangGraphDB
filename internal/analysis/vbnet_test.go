package analysis_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"graphdb/internal/analysis"
)

func TestParseVBNet(t *testing.T) {
	// 1. Verify Parser Registration
	parser, ok := analysis.GetParser(".vb")
	if !ok {
		t.Fatalf("VB.NET parser not registered")
	}

	// 2. Load Fixture
	absPath, err := filepath.Abs("../../test/fixtures/vbnet/sample.vb")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	// 3. Parse
	nodes, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// 4. Assertions
	foundGreeter := false
	foundGreet := false
	foundCalculate := false

	for _, n := range nodes {
		name, _ := n.Properties["name"].(string)

		// Check File Node for NO content
		if n.Label == "File" {
			if _, hasContent := n.Properties["content"]; hasContent {
				t.Errorf("File node '%s' should NOT have 'content' property", n.ID)
			}
		}

		if n.Label == "Class" && name == "Greeter" {
			foundGreeter = true
		}
		if n.Label == "Function" && name == "Greet" {
			foundGreet = true
			if _, ok := n.Properties["end_line"]; !ok {
				t.Errorf("Function 'Greet' missing end_line")
			}
		}
		if n.Label == "Function" && name == "Calculate" {
			foundCalculate = true
			if _, ok := n.Properties["end_line"]; !ok {
				t.Errorf("Function 'Calculate' missing end_line")
			}
		}
	}

	if !foundGreeter {
		t.Errorf("Expected Class 'Greeter' not found")
	}
	if !foundGreet {
		t.Errorf("Expected Function 'Greet' not found")
	}
	if !foundCalculate {
		t.Errorf("Expected Function 'Calculate' not found")
	}

	// 5. Check Call Edge
	foundCall := false
	for _, e := range edges {
		// Source: ...:Greet, Target: ...:Calculate
		if strings.HasSuffix(e.SourceID, ":Greet") && strings.HasSuffix(e.TargetID, ":Calculate") {
			foundCall = true
			break
		}
	}
	if !foundCall {
		t.Errorf("Expected Call Edge from Greet to Calculate not found")
	}
}

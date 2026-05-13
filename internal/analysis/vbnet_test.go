package analysis_test

import (
	"os"
	"path/filepath"
	"testing"

	"clang-graphdb/internal/analysis"
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
            // ID should be "Greeter" (no path prefix)
            if n.ID != "Greeter" {
                t.Errorf("Expected Class ID 'Greeter', got '%s'", n.ID)
            }
		}
		if n.Label == "Function" && name == "Greet" {
			foundGreet = true
            if n.ID != "Greeter.Greet" {
                t.Errorf("Expected Function ID 'Greeter.Greet', got '%s'", n.ID)
            }
			if _, ok := n.Properties["end_line"]; !ok {
				t.Errorf("Function 'Greet' missing end_line")
			}
		}
		if n.Label == "Function" && name == "Calculate" {
			foundCalculate = true
             if n.ID != "Greeter.Calculate" {
                t.Errorf("Expected Function ID 'Greeter.Calculate', got '%s'", n.ID)
            }
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
		// Source: Greeter.Greet
        // Target: Calculate (simple name resolution)
		if e.SourceID == "Greeter.Greet" && e.TargetID == "Calculate" {
			foundCall = true
			break
		}
	}
	if !foundCall {
        t.Log("Edges found:")
        for _, e := range edges {
            t.Logf("  %s -> %s", e.SourceID, e.TargetID)
        }
		t.Errorf("Expected Call Edge from Greeter.Greet to Calculate not found")
	}
}

package analysis_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"graphdb/internal/analysis"
)

func TestParseSQL(t *testing.T) {
	// 1. Setup
	absPath, err := filepath.Abs("../../test/fixtures/sql/sample.sql")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	content := []byte(`
CREATE FUNCTION CalculateTotal() RETURNS INT AS $$
BEGIN
    RETURN 100;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION ProcessOrder() RETURNS VOID AS $$
BEGIN
    SELECT CalculateTotal();
END;
$$ LANGUAGE plpgsql;
`)

	// 2. Get Parser
	parser, ok := analysis.GetParser(".sql")
	if !ok {
		t.Fatalf("SQL parser not registered")
	}

	// 3. Parse
	nodes, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// 4. Verification
	foundCalculateTotal := false
	foundProcessOrder := false

			for _, n := range nodes {
				name, _ := n.Properties["name"].(string)
				fqn, fqnOk := n.Properties["fqn"].(string)
				expectedFQN := fmt.Sprintf("%s:%s", absPath, name)
				
				if !fqnOk || fqn != expectedFQN {
					t.Errorf("Expected fqn %s, got %v", expectedFQN, n.Properties["fqn"])
				}
	
				if name == "CalculateTotal" && n.Label == "Function" {
					foundCalculateTotal = true
					if _, ok := n.Properties["end_line"]; !ok {
						t.Errorf("Function 'CalculateTotal' missing end_line")
					}
					startLine, _ := n.Properties["start_line"].(uint32)
					endLine, _ := n.Properties["end_line"].(uint32)
					if startLine == endLine {
						t.Errorf("Function 'CalculateTotal' should span multiple lines, got start_line=%d end_line=%d", startLine, endLine)
					}
					if _, ok := n.Properties["content"]; ok {
						t.Errorf("Function 'CalculateTotal' should not have content")
					}
				}
				if name == "ProcessOrder" && n.Label == "Function" { // Procedure treated as Function
					foundProcessOrder = true
					if _, ok := n.Properties["end_line"]; !ok {
						t.Errorf("Function 'ProcessOrder' missing end_line")
					}
					startLine, _ := n.Properties["start_line"].(uint32)
					endLine, _ := n.Properties["end_line"].(uint32)
					if startLine == endLine {
						t.Errorf("Function 'ProcessOrder' should span multiple lines, got start_line=%d end_line=%d", startLine, endLine)
					}
					if _, ok := n.Properties["content"]; ok {
						t.Errorf("Function 'ProcessOrder' should not have content")
					}
				}
			}
	
			if !foundCalculateTotal {
				t.Errorf("Expected Function 'CalculateTotal' not found")
			}
			if !foundProcessOrder {
				t.Errorf("Expected Function/Procedure 'ProcessOrder' not found")
			}
	
			// Helper to find edge
			hasEdge := func(srcName, tgtName string) bool {
				srcFQN := fmt.Sprintf("%s:%s", absPath, srcName)
				srcID := analysis.GenerateNodeID("Function", srcFQN, "")
				tgtFQN := fmt.Sprintf("%s:%s", absPath, tgtName)
	
				for _, e := range edges {
					if e.SourceID == srcID && e.TargetID == tgtFQN {
						return true
					}
				}
				return false
			}
	if !hasEdge("ProcessOrder", "CalculateTotal") {
        // Debug
        t.Log("Edges found:")
        for _, e := range edges {
            t.Logf("  %s -> %s", e.SourceID, e.TargetID)
        }
		t.Errorf("Expected Call Edge ProcessOrder -> CalculateTotal not found")
	}
}

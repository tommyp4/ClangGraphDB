package analysis_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"graphdb/internal/analysis"
)

func TestParseJava(t *testing.T) {
	parser, ok := analysis.GetParser(".java")
	if !ok {
		t.Skip("Java parser not registered (yet)")
	}

	absPath, err := filepath.Abs("../../test/fixtures/java/sample.java")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	nodes, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Helper to find node by name and label
	findNode := func(name, label string) bool {
		for _, n := range nodes {
			nName, _ := n.Properties["name"].(string)
			if nName == name && n.Label == label {
				if _, ok := n.Properties["end_line"]; !ok {
					t.Errorf("Node '%s' (%s) missing end_line", name, label)
				}
				if label == "Function" || label == "Class" || label == "Interface" || label == "Constructor" {
					startLine, _ := n.Properties["start_line"].(int)
					endLine, _ := n.Properties["end_line"].(int)
					if startLine == endLine {
						t.Errorf("Node '%s' (%s) should span multiple lines, got start_line=%d end_line=%d", name, label, startLine, endLine)
					}
				}
				if _, ok := n.Properties["content"]; ok {
					t.Errorf("Node '%s' (%s) should not have content", name, label)
				}
				return true
			}
		}
		return false
	}

	// 1. Verify Structure
	if !findNode("Sample", "Class") {
		t.Errorf("Expected Class 'Sample' not found")
	}
	if !findNode("Base", "Class") {
		t.Errorf("Expected Class 'Base' not found")
	}
	if !findNode("Worker", "Interface") {
		t.Errorf("Expected Interface 'Worker' not found")
	}
	if !findNode("items", "Field") {
		t.Errorf("Expected Field 'items' not found")
	}
	if !findNode("helper", "Field") {
		t.Errorf("Expected Field 'helper' not found")
	}

	// 2. Verify Inheritance / Implementation Edges
	foundExtends := false
	foundImplements := false

	for _, e := range edges {
		// We expect Sample -> Base (EXTENDS)
		if strings.Contains(e.SourceID, "Sample") && strings.Contains(e.TargetID, "Base") && e.Type == "EXTENDS" {
			foundExtends = true
		}
		// We expect Sample -> Worker (IMPLEMENTS)
		if strings.Contains(e.SourceID, "Sample") && strings.Contains(e.TargetID, "Worker") && e.Type == "IMPLEMENTS" {
			foundImplements = true
		}
	}

	if !foundExtends {
		t.Errorf("Expected EXTENDS edge from Sample to Base not found")
	}
	if !foundImplements {
		t.Errorf("Expected IMPLEMENTS edge from Sample to Worker not found")
	}

	// 3. Verify Import Resolution / Uses
    // Check call to helper.doWork() -> should link to Worker:doWork
    foundWorkerCall := false
    // Check call to items.size() -> should link to java.util.List:size
    foundListCall := false

    for _, e := range edges {
        if e.Type == "CALLS" {
            // Check TargetID contains "Worker.doWork" (ignoring full package prefix issues for now, just substring)
            if strings.Contains(e.TargetID, "Worker.doWork") {
                foundWorkerCall = true
            }
            if strings.Contains(e.TargetID, "java.util.List.size") {
                foundListCall = true
            }
        }
    }

    if !foundWorkerCall {
        t.Errorf("Expected CALLS edge to Worker.doWork not found")
    }
    if !foundListCall {
        t.Errorf("Expected CALLS edge to java.util.List.size not found")
    }
}

func TestJavaIDCollision(t *testing.T) {
	parser, ok := analysis.GetParser(".java")
	if !ok {
		t.Skip("Java parser not registered (yet)")
	}

	absPath, err := filepath.Abs("../../test/fixtures/java/IDCollisionTest.java")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	nodes, _, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// 1. Verify IDs are unique
	ids := make(map[string]bool)
	for _, n := range nodes {
		if ids[n.ID] {
			t.Errorf("Duplicate ID found: %s", n.ID)
		}
		ids[n.ID] = true
	}

	// 2. Verify fqn property exists and is correct
	for _, n := range nodes {
		fqn, ok := n.Properties["fqn"].(string)
		if !ok || fqn == "" {
			t.Errorf("Node %s missing fqn property", n.ID)
		}
		if strings.Contains(fqn, "IDCollisionTest.java") {
			t.Errorf("FQN should not contain file path: %s", fqn)
		}
	}

	// 3. Verify specific node IDs
	expectedIDs := []string{
		"Class:com.example.CollisionTest:",
		"Field:com.example.CollisionTest.process:",
		"Constructor:com.example.CollisionTest.CollisionTest:()",
		"Constructor:com.example.CollisionTest.CollisionTest:(int)",
		"Function:com.example.CollisionTest.process:()",
		"Function:com.example.CollisionTest.process:(int,String)",
	}

	for _, expectedID := range expectedIDs {
		if !ids[expectedID] {
			t.Errorf("Expected ID not found: %s", expectedID)
		}
	}
}

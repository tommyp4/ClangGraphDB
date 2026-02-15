package analysis_test

import (
	"path/filepath"
	"strings"
	"testing"

	"graphdb/internal/analysis"
)

func TestParseCSharp(t *testing.T) {
	parser, ok := analysis.GetParser(".cs")
	if !ok {
		t.Fatalf("CSharp parser not registered")
	}

	absPath, err := filepath.Abs("../../test/fixtures/csharp/sample.cs")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	content := []byte(`using System;
public class Greeter {
    public void Greet(string name) {
        Console.WriteLine("Hello " + name);
    }
}`)

	nodes, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	foundGreet := false
	foundGreeter := false

	for _, n := range nodes {
		name, _ := n.Properties["name"].(string)
		if name == "Greet" && n.Label == "Function" {
			foundGreet = true
		}
		if name == "Greeter" && n.Label == "Class" {
			foundGreeter = true
		}
	}

	if !foundGreeter {
		t.Errorf("Expected Class 'Greeter' not found")
	}
	if !foundGreet {
		t.Errorf("Expected Function 'Greet' not found")
	}

	// Verify Call Edge
	foundCall := false
	for _, e := range edges {
		// Source: ...:Greet
		// Target: WriteLine OR System.WriteLine (Resolution candidates)
		// Old behavior was ...:WriteLine. New behavior is logical ID.
		if strings.HasSuffix(e.SourceID, "Greet") && (strings.HasSuffix(e.TargetID, "WriteLine") || e.TargetID == "WriteLine") {
			foundCall = true
			break
		}
	}
	if !foundCall {
		t.Errorf("Expected Call Edge from Greet to WriteLine not found")
	}
}

func TestParseCSharp_ClassAndConstructor(t *testing.T) {
	parser, ok := analysis.GetParser(".cs")
	if !ok {
		t.Fatalf("CSharp parser not registered")
	}

	absPath, err := filepath.Abs("dummy_collision.cs")
	content := []byte(`
namespace MyApp.Core;

public class User {
    public User() { }
    public void Save() { }
}

public class Order {
    public Order() { }
    public void Save() { }
}
`)

	nodes, _, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ids := make(map[string]int)
	for _, n := range nodes {
		ids[n.ID]++
	}

	for id, count := range ids {
		if count > 1 {
			t.Errorf("Duplicate ID found: %s (Count: %d)", id, count)
		}
	}

	// Expected specific IDs (Qualified with namespace and class)
	expectedIDs := []string{
		absPath + ":MyApp.Core.User",
		absPath + ":MyApp.Core.User.User", // Constructor
		absPath + ":MyApp.Core.User.Save",
		absPath + ":MyApp.Core.Order",
		absPath + ":MyApp.Core.Order.Order", // Constructor
		absPath + ":MyApp.Core.Order.Save",
	}

	for _, expected := range expectedIDs {
		if _, exists := ids[expected]; !exists {
			t.Errorf("Expected ID not found: %s", expected)
		}
	}
}

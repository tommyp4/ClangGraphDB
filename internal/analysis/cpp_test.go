package analysis_test

import (
	"path/filepath"
	"strings"
	"testing"

	"graphdb/internal/analysis"
)

func TestParseCPP(t *testing.T) {
	parser, ok := analysis.GetParser(".cpp")
	if !ok {
		t.Fatalf("CPP parser not registered")
	}

	absPath, err := filepath.Abs("../../test/fixtures/cpp/sample.cpp")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	content := []byte(`#include <iostream>
void hello() {
    std::cout << "Hello";
}
class Greeter {
public:
    void greet() { hello(); }
};`)

	nodes, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	foundHello := false
	foundGreet := false

	for _, n := range nodes {
		name, _ := n.Properties["name"].(string)
		if name == "hello" && n.Label == "Function" {
			foundHello = true
			if _, ok := n.Properties["end_line"]; !ok {
				t.Errorf("Function 'hello' missing end_line")
			}
			if _, ok := n.Properties["content"]; ok {
				t.Errorf("Function 'hello' should not have content")
			}
		}
		if name == "greet" && n.Label == "Function" {
			foundGreet = true
			if _, ok := n.Properties["end_line"]; !ok {
				t.Errorf("Function 'greet' missing end_line")
			}
			if _, ok := n.Properties["content"]; ok {
				t.Errorf("Function 'greet' should not have content")
			}
		}
	}

	if !foundHello {
		t.Errorf("Expected Function 'hello' not found")
	}
	if !foundGreet {
		t.Errorf("Expected Function 'greet' not found")
	}

	// Helper to find edge
	hasEdge := func(srcName, tgtName string) bool {
		for _, e := range edges {
			if strings.Contains(e.SourceID, srcName) && strings.Contains(e.TargetID, tgtName) {
				return true
			}
		}
		return false
	}

	if !hasEdge("greet", "hello") {
		t.Errorf("Expected Call Edge greet -> hello not found")
	}
}

func TestParseCPP_Resolution(t *testing.T) {
	parser, ok := analysis.GetParser(".cpp")
	if !ok {
		t.Fatalf("CPP parser not registered")
	}

	absPath, err := filepath.Abs("../../test/fixtures/cpp/sample.cpp")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}
	
	content := []byte(`#include "math.h"
#include <iostream>

int global_counter = 0;

class Base {
public:
    int id;
};

class Derived : public Base {
public:
    void doWork() {
        global_counter++;
        id = 100;
        int result = Math::Add(5, 10);
    }
};

void main() {
    Derived d;
    d.doWork();
}`)

	nodes, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// 1. Check Inheritance
	hasInheritance := false
	for _, e := range edges {
		// IDs are FQN now: Derived, Base
		if strings.Contains(e.SourceID, "Derived") && strings.Contains(e.TargetID, "Base") && e.Type == "INHERITS" {
			hasInheritance = true
			break
		}
	}
	if !hasInheritance {
		t.Errorf("Expected INHERITS edge Derived -> Base")
	}

	// 2. Check Global
	hasGlobal := false
	for _, n := range nodes {
		if n.Label == "Global" && n.Properties["name"] == "global_counter" {
			hasGlobal = true
			break
		}
	}
	if !hasGlobal {
		t.Errorf("Expected Global node 'global_counter'")
	}

	// 3. Check Field
	hasField := false
	for _, n := range nodes {
		if n.Label == "Field" && n.Properties["name"] == "id" {
			hasField = true
			break
		}
	}
	if !hasField {
		t.Errorf("Expected Field node 'id'")
	}

	// 4. Check Usage of Global
	hasGlobalUsage := false
	for _, e := range edges {
		if strings.Contains(e.SourceID, "doWork") && strings.Contains(e.TargetID, "global_counter") && e.Type == "USES" {
			hasGlobalUsage = true
			break
		}
	}
	if !hasGlobalUsage {
		t.Logf("Edges found:")
		for _, e := range edges {
			t.Logf("  %s -> %s (%s)", e.SourceID, e.TargetID, e.Type)
		}
		t.Errorf("Expected USES edge doWork -> global_counter")
	}

	// 5. Check Include Resolution (Math::Add -> math.h)
	// We expect Math::Add to resolve to "Math::Add" (unqualified) because it matches math.h base name
	hasIncludeRes := false
	for _, e := range edges {
		// Source is Derived::doWork
		if strings.Contains(e.SourceID, "Derived::doWork") {
			if e.TargetID == "Math::Add" {
				hasIncludeRes = true
				break
			}
		}
	}
	if !hasIncludeRes {
		t.Logf("Edges found:")
		for _, e := range edges {
			t.Logf("  %s -> %s (%s)", e.SourceID, e.TargetID, e.Type)
		}
		t.Errorf("Expected resolution of Math::Add to Math::Add (found via math.h logic)")
	}
}

func TestParseCPP_ClassAndConstructor(t *testing.T) {
	parser, ok := analysis.GetParser(".cpp")
	if !ok {
		t.Fatalf("CPP parser not registered")
	}

	absPath, err := filepath.Abs("dummy_collision.cpp")
	content := []byte(`
namespace MyApp {
    class User {
    public:
        User() {} // Constructor
        void save() {}
    };

    class Order {
    public:
        Order() {} // Constructor
        void save() {}
    };
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

	// Expected specific IDs (Qualified with namespace and class, WITH file path)
    relPath := filepath.ToSlash(absPath)
	expectedIDs := []string{
		analysis.GenerateNodeID("Class", relPath+":MyApp::User", ""),
		analysis.GenerateNodeID("Function", relPath+":MyApp::User::User", "()"), // Constructor
		analysis.GenerateNodeID("Function", relPath+":MyApp::User::save", "()"),
		analysis.GenerateNodeID("Class", relPath+":MyApp::Order", ""),
		analysis.GenerateNodeID("Function", relPath+":MyApp::Order::Order", "()"), // Constructor
		analysis.GenerateNodeID("Function", relPath+":MyApp::Order::save", "()"),
	}

	for _, expected := range expectedIDs {
		if _, exists := ids[expected]; !exists {
            // Log all IDs to help debugging
            for id := range ids {
                t.Logf("Actual ID: %s", id)
            }
			t.Fatalf("Expected ID not found: %s", expected)
		}
	}
}

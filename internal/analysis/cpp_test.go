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
			startLine, _ := n.Properties["start_line"].(uint32)
			endLine, _ := n.Properties["end_line"].(uint32)
			if startLine == endLine {
				t.Errorf("Function 'hello' should span multiple lines, got start_line=%d end_line=%d", startLine, endLine)
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

func TestParseCPP_UsageBug(t *testing.T) {
	parser, ok := analysis.GetParser(".cpp")
	if !ok {
		t.Fatalf("CPP parser not registered")
	}

	absPath, err := filepath.Abs("bug.cpp")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	content := []byte(`
void targetFunc() {}
void callerFunc(int param) { targetFunc(); }
`)

	_, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	foundCall := false
	for _, e := range edges {
		if strings.Contains(e.SourceID, "callerFunc") && strings.Contains(e.TargetID, "targetFunc") {
			foundCall = true
			if strings.HasPrefix(e.SourceID, "Class:") {
				t.Errorf("Bug: SourceID is a Class! Got: %s", e.SourceID)
			}
			if !strings.HasPrefix(e.SourceID, "Function:") {
				t.Errorf("Expected SourceID to be a Function. Got: %s", e.SourceID)
			}
			if !strings.Contains(e.SourceID, "(intparam)") {
				t.Errorf("Expected SourceID to contain parameters in signature. Got: %s", e.SourceID)
			}
		}
	}
	if !foundCall {
		t.Errorf("Expected call edge not found")
	}
}

func TestParseCPP_Fragmentation(t *testing.T) {
	parser, ok := analysis.GetParser(".cpp")
	if !ok {
		t.Fatalf("CPP parser not registered")
	}

	// 1. Parse the definition file
	defPath, _ := filepath.Abs("def.cpp")
	defContent := []byte(`int Auto_Plate(int nJointIndex = -1) { return 0; }`)
	defNodes, _, err := parser.Parse(defPath, defContent)
	if err != nil {
		t.Fatalf("Parse def failed: %v", err)
	}

	var defID string
	for _, n := range defNodes {
		if strings.Contains(n.ID, "Auto_Plate") {
			defID = n.ID
			break
		}
	}
	if defID == "" {
		t.Fatalf("Definition ID not found")
	}
	// Expected defID: Function:abs/path/to/def.cpp:Auto_Plate:(intnJointIndex=-1)

	// 2. Parse the call site file
	callPath, _ := filepath.Abs("call.cpp")
	callContent := []byte(`
#include "def.h"
void caller() { Auto_Plate(10); }
`)
	_, callEdges, err := parser.Parse(callPath, callContent)
	if err != nil {
		t.Fatalf("Parse call failed: %v", err)
	}

	foundLink := false
	for _, e := range callEdges {
		if strings.Contains(e.SourceID, "caller") && strings.Contains(e.TargetID, "Auto_Plate") {
			// With the fallback, TargetID should just be "Auto_Plate"
			// The Neo4j query for GetNeighbors (and other structural queries)
			// uses: MATCH (n) WHERE n.id = $id OR n.fqn = $id OR n.name = $id
			// So "Auto_Plate" will match the definition node by its name property.
			if e.TargetID == "Auto_Plate" {
				foundLink = true
			} else {
				t.Logf("Actual TargetID: %s", e.TargetID)
			}
		}
	}

	if !foundLink {
		t.Errorf("Fragmentation detected: Call site did not link to definition name")
	}
}

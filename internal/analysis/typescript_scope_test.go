package analysis_test

import (
	"graphdb/internal/analysis"
	"testing"
)

func TestParseTypeScript_Scope(t *testing.T) {
	parser, ok := analysis.GetParser(".ts")
	if !ok {
		t.Fatalf("TypeScript parser not registered")
	}

	content := []byte(`
    class MyClass {
        methodA() {
            this.methodB();
        }
        methodB() {
            console.log("B");
        }
    }
    `)

	_, edges, err := parser.Parse("test.ts", content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	foundCall := false
    expectedSource := "test.ts:MyClass.methodA"
    expectedTarget := "test.ts:MyClass.methodB"

	for _, e := range edges {
		if e.Type == "CALLS" {
            if e.SourceID == expectedSource && e.TargetID == expectedTarget {
                foundCall = true
                break
            }
		}
	}

	if !foundCall {
		t.Errorf("Expected CALLS edge from %s to %s not found", expectedSource, expectedTarget)
        for _, e := range edges {
             if e.Type == "CALLS" {
                 t.Logf("Found: %s -> %s", e.SourceID, e.TargetID)
             }
        }
	}
}

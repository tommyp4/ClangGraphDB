package analysis_test

import (
	"strings"
	"testing"

	"clang-graphdb/internal/analysis"
)

func TestParseTypeScript_WindowsPaths(t *testing.T) {
	parser, ok := analysis.GetParser(".ts")
	if !ok {
		t.Fatalf("TypeScript parser not registered")
	}

	// Simulate a Windows path
	winPath := `C:\Users\dev\project\src\utils\math.ts`
	content := []byte(`
export function add(a: number, b: number): number {
    return a + b;
}
`)

	nodes, _, err := parser.Parse(winPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Check if IDs use forward slashes
	expectedPrefix := "C:/Users/dev/project/src/utils/math.ts"
	
	foundAdd := false
	for _, n := range nodes {
		// The ID should be normalized
		if strings.HasPrefix(n.ID, expectedPrefix) {
			if strings.HasSuffix(n.ID, ":add") {
				foundAdd = true
			}
		} else {
            // If any ID starts with the backslash version, fail
            if strings.HasPrefix(n.ID, winPath) {
                t.Errorf("ID contains backslashes: %s", n.ID)
            }
        }
	}

	if !foundAdd {
		t.Errorf("Expected node with normalized prefix %s not found. Nodes found: %+v", expectedPrefix, nodes)
	}
}

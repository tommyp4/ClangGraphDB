package analysis_test

import (
	"path/filepath"
	"strings"
	"testing"
    "os"
	"graphdb/internal/analysis"
)

func TestParseTypeScriptDI(t *testing.T) {
	parser := &analysis.TypeScriptParser{}

	// Load fixture
	absPath, _ := filepath.Abs("../../test/fixtures/typescript/DISample.ts")
	content, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	nodes, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Helper to find edge
	findEdge := func(source, target, edgeType string) bool {
		for _, e := range edges {
			if strings.Contains(e.SourceID, source) && strings.Contains(e.TargetID, target) && e.Type == edgeType {
				return true
			}
		}
		return false
	}

    // Check Nodes
    foundClass := false
    for _, n := range nodes {
        if n.Label == "Class" && strings.Contains(n.ID, "DISample") {
            foundClass = true
        }
    }
    if !foundClass {
        t.Errorf("Class DISample not found")
    }

	// 1. Check Field Injection (explicit type)
	// private logger: Logger;
	// Expect: DISample --[DEPENDS_ON]--> Logger
	if !findEdge("DISample", "Logger", "DEPENDS_ON") {
		t.Errorf("Missing DEPENDS_ON edge for Logger field")
	}

	// 2. Check Generic Field Injection
	// private repos: Repository<User>;
	// Expect: DISample --[DEPENDS_ON]--> Repository
    // Expect: DISample --[DEPENDS_ON]--> User
	if !findEdge("DISample", "Repository", "DEPENDS_ON") {
		t.Errorf("Missing DEPENDS_ON edge for Repository field generic")
	}
    if !findEdge("DISample", "User", "DEPENDS_ON") {
		t.Errorf("Missing DEPENDS_ON edge for User field generic arg")
	}

	// 3. Check Constructor Injection (Parameter Property)
	// constructor(private userService: UserService, ...)
	// Expect: DISample --[DEPENDS_ON]--> UserService
	if !findEdge("DISample", "UserService", "DEPENDS_ON") {
		t.Errorf("Missing DEPENDS_ON edge for UserService constructor param property")
	}

    // 4. Check Constructor Injection (Plain Param)
    // constructor(..., logger: Logger)
    // Expect: DISample --[DEPENDS_ON]--> Logger (redundant, but check)
    // It's covered by field Logger check above, but logically it should be there.
}

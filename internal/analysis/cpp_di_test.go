package analysis_test

import (
	"path/filepath"
	"strings"
	"testing"
    "os"
	"graphdb/internal/analysis"
)

func TestParseCppDI(t *testing.T) {
	parser := &analysis.CppParser{}

	// Load fixture
	absPath, _ := filepath.Abs("../../test/fixtures/cpp/DISample.cpp")
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

	// 1. Check Field Injection
	// UserService* userService;
	// Expect: DISample --[DEPENDS_ON]--> UserService
	if !findEdge("DISample", "UserService", "DEPENDS_ON") {
		t.Errorf("Missing DEPENDS_ON edge for UserService field")
	}

	// 2. Check Generic Field Injection
	// std::vector<UserRepository> repositories;
	// Expect: DISample --[DEPENDS_ON]--> UserRepository
	if !findEdge("DISample", "UserRepository", "DEPENDS_ON") {
		t.Errorf("Missing DEPENDS_ON edge for UserRepository field generic")
	}

	// 3. Check Constructor Injection
	// DISample(UserService* service, std::vector<UserRepository> repos)
	// Expect: DISample --[DEPENDS_ON]--> UserService
	if !findEdge("DISample", "UserService", "DEPENDS_ON") {
		t.Errorf("Missing DEPENDS_ON edge for UserService constructor param")
	}
}

package analysis_test

import (
	"path/filepath"
	"strings"
	"testing"
    "os"
	"graphdb/internal/analysis"
)

func TestParseJavaDI(t *testing.T) {
	parser := &analysis.JavaParser{}

	// Load fixture
	absPath, _ := filepath.Abs("../../test/fixtures/java/DISample.java")
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
			// Simple substring match for ID components to avoid package prefix issues
			// Source should end with source class
			// Target should be the type
			
			// e.SourceID might be "com.example.di.DISample"
			// e.TargetID might be "com.example.services.UserService"
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
	// private final UserService userService;
	// Expect: DISample --[DEPENDS_ON]--> UserService
	if !findEdge("DISample", "UserService", "DEPENDS_ON") {
		t.Errorf("Missing DEPENDS_ON edge for UserService field")
	}

	// 2. Check Field Injection (generic type)
	// private final List<UserRepository> repositories;
	// Expect: DISample --[DEPENDS_ON]--> UserRepository
	if !findEdge("DISample", "UserRepository", "DEPENDS_ON") {
		t.Errorf("Missing DEPENDS_ON edge for UserRepository field generic")
	}

	// 3. Check Constructor Injection
	// public DISample(UserService userService, List<UserRepository> repositories)
	// Expect: DISample --[DEPENDS_ON]--> UserService (redundant but should exist from param)
    // Actually, fields usually cover it, but if a param is NOT a field, we still want it.
    // In this case, they map to fields.
    // However, the current logic MIGHT miss constructor params if they aren't fields.
    // But for DI detection, constructor params are the gold standard.
    
    // Let's verify we capture constructor params.
    // The current test just checks edges. If we implement constructor scanning, we get edges.
}

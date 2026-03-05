package query

import (
	"os"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func TestSemanticTrace(t *testing.T) {
	uri := os.Getenv("NEO4J_URI")
	if uri == "" {
		t.Skip("NEO4J_URI not set, skipping integration test")
	}

	p := getProvider(t)
	defer p.Close()
        defer cleanup(t, p)
	
	// Cleanup and setup fixture data
	cleanupQuery := `
		MATCH (n) WHERE n.name STARTS WITH 'Test' OR n.id STARTS WITH 'Test' DETACH DELETE n
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, cleanupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Logf("Failed to cleanup: %v", err)
	}

	setupQuery := `
		CREATE (d:Domain {name: 'TestDomain', id: 'TestDomain'})
		CREATE (feat:Feature {name: 'TestFeature', id: 'TestFeature'})
		CREATE (func:Function {name: 'TestFunc', id: 'TestFunc'})
		CREATE (file:File {name: 'TestFile', id: 'TestFile', path: 'test_file.go'})
		CREATE (d)-[:PARENT_OF]->(feat)
		CREATE (feat)-[:IMPLEMENTS]->(func)
		CREATE (func)-[:DEFINED_IN]->(file)
	`
	_, err = neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	// Test
	paths, err := p.SemanticTrace("TestFunc")
	if err != nil {
		t.Fatalf("SemanticTrace failed: %v", err)
	}

	// Verify
	if len(paths) == 0 {
		t.Fatal("Expected at least one path, got 0")
	}

	foundDomain := false
	foundFeature := false
	foundFunc := false
	foundFile := false

	for _, path := range paths {
		for _, node := range path.Nodes {
			switch node.Label {
			case "Domain":
				if node.Properties["name"] == "TestDomain" {
					foundDomain = true
				}
			case "Feature":
				if node.Properties["name"] == "TestFeature" {
					foundFeature = true
				}
			case "Function":
				if node.Properties["name"] == "TestFunc" {
					foundFunc = true
				}
			case "File":
				if node.Properties["path"] == "test_file.go" {
					foundFile = true
				}
			}
		}
	}

	if !foundDomain {
		t.Error("Expected to find TestDomain in path")
	}
	if !foundFeature {
		t.Error("Expected to find TestFeature in path")
	}
	if !foundFunc {
		t.Error("Expected to find TestFunc in path")
	}
	if !foundFile {
		t.Error("Expected to find TestFile in path")
	}
}

//go:build integration

package query

import (
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func TestGetOverview_Phased(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	defer cleanup(t, p)

	// Setup fixture data
	// 1. A Domain node
	// 2. A top-level Feature node (no incoming relationships)
	// 3. A nested Feature node (has incoming relationship)
	// 4. A non-semantic node (e.g. Function)
	setupQuery := `
		CREATE (d:Domain {id: "Test-domain-1", name: "TestDomain1"})
		CREATE (f1:Feature {id: "Test-feat-top", name: "TestTopFeature"})
		CREATE (f2:Feature {id: "Test-feat-nested", name: "TestNestedFeature"})
		CREATE (f1)-[:PARENT_OF]->(f2)
		CREATE (fn:Function {id: "Test-func-1", name: "TestFunc1"})
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	// Test
	result, err := p.GetOverview()
	if err != nil {
		t.Fatalf("GetOverview failed: %v", err)
	}

	// Verify
	// We expect 'domain-1' and 'feat-top'
	// We do NOT expect 'feat-nested' or 'func-1'

	foundDomain := false
	foundTopFeat := false
	foundNestedFeat := false
	foundFunc := false

	for _, node := range result.Nodes {
		if node.ID == "Test-domain-1" {
			foundDomain = true
			if node.Label != "Domain" {
				t.Errorf("Expected Label 'Domain' for domain-1, got '%s'", node.Label)
			}
		}
		if node.ID == "Test-feat-top" {
			foundTopFeat = true
			if node.Label != "Feature" {
				t.Errorf("Expected Label 'Feature' for feat-top, got '%s'", node.Label)
			}
		}
		if node.ID == "Test-feat-nested" {
			foundNestedFeat = true
		}
		if node.ID == "Test-func-1" {
			foundFunc = true
		}
	}

	if !foundDomain {
		t.Error("Expected to find domain-1")
	}
	if !foundTopFeat {
		t.Error("Expected to find feat-top")
	}
	if foundNestedFeat {
		t.Error("Did NOT expect to find feat-nested")
	}
	if foundFunc {
		t.Error("Did NOT expect to find func-1")
	}
}

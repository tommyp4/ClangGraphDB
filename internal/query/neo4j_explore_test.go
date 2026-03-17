package query

import (
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"testing"
)

func TestNeo4jExploreDomain(t *testing.T) {
	p := getProvider(t)
	defer p.Close()

	// Ensure cleanup for this specific test
	defer func() {
		_, _ = neo4j.ExecuteQuery(p.ctx, p.driver, `
			MATCH (n) WHERE n.id STARTS WITH 'explore-test-' DETACH DELETE n
		`, nil, neo4j.EagerResultTransformer)
	}()

	// Clear before running just in case
	_, _ = neo4j.ExecuteQuery(p.ctx, p.driver, `
		MATCH (n) WHERE n.id STARTS WITH 'explore-test-' DETACH DELETE n
	`, nil, neo4j.EagerResultTransformer)

	// Setup hierarchy
	// TopLevel(Domain) -> Category(Feature) -> Feature(Feature)
	// Function implements Feature
	setupQuery := `
		CREATE (top:Domain {id: 'explore-test-top', name: 'Top Level'})
		CREATE (cat:Feature {id: 'explore-test-cat', name: 'Category'})
		CREATE (feat:Feature {id: 'explore-test-feat', name: 'Feature'})
		CREATE (fn:Function {id: 'explore-test-fn', name: 'MyFn'})
		
		CREATE (top)-[:PARENT_OF]->(cat)
		CREATE (cat)-[:PARENT_OF]->(feat)
		CREATE (fn)-[:IMPLEMENTS]->(feat)
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup explore fixture: %v", err)
	}

	// Test 1: Explore Top Level Domain
	// It should find the descendant implementing function "explore-test-fn"
	res, err := p.ExploreDomain("explore-test-top")
	if err != nil {
		t.Fatalf("ExploreDomain failed: %v", err)
	}

	if res.Feature.ID != "explore-test-top" {
		t.Errorf("Expected feature ID explore-test-top, got %v", res.Feature.ID)
	}

	if len(res.Functions) != 1 {
		t.Errorf("Expected 1 function for top-level, got %v", len(res.Functions))
	} else if res.Functions[0].ID != "explore-test-fn" {
		t.Errorf("Expected function ID explore-test-fn, got %v", res.Functions[0].ID)
	}

	if len(res.Children) != 1 {
		t.Errorf("Expected 1 child for top-level, got %v", len(res.Children))
	} else if res.Children[0].ID != "explore-test-cat" {
		t.Errorf("Expected child ID explore-test-cat, got %v", res.Children[0].ID)
	}

	// Test 2: Explore Leaf Feature
	res2, err := p.ExploreDomain("explore-test-feat")
	if err != nil {
		t.Fatalf("ExploreDomain failed: %v", err)
	}
	if len(res2.Functions) != 1 {
		t.Errorf("Expected 1 function for leaf, got %v", len(res2.Functions))
	}
	if res2.Parent == nil || res2.Parent.ID != "explore-test-cat" {
		t.Errorf("Expected parent ID explore-test-cat, got %v", res2.Parent)
	}
}

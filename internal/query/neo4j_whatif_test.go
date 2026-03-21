//go:build integration

package query

import (
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func TestWhatIf(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	defer cleanup(t, p)

	// Setup fixture data
	// Target set: {Inside1, Inside2}
	// Outside nodes: {Outside1, Outside2}
	// Global: {Global1} - shared
	// Orphaned: {Orphan1} - only reachable via Inside2
	setupQuery := `
		CREATE (o1:Function {name: 'TestOutside1', id: 'TestOutside1'})
		CREATE (o2:Function {name: 'TestOutside2', id: 'TestOutside2'})
		CREATE (i1:Function {name: 'TestInside1', id: 'TestInside1'})
		CREATE (i2:Function {name: 'TestInside2', id: 'TestInside2'})
		CREATE (orphan:Function {name: 'TestOrphan', id: 'TestOrphan'})
		CREATE (g:Global {name: 'TestGlobal1', id: 'TestGlobal1'})
		
		// Outside to Inside (Cross Boundary / Severed)
		CREATE (o1)-[:CALLS]->(i1)
		
		// Inside to Outside (Severed)
		CREATE (i1)-[:CALLS]->(o2)
		
		// Inside to Orphan (Orphaned if Inside is removed)
		CREATE (i2)-[:CALLS]->(orphan)
		
		// Shared State
		CREATE (o2)-[:USES_GLOBAL]->(g)
		CREATE (i2)-[:USES_GLOBAL]->(g)
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	// Test
	targets := []string{"TestInside1", "TestInside2"}
	result, err := p.WhatIf(targets)
	if err != nil {
		t.Fatalf("WhatIf failed: %v", err)
	}

	// Verify SeveredEdges (o1->i1, i1->o2, i2->orphan, i2->g)
	// Wait, is i2->orphan severed? Yes, it crosses the boundary.
	// Is i2->g severed? Yes.
	if len(result.SeveredEdges) < 4 {
		t.Errorf("Expected at least 4 severed edges, got %d", len(result.SeveredEdges))
	}

	// Verify CrossBoundaryCalls (o1->i1)
	foundCross := false
	for _, edge := range result.CrossBoundaryCalls {
		if edge.SourceID == "TestOutside1" && edge.TargetID == "TestInside1" {
			foundCross = true
		}
	}
	if !foundCross {
		t.Error("Expected to find cross-boundary call TestOutside1 -> TestInside1")
	}

	// Verify OrphanedNodes (TestOrphan)
	// TestOrphan is only reachable via TestInside2.
	foundOrphan := false
	for _, node := range result.OrphanedNodes {
		if node.ID == "TestOrphan" {
			foundOrphan = true
		}
	}
	if !foundOrphan {
		t.Error("Expected to find orphaned node TestOrphan")
	}

	// Verify SharedState (TestGlobal1)
	foundShared := false
	for _, node := range result.SharedState {
		if node.ID == "TestGlobal1" {
			foundShared = true
		}
	}
	if !foundShared {
		t.Error("Expected to find shared state TestGlobal1")
	}

	// Verify AffectedNodes
	// Should contain TestOutside1, TestInside1, TestOutside2, TestInside2, TestOrphan, TestGlobal1
	expectedAffected := []string{"TestOutside1", "TestInside1", "TestOutside2", "TestInside2", "TestOrphan", "TestGlobal1"}
	for _, id := range expectedAffected {
		found := false
		for _, node := range result.AffectedNodes {
			if node.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find %s in AffectedNodes", id)
		}
	}
}

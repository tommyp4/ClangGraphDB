package query

import (
	"graphdb/internal/graph"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func TestNeo4jBatchOperations(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	
	// Ensure cleanup for this specific test
	defer func() {
		_, _ = neo4j.ExecuteQuery(p.ctx, p.driver, `
			MATCH (n) WHERE n.id STARTS WITH 'batch-test-' DETACH DELETE n
		`, nil, neo4j.EagerResultTransformer)
	}()
	
	// Clear before running just in case
	_, _ = neo4j.ExecuteQuery(p.ctx, p.driver, `
		MATCH (n) WHERE n.id STARTS WITH 'batch-test-' DETACH DELETE n
	`, nil, neo4j.EagerResultTransformer)

	// Setup initial state for batch tests
	setupQuery := `
		CREATE (f1:Function {id: 'batch-test-f1', file: 'f1.go', start_line: 1, end_line: 10, content: 'func f1() {}'})
		CREATE (f2:Function {id: 'batch-test-f2', file: 'f2.go', start_line: 11, end_line: 20, content: 'func f2() {}'})
		CREATE (f3:Function {id: 'batch-test-f3', file: 'f3.go', start_line: 21, end_line: 30, content: 'func f3() {}', atomic_features: ['feature1']})
		CREATE (feat1:Feature {id: 'batch-test-feat1'})
		CREATE (feat2:Feature {id: 'batch-test-feat2', name: 'Existing Name', summary: 'Existing Summary'})
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup batch fixture: %v", err)
	}

	// 1. Test GetUnextractedFunctions
	unextracted, err := p.GetUnextractedFunctions(10)
	if err != nil {
		t.Fatalf("GetUnextractedFunctions failed: %v", err)
	}
	if len(unextracted) != 2 {
		t.Errorf("Expected 2 unextracted functions, got %d", len(unextracted))
	}

	// 2. Test UpdateAtomicFeatures
	err = p.UpdateAtomicFeatures("batch-test-f1", []string{"new-feature-1", "new-feature-2"})
	if err != nil {
		t.Fatalf("UpdateAtomicFeatures failed: %v", err)
	}
	
	// Verify it was updated by re-fetching
	unextractedAfter, _ := p.GetUnextractedFunctions(10)
	if len(unextractedAfter) != 1 {
		t.Errorf("Expected 1 unextracted function after update, got %d", len(unextractedAfter))
	}

	// 3. Test GetUnembeddedNodes
	// f1, f2, f3, feat1, feat2 should all lack embeddings initially
	unembedded, err := p.GetUnembeddedNodes(100)
	if err != nil {
		t.Fatalf("GetUnembeddedNodes failed: %v", err)
	}
	unembeddedTestNodes := 0
	for _, n := range unembedded {
		if len(n.ID) > 11 && n.ID[:11] == "batch-test-" {
			unembeddedTestNodes++
		}
	}
	if unembeddedTestNodes != 5 {
		t.Errorf("Expected 5 unembedded test nodes, got %d", unembeddedTestNodes)
	}

	// 4. Test UpdateEmbeddings
	err = p.UpdateEmbeddings("batch-test-f1", []float32{0.1, 0.2, 0.3})
	if err != nil {
		t.Fatalf("UpdateEmbeddings failed: %v", err)
	}

	// 5. Test GetEmbeddingsOnly
	embeddings, err := p.GetEmbeddingsOnly()
	if err != nil {
		t.Fatalf("GetEmbeddingsOnly failed: %v", err)
	}
	if len(embeddings) < 1 {
		t.Errorf("Expected at least 1 embedded node, got %d", len(embeddings))
	}
	if emb, ok := embeddings["batch-test-f1"]; ok {
		if emb[0] != 0.1 || emb[1] != 0.2 || emb[2] != 0.3 {
			t.Errorf("Embedding values incorrect, got %v", emb)
		}
	} else {
		t.Errorf("Did not find batch-test-f1 in embeddings")
	}

	// 6. Test GetUnnamedFeatures
	unnamed, err := p.GetUnnamedFeatures(10)
	if err != nil {
		t.Fatalf("GetUnnamedFeatures failed: %v", err)
	}
	// feat1 should be unnamed, feat2 is named
	foundFeat1 := false
	for _, n := range unnamed {
		if n.ID == "batch-test-feat1" {
			foundFeat1 = true
		}
	}
	if !foundFeat1 {
		t.Errorf("Expected to find batch-test-feat1 in unnamed features")
	}

	// 7. Test UpdateFeatureSummary
	err = p.UpdateFeatureSummary("batch-test-feat1", "New Name", "New Summary")
	if err != nil {
		t.Fatalf("UpdateFeatureSummary failed: %v", err)
	}

	// 8. Test UpdateFeatureTopology
	nodes := []*graph.Node{
		{ID: "batch-test-feat3", Properties: map[string]any{"some_prop": "val"}},
	}
	edges := []*graph.Edge{
		{SourceID: "batch-test-feat3", TargetID: "batch-test-f1", Type: "CONTAINS"},
	}
	err = p.UpdateFeatureTopology(nodes, edges)
	if err != nil {
		t.Fatalf("UpdateFeatureTopology failed: %v", err)
	}
	
	// Verify topology update by querying it directly
	query := `
		MATCH (s {id: 'batch-test-feat3'})-[r:CONTAINS]->(t {id: 'batch-test-f1'})
		RETURN count(r) as count
	`
	res, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, nil, neo4j.EagerResultTransformer)
	if err != nil || len(res.Records) == 0 {
		t.Fatalf("Failed to verify topology: %v", err)
	}
	count, _, _ := neo4j.GetRecordValue[int64](res.Records[0], "count")
	if count != 1 {
		t.Errorf("Expected 1 CONTAINS relationship, got %d", count)
	}
}

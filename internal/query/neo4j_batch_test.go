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
		CREATE (f1:Function:CodeElement {id: 'batch-test-f1', file: 'f1.go', line: 1, start_line: 1, end_line: 10, content: 'func f1() {}'})
		CREATE (f2:Function:CodeElement {id: 'batch-test-f2', file: 'f2.go', line: 11, start_line: 11, end_line: 20, content: 'func f2() {}'})
		CREATE (f3:Function:CodeElement {id: 'batch-test-f3', name: 'f3', file: 'f3.go', line: 21, start_line: 21, end_line: 30, content: 'func f3() {}', atomic_features: ['feature1']})
		CREATE (feat1:Feature {id: 'batch-test-feat1'})
		CREATE (feat2:Feature {id: 'batch-test-feat2', name: 'Existing Name', description: 'Existing Description'})
		CREATE (feat3:Feature {id: 'batch-test-feat-semi', name: 'Some Name'})
		CREATE (feat4:Feature {id: 'batch-test-feat-empty', name: '', description: ''})
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
	err = p.UpdateAtomicFeatures("batch-test-f1", []string{"new-feature-1", "new-feature-2"}, true)
	if err != nil {
		t.Fatalf("UpdateAtomicFeatures failed: %v", err)
	}

	// Verify it was updated (including is_volatile)
	verifyQuery := `
		MATCH (n:Function {id: 'batch-test-f1'})
		RETURN n.is_volatile as is_volatile
	`
	vRes, err := neo4j.ExecuteQuery(p.ctx, p.driver, verifyQuery, nil, neo4j.EagerResultTransformer)
	if err != nil || len(vRes.Records) == 0 {
		t.Fatalf("Failed to verify is_volatile update: %v", err)
	}
	isVolatile, _, _ := neo4j.GetRecordValue[bool](vRes.Records[0], "is_volatile")
	if !isVolatile {
		t.Errorf("Expected is_volatile to be true")
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
	if unembeddedTestNodes != 7 {
		t.Errorf("Expected 7 unembedded test nodes, got %d", unembeddedTestNodes)
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
	unnamed, err := p.GetUnnamedFeatures(1000)
	if err != nil {
		t.Fatalf("GetUnnamedFeatures failed: %v", err)
	}
	// feat1 should be unnamed (no name property), 
	// feat2 is named and described, so should be excluded.
	// feat3 has name but no description (semi-named), so should be picked up.
	// feat4 has empty name and empty description, so should be picked up.
	foundFeat1 := false
	foundFeatSemi := false
	foundFeatEmpty := false
	foundFeat2 := false
	for _, n := range unnamed {
		if n.ID == "batch-test-feat1" {
			foundFeat1 = true
		}
		if n.ID == "batch-test-feat-semi" {
			foundFeatSemi = true
		}
		if n.ID == "batch-test-feat-empty" {
			foundFeatEmpty = true
		}
		if n.ID == "batch-test-feat2" {
			foundFeat2 = true
		}
	}
	if !foundFeat1 {
		t.Errorf("Expected to find batch-test-feat1 in unnamed features (missing name)")
	}
	if !foundFeatSemi {
		t.Errorf("Expected to find batch-test-feat-semi in unnamed features (missing description)")
	}
	if !foundFeatEmpty {
		t.Errorf("Expected to find batch-test-feat-empty in unnamed features (empty name)")
	}
	if foundFeat2 {
		t.Errorf("Did not expect to find batch-test-feat2 in unnamed features (already named and described)")
	}

	// 6.5 Test GetFunctionMetadata
	metadata, err := p.GetFunctionMetadata()
	if err != nil {
		t.Fatalf("GetFunctionMetadata failed: %v", err)
	}
	foundF3Metadata := false
	for _, n := range metadata {
		if n.ID == "batch-test-f3" {
			foundF3Metadata = true
			if n.Properties["name"] != "f3" {
				t.Errorf("GetFunctionMetadata missing/wrong name: %v", n.Properties["name"])
			}
			if n.Properties["file"] != "f3.go" {
				t.Errorf("GetFunctionMetadata missing/wrong file: %v", n.Properties["file"])
			}
			if n.Properties["line"] != int64(21) {
				t.Errorf("GetFunctionMetadata missing/wrong line: %v", n.Properties["line"])
			}
			if n.Properties["end_line"] != int64(30) {
				t.Errorf("GetFunctionMetadata missing/wrong end_line: %v", n.Properties["end_line"])
			}
			af, ok := n.Properties["atomic_features"].([]string)
			if !ok || len(af) == 0 || af[0] != "feature1" {
				t.Errorf("GetFunctionMetadata missing/wrong atomic_features: %v", n.Properties["atomic_features"])
			}
		}
	}
	if !foundF3Metadata {
		t.Errorf("Expected to find batch-test-f3 in function metadata")
	}

	// 7. Test UpdateFeatureSummary
	err = p.UpdateFeatureSummary("batch-test-feat1", "New Name", "New Description")
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
	
	// Verify topology update: node should have :CodeElement label
	labelQuery := `
		MATCH (n:CodeElement {id: 'batch-test-feat3'})
		RETURN count(n) as count
	`
	labelRes, err := neo4j.ExecuteQuery(p.ctx, p.driver, labelQuery, nil, neo4j.EagerResultTransformer)
	if err != nil || len(labelRes.Records) == 0 {
		t.Fatalf("Failed to verify CodeElement label: %v", err)
	}
	labelCount, _, _ := neo4j.GetRecordValue[int64](labelRes.Records[0], "count")
	if labelCount != 1 {
		t.Errorf("Expected batch-test-feat3 to have :CodeElement label, got count %d", labelCount)
	}

	// Verify topology update by querying the edge
	query := `
		MATCH (s:CodeElement {id: 'batch-test-feat3'})-[r:CONTAINS]->(t:CodeElement {id: 'batch-test-f1'})
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

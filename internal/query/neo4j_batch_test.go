//go:build integration

package query

import (
	"clang-graphdb/internal/config"
	"clang-graphdb/internal/graph"
	"os"
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
		CREATE (f1:Function:CodeElement {id: 'batch-test-f1', file: 'f1.go', start_line: 1, end_line: 10, content: 'func f1() {}'})
		CREATE (f2:Function:CodeElement {id: 'batch-test-f2', file: 'f2.go', start_line: 11, end_line: 20, content: 'func f2() {}'})
		CREATE (f3:Function:CodeElement {id: 'batch-test-f3', name: 'f3', file: 'f3.go', start_line: 21, end_line: 30, content: 'func f3() {}', atomic_features: ['feature1']})
		CREATE (feat1:Feature {id: 'batch-test-feat1'})
		CREATE (feat2:Feature {id: 'batch-test-feat2', name: 'Existing Name', description: 'Existing Description'})
		CREATE (feat3:Feature {id: 'batch-test-feat-semi', name: 'Some Name'})
		CREATE (feat4:Feature {id: 'batch-test-feat-empty', name: '', description: ''})
		CREATE (dom1:Domain {id: 'batch-test-dom1'})
		CREATE (dom2:Domain {id: 'batch-test-dom2', name: 'Existing Domain', description: 'Existing Domain Desc'})
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup batch fixture: %v", err)
	}

	// 1. Test GetUnextractedFunctions
	unextracted, err := p.GetUnextractedFunctions(10000)
	if err != nil {
		t.Fatalf("GetUnextractedFunctions failed: %v", err)
	}

	unextractedTestNodes := 0
	for _, n := range unextracted {
		if len(n.ID) > 11 && n.ID[:11] == "batch-test-" {
			unextractedTestNodes++
		}
	}
	if unextractedTestNodes != 2 {
		t.Errorf("Expected 2 unextracted functions, got %d", unextractedTestNodes)
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
	unextractedAfter, _ := p.GetUnextractedFunctions(10000)
	unextractedAfterTestNodes := 0
	for _, n := range unextractedAfter {
		if len(n.ID) > 11 && n.ID[:11] == "batch-test-" {
			unextractedAfterTestNodes++
		}
	}
	if unextractedAfterTestNodes != 1 {
		t.Errorf("Expected 1 unextracted function after update, got %d", unextractedAfterTestNodes)
	}

	// 3. Test GetUnembeddedNodes
	// GetUnembeddedNodes queries Function and Feature labels (not Domain),
	// so we expect: f1, f2, f3, feat1, feat2, feat-semi, feat-empty = 7 nodes
	unembedded, err := p.GetUnembeddedNodes(10000)
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
	// dom1 should be unnamed (no name property).
	// dom2 is named and described, so should be excluded.
	foundFeat1 := false
	foundFeatSemi := false
	foundFeatEmpty := false
	foundFeat2 := false
	foundDom1 := false
	foundDom2 := false
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
		if n.ID == "batch-test-dom1" {
			foundDom1 = true
		}
		if n.ID == "batch-test-dom2" {
			foundDom2 = true
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
	if !foundDom1 {
		t.Errorf("Expected to find batch-test-dom1 in unnamed features (Domain node missing name)")
	}
	if foundDom2 {
		t.Errorf("Did not expect to find batch-test-dom2 in unnamed features (Domain already named and described)")
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
			if n.Properties["start_line"] != int64(21) {
				t.Errorf("GetFunctionMetadata missing/wrong start_line: %v", n.Properties["start_line"])
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

	// 7. Test UpdateFeatureSummary (Feature)
	err = p.UpdateFeatureSummary("batch-test-feat1", "New Name", "New Description")
	if err != nil {
		t.Fatalf("UpdateFeatureSummary failed for feature: %v", err)
	}

	// 7.5 Test UpdateFeatureSummary (Domain)
	err = p.UpdateFeatureSummary("batch-test-dom1", "New Domain Name", "New Domain Desc")
	if err != nil {
		t.Fatalf("UpdateFeatureSummary failed for domain: %v", err)
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

func TestGetUnextractedFunctions_HappyPath(t *testing.T) {
	// This test asserts that GetUnextractedFunctions properly retrieves nodes
	// using the canonical 'start_line' property.

	cfg := config.Config{
		Neo4jURI:      os.Getenv("NEO4J_URI"),
		Neo4jUser:     os.Getenv("NEO4J_USER"),
		Neo4jPassword: os.Getenv("NEO4J_PASSWORD"),
	}

	p, err := NewNeo4jProvider(cfg)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer p.Close()

	// 1. Setup Data with 'start_line' (canonical schema)
	setupQuery := `
		CREATE (n:CodeElement:Function {
			id: 'schema-test-happy',
			name: 'TestFunc',
			file: 'test_file.go',
			start_line: 10,
			end_line: 20
		})
	`
	_, err = neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	defer func() {
		cleanupQuery := `MATCH (n) WHERE n.id = 'schema-test-happy' DETACH DELETE n`
		neo4j.ExecuteQuery(p.ctx, p.driver, cleanupQuery, nil, neo4j.EagerResultTransformer)
	}()

	// 2. Execute Method
	nodes, err := p.GetUnextractedFunctions(10000)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// 3. Assert
	found := false
	for _, n := range nodes {
		if n.ID == "schema-test-happy" {
			found = true
			if _, ok := n.Properties["start_line"]; !ok {
				t.Errorf("Node found but property mapping expected 'start_line' in properties, got: %v", n.Properties)
			}
		}
	}

	if !found {
		t.Errorf("GetUnextractedFunctions failed to find the node.")
	}
}

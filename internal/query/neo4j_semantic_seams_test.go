package query

import (
	"context"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// mockSemanticSeamsProvider is a minimal mock for testing the interface definition.
type mockSemanticSeamsProvider struct {
	GraphProvider
	GetSemanticSeamsCalled bool
}

func (m *mockSemanticSeamsProvider) GetSemanticSeams(ctx context.Context, similarityThreshold float64) ([]*SemanticSeamResult, error) {
	m.GetSemanticSeamsCalled = true
	return []*SemanticSeamResult{
		{
			Container:  "mock_file.go",
			MethodA:    "funcA",
			MethodB:    "funcB",
			Similarity: 0.1,
		},
	}, nil
}

func (m *mockSemanticSeamsProvider) Close() error { return nil }

func TestGetSemanticSeamsInterface(t *testing.T) {
	// 1. Setup Mock
	mock := &mockSemanticSeamsProvider{}
	var provider GraphProvider = mock

	// 2. Execute
	threshold := 0.5
	results, err := provider.GetSemanticSeams(context.Background(), threshold)

	// 3. Assertions
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !mock.GetSemanticSeamsCalled {
		t.Errorf("Expected GetSemanticSeams to be called on mock")
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	} else {
		if results[0].Container != "mock_file.go" {
			t.Errorf("Expected Container 'mock_file.go', got '%s'", results[0].Container)
		}
		if results[0].MethodA != "funcA" {
			t.Errorf("Expected MethodA 'funcA', got '%s'", results[0].MethodA)
		}
		if results[0].Similarity != 0.1 {
			t.Errorf("Expected Similarity 0.1, got %f", results[0].Similarity)
		}
	}
}

func TestNeo4jProvider_GetSemanticSeams_Integration(t *testing.T) {
	p := getProvider(t)
	defer p.Close()

	// Ensure cleanup for this specific test
	defer func() {
		_, _ = neo4j.ExecuteQuery(p.ctx, p.driver, `
			MATCH (n) WHERE n.id STARTS WITH 'seams-test-' DETACH DELETE n
		`, nil, neo4j.EagerResultTransformer)
	}()

	// Clear before running just in case
	_, _ = neo4j.ExecuteQuery(p.ctx, p.driver, `
		MATCH (n) WHERE n.id STARTS WITH 'seams-test-' DETACH DELETE n
	`, nil, neo4j.EagerResultTransformer)

	// Setup initial state: 1 File with 2 Functions that are divergent
	setupQuery := `
		CREATE (f:File {id: 'seams-test-file1', name: 'divergent_file.go'})
		CREATE (f1:Function {id: 'seams-test-f1', name: 'func1', embedding: [1.0, 0.0, 0.0]})
		CREATE (f2:Function {id: 'seams-test-f2', name: 'func2', embedding: [0.0, 1.0, 0.0]})
		CREATE (f)-[:DEFINES]->(f1)
		CREATE (f)-[:DEFINES]->(f2)

		CREATE (f_close:File {id: 'seams-test-file2', name: 'cohesive_file.go'})
		CREATE (f3:Function {id: 'seams-test-f3', name: 'func3', embedding: [1.0, 0.0, 0.0]})
		CREATE (f4:Function {id: 'seams-test-f4', name: 'func4', embedding: [0.9, 0.1, 0.0]})
		CREATE (f_close)-[:DEFINES]->(f3)
		CREATE (f_close)-[:DEFINES]->(f4)
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup seams fixture: %v", err)
	}

	// 1. Test GetSemanticSeams with a threshold that should only find the divergent pair
	results, err := p.GetSemanticSeams(p.ctx, 0.6)
	if err != nil {
		t.Fatalf("GetSemanticSeams failed: %v", err)
	}

	// 2. Assertions
	foundDivergent := false
	for _, res := range results {
		if res.Container == "divergent_file.go" {
			foundDivergent = true
			if res.MethodA != "func1" && res.MethodA != "func2" {
				t.Errorf("Expected MethodA to be func1 or func2, got %s", res.MethodA)
			}
			if res.Similarity > 0.51 { // Cosine similarity should be 0.5 in this environment
				t.Errorf("Expected low similarity for divergent file, got %f", res.Similarity)
			}
		}
		if res.Container == "cohesive_file.go" {
			t.Errorf("Did not expect cohesive_file.go to be in results")
		}
	}

	if !foundDivergent {
		t.Errorf("Expected to find divergent_file.go in results")
	}
}

//go:build integration

package query

import (
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type mockDuplicatesProvider struct {
	GraphProvider
	FindDuplicatesCalled bool
}

func (m *mockDuplicatesProvider) FindDuplicates(similarityThreshold float64, limit int) ([]*DuplicateResult, error) {
	m.FindDuplicatesCalled = true
	return []*DuplicateResult{
		{
			FunctionA:  "funcA",
			IDA:        "id_a",
			FunctionB:  "funcB",
			IDB:        "id_b",
			Similarity: 0.95,
		},
	}, nil
}

func (m *mockDuplicatesProvider) Close() error { return nil }

func TestFindDuplicatesInterface(t *testing.T) {
	mock := &mockDuplicatesProvider{}
	var provider GraphProvider = mock

	results, err := provider.FindDuplicates(0.9, 10)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !mock.FindDuplicatesCalled {
		t.Errorf("Expected FindDuplicates to be called on mock")
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].FunctionA != "funcA" {
		t.Errorf("Expected FunctionA 'funcA', got %v", results[0].FunctionA)
	}
}

func TestNeo4jProvider_FindDuplicates_Integration(t *testing.T) {
	p := getProvider(t)
	defer p.Close()

	defer func() {
		_, _ = neo4j.ExecuteQuery(p.ctx, p.driver, `MATCH (n) WHERE n.id STARTS WITH 'dup-test-' DETACH DELETE n`, nil, neo4j.EagerResultTransformer)
	}()
	_, _ = neo4j.ExecuteQuery(p.ctx, p.driver, `MATCH (n) WHERE n.id STARTS WITH 'dup-test-' DETACH DELETE n`, nil, neo4j.EagerResultTransformer)

	setupQuery := `
		CREATE (f1:Function {id: 'dup-test-f1', name: 'func1', embedding: [1.0, 0.0, 0.0]})
		CREATE (f2:Function {id: 'dup-test-f2', name: 'func2', embedding: [0.99, 0.1, 0.0]})
		CREATE (f3:Function {id: 'dup-test-f3', name: 'func3', embedding: [0.0, 1.0, 0.0]})
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	// Should find func1 and func2 but not func3
	results, err := p.FindDuplicates(0.9, 10)
	if err != nil {
		t.Fatalf("FindDuplicates failed: %v", err)
	}

	found := false
	for _, res := range results {
		if (res.FunctionA == "func1" && res.FunctionB == "func2") || (res.FunctionA == "func2" && res.FunctionB == "func1") {
			found = true
			if res.Similarity < 0.9 {
				t.Errorf("Expected high similarity, got %f", res.Similarity)
			}
		}
		if res.FunctionA == "func3" || res.FunctionB == "func3" {
			t.Errorf("Did not expect func3 in duplicates")
		}
	}

	if !found {
		t.Errorf("Expected to find dup-test-f1 and dup-test-f2 as duplicates")
	}
}

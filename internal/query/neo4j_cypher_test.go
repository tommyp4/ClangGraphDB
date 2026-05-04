//go:build integration

package query

import (
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type mockCypherProvider struct {
	GraphProvider
	RunCypherCalled bool
}

func (m *mockCypherProvider) RunCypher(query string) ([]map[string]any, error) {
	m.RunCypherCalled = true
	return []map[string]any{
		{"count": int64(1)},
	}, nil
}

func (m *mockCypherProvider) Close() error { return nil }

func TestRunCypherInterface(t *testing.T) {
	mock := &mockCypherProvider{}
	var provider GraphProvider = mock

	results, err := provider.RunCypher("MATCH (n) RETURN count(n) AS count")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !mock.RunCypherCalled {
		t.Errorf("Expected RunCypher to be called on mock")
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0]["count"] != int64(1) {
		t.Errorf("Expected count 1, got %v", results[0]["count"])
	}
}

func TestNeo4jProvider_RunCypher_Integration(t *testing.T) {
	p := getProvider(t)
	defer p.Close()

	defer func() {
		_, _ = neo4j.ExecuteQuery(p.ctx, p.driver, `MATCH (n) WHERE n.id STARTS WITH 'cypher-test-' DETACH DELETE n`, nil, neo4j.EagerResultTransformer)
	}()
	_, _ = neo4j.ExecuteQuery(p.ctx, p.driver, `MATCH (n) WHERE n.id STARTS WITH 'cypher-test-' DETACH DELETE n`, nil, neo4j.EagerResultTransformer)

	setupQuery := `
		CREATE (f:Function {id: 'cypher-test-f1', name: 'myCypherFunc'})
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	results, err := p.RunCypher("MATCH (n:Function {id: 'cypher-test-f1'}) RETURN n.name AS name, n")
	if err != nil {
		t.Fatalf("RunCypher failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0]["name"] != "myCypherFunc" {
		t.Errorf("Expected name 'myCypherFunc', got %v", results[0]["name"])
	}
	
	nodeMap, ok := results[0]["n"].(map[string]any)
	if !ok {
		t.Fatalf("Expected n to be a map, got %T", results[0]["n"])
	}
	if nodeMap["id"] != "cypher-test-f1" {
		t.Errorf("Expected node map id 'cypher-test-f1', got %v", nodeMap["id"])
	}
}

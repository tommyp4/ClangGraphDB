package query

import (
	"fmt"
	"graphdb/internal/graph"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// GetCoverage returns the test functions that test a given function.
func (p *Neo4jProvider) GetCoverage(nodeID string) ([]*graph.Node, error) {
	query := `
		// Get Coverage
		MATCH (p) WHERE p.id = $id OR p.fqn = $id OR p.name = $id
		MATCH (t)-[:TESTS]->(p)
		WHERE (t:Function OR t:Method)
		RETURN t.id as id, labels(t) as labels, properties(t) as props
	`

	result, err := p.executeQuery(query, map[string]any{
		"id": nodeID,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to execute GetCoverage query: %w", err)
	}

	testNodes := make([]*graph.Node, 0, len(result.Records))
	for _, record := range result.Records {
		id, _, _ := neo4j.GetRecordValue[string](record, "id")
		labelsRaw, _, _ := neo4j.GetRecordValue[[]any](record, "labels")
		props, _, _ := neo4j.GetRecordValue[map[string]any](record, "props")

		label := "Function"
		if len(labelsRaw) > 0 {
			if firstLabel, ok := labelsRaw[0].(string); ok {
				label = firstLabel
			}
		}

		testNodes = append(testNodes, &graph.Node{
			ID:         id,
			Label:      label,
			Properties: sanitizeProperties(props),
		})
	}

	return testNodes, nil
}

// LinkTests creates TESTS edges between test functions and production functions based on naming conventions.
func (p *Neo4jProvider) LinkTests() error {
	query := `
		// Link Tests
		MATCH (t) WHERE (t:Function OR t:Method) AND t.is_test = true
		MATCH (p) WHERE (p:Function OR p:Method) AND (p.is_test IS NULL OR p.is_test = false)
		  AND (t.name = "Test" + p.name OR t.name = p.name + "Test" OR t.name = p.name + "Tests")
		MERGE (t)-[:TESTS]->(p)
		RETURN count(*) as count
	`

	res, err := p.executeQuery(query, nil)
	if err != nil {
		return fmt.Errorf("failed to execute LinkTests query: %w", err)
	}

	count, _, _ := neo4j.GetRecordValue[int64](res.Records[0], "count")
	fmt.Printf("Created %d TESTS edges.\n", count)

	return nil
}

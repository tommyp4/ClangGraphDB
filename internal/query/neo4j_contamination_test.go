package query

import (
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func TestCalculateRiskScores(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	cleanup(t, p)
	defer cleanup(t, p)

	// Setup fixture
	// f1: High fan-in, high churn
	// f2: Low fan-in, low churn
	setupQuery := `
		CREATE (f1:Function {name: 'Test_HighRisk', id: 'f1'})
		CREATE (file1:File {file: 'Test_churny.go', id: 'file1', change_frequency: 100})
		CREATE (f1)-[:DEFINED_IN]->(file1)
		
		CREATE (caller1:Function {name: 'Test_Caller1', id: 'c1'})
		CREATE (caller2:Function {name: 'Test_Caller2', id: 'c2'})
		CREATE (caller1)-[:CALLS]->(f1)
		CREATE (caller2)-[:CALLS]->(f1)

		CREATE (f2:Function {name: 'Test_LowRisk', id: 'f2'})
		CREATE (file2:File {file: 'Test_stable.go', id: 'file2', change_frequency: 1})
		CREATE (f2)-[:DEFINED_IN]->(file2)
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	err = p.CalculateRiskScores()
	if err != nil {
		t.Fatalf("CalculateRiskScores failed: %v", err)
	}

	// Verify
	verifyQuery := `
		MATCH (f:Function)
		WHERE f.name STARTS WITH 'Test_'
		RETURN f.name as name, f.risk_score as score
		ORDER BY f.risk_score DESC
	`
	result, err := neo4j.ExecuteQuery(p.ctx, p.driver, verifyQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to verify: %v", err)
	}

	if len(result.Records) < 2 {
		t.Fatalf("Expected at least 2 records, got %d", len(result.Records))
	}

	topName, _, _ := neo4j.GetRecordValue[string](result.Records[0], "name")
	if topName != "Test_HighRisk" {
		t.Errorf("Expected Test_HighRisk to have highest score, got %s", topName)
	}

	topScore, _, _ := neo4j.GetRecordValue[float64](result.Records[0], "score")
	if topScore <= 0 {
		t.Errorf("Expected positive risk score, got %f", topScore)
	}
}

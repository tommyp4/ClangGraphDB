//go:build integration

package query

import (
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"testing"
)

func TestCoverageIntegration(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	defer cleanup(t, p)

	// 1. Setup fixture data with production and test functions
	setupQuery := `
		CREATE (p1:Function {name: 'TestCoverageFunc', id: 'TestCoverageFunc', is_test: false})
		CREATE (t1:Function {name: 'TestOtherFunc', id: 'TestOtherFunc', is_test: true})
		CREATE (t2:Function {name: 'TestCoverageFuncTest', id: 'TestCoverageFuncTest', is_test: true})
		CREATE (t3:Function {name: 'TestCoverageFuncTests', id: 'TestCoverageFuncTests', is_test: true})
		CREATE (t4:Function {name: 'TestTestCoverageFunc', id: 'TestTestCoverageFunc', is_test: true})
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	// 2. Run LinkTests
	err = p.LinkTests()
	if err != nil {
		t.Fatalf("LinkTests failed: %v", err)
	}

	// 3. Verify TESTS edges
	verifyQuery := `
		MATCH (t:Function)-[:TESTS]->(p:Function {name: 'TestCoverageFunc'})
		RETURN count(t) as count, collect(t.name) as names
	`
	res, err := neo4j.ExecuteQuery(p.ctx, p.driver, verifyQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to verify TESTS edges: %v", err)
	}

	count, _, _ := neo4j.GetRecordValue[int64](res.Records[0], "count")
	namesRaw, _, _ := neo4j.GetRecordValue[[]any](res.Records[0], "names")

	// We expect TestCoverageFuncTest, TestCoverageFuncTests, TestTestCoverageFunc to link.
	// TestOtherFunc should NOT link.
	if count != 3 {
		t.Errorf("Expected 3 TESTS edges, got %d (%v)", count, namesRaw)
	}

	// 4. Test GetCoverage
	coverage, err := p.GetCoverage("TestCoverageFunc")
	if err != nil {
		t.Fatalf("GetCoverage failed: %v", err)
	}

	if len(coverage) != 3 {
		t.Errorf("Expected 3 test nodes in coverage, got %d", len(coverage))
	}

	// 5. Test Method coverage
	methodSetupQuery := `
		CREATE (pm1:Method {name: 'TestCoverageMethod', id: 'TestCoverageMethod', is_test: false})
		CREATE (tm1:Method {name: 'TestCoverageMethodTest', id: 'TestCoverageMethodTest', is_test: true})
		CREATE (tm2:Method {name: 'TestTestCoverageMethod', id: 'TestTestCoverageMethod', is_test: true})
	`
	_, err = neo4j.ExecuteQuery(p.ctx, p.driver, methodSetupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup Method fixture: %v", err)
	}

	err = p.LinkTests()
	if err != nil {
		t.Fatalf("LinkTests for Methods failed: %v", err)
	}

	methodCoverage, err := p.GetCoverage("TestCoverageMethod")
	if err != nil {
		t.Fatalf("GetCoverage for TestCoverageMethod failed: %v", err)
	}

	if len(methodCoverage) != 2 {
		t.Errorf("Expected 2 test nodes in coverage for TestCoverageMethod, got %d", len(methodCoverage))
	}

	for _, node := range methodCoverage {
		if node.Label != "Method" {
			t.Errorf("Expected test node label 'Method', got %s", node.Label)
		}
	}
}

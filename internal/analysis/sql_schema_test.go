package analysis_test

import (
	"testing"

	"graphdb/internal/analysis"
)

func TestParseSQL_Schema(t *testing.T) {
	parser, ok := analysis.GetParser(".sql")
	if !ok {
		t.Fatalf("SQL parser not registered")
	}

	content := []byte(`
CREATE FUNCTION sales.calculate_tax() RETURNS INT AS $$
BEGIN
    RETURN 10;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION finance.process_invoice() RETURNS VOID AS $$
BEGIN
    SELECT sales.calculate_tax();
END;
$$ LANGUAGE plpgsql;
`)

	nodes, edges, err := parser.Parse("dummy.sql", content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ids := make(map[string]bool)
	for _, n := range nodes {
		ids[n.ID] = true
	}

	expectedID1 := analysis.GenerateNodeID("Function", "dummy.sql:sales.calculate_tax", "")
	expectedID2 := analysis.GenerateNodeID("Function", "dummy.sql:finance.process_invoice", "")

	if !ids[expectedID1] {
		t.Errorf("Expected Function ID %s not found. Found: %+v", expectedID1, ids)
	}

	if !ids[expectedID2] {
		t.Errorf("Expected Function ID %s not found. Found: %+v", expectedID2, ids)
	}
	
	foundCall := false
	expectedTargetFQN := "dummy.sql:sales.calculate_tax"
	for _, e := range edges {
		if e.SourceID == expectedID2 && e.TargetID == expectedTargetFQN {
			foundCall = true
			break
		}
	}
	if !foundCall {
        t.Logf("Actual edges:")
        for _, e := range edges {
            t.Logf("  %s -> %s (%s)", e.SourceID, e.TargetID, e.Type)
        }
		t.Errorf("Expected Call Edge %s -> %s not found", expectedID2, expectedTargetFQN)
	}
}

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

	if !ids["sales.calculate_tax"] {
		t.Errorf("Expected Function ID sales.calculate_tax not found. Found: %+v", ids)
	}

	if !ids["finance.process_invoice"] {
		t.Errorf("Expected Function ID finance.process_invoice not found. Found: %+v", ids)
	}
	
	foundCall := false
	for _, e := range edges {
		if e.SourceID == "finance.process_invoice" && e.TargetID == "sales.calculate_tax" {
			foundCall = true
			break
		}
	}
	if !foundCall {
		t.Errorf("Expected Call Edge finance.process_invoice -> sales.calculate_tax not found")
	}
}

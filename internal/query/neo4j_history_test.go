//go:build integration

package query

import (
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func TestUpdateFileHistory(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	cleanup(t, p)
	defer cleanup(t, p)

	// Setup fixture: File nodes
	setupQuery := `
		CREATE (:File {file: 'Test_file1.go', id: 'Test_file1.go'})
		CREATE (:File {file: 'Test_file2.go', id: 'Test_file2.go'})
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	metrics := map[string]FileHistoryMetrics{
		"Test_file1.go": {
			ChangeFrequency: 10,
			LastChanged:     "2023-01-01T10:00:00Z",
			CoChanges:       []string{"Test_file2.go"},
		},
		"Test_file2.go": {
			ChangeFrequency: 5,
			LastChanged:     "2023-01-02T10:00:00Z",
			CoChanges:       []string{"Test_file1.go"},
		},
	}

	err = p.UpdateFileHistory(metrics)
	if err != nil {
		t.Fatalf("UpdateFileHistory failed: %v", err)
	}

	// Verify
	verifyQuery := `
		MATCH (f:File)
		WHERE f.file STARTS WITH 'Test_'
		RETURN f.file as file, f.change_frequency as freq, f.last_changed as last, f.co_changes as co
	`
	result, err := neo4j.ExecuteQuery(p.ctx, p.driver, verifyQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to verify: %v", err)
	}

	if len(result.Records) != 2 {
		t.Errorf("Expected 2 records, got %d", len(result.Records))
	}

	for _, record := range result.Records {
		file, _, _ := neo4j.GetRecordValue[string](record, "file")
		freqVal, _ := record.Get("freq")
		if freqVal == nil {
			t.Errorf("%s: freq is nil", file)
			continue
		}
		freq := freqVal.(int64)
		last, _, _ := neo4j.GetRecordValue[string](record, "last")
		coVal, _ := record.Get("co")
		var co []any
		if coVal != nil {
			co = coVal.([]any)
		}

		expected := metrics[file]
		if freq != int64(expected.ChangeFrequency) {
			t.Errorf("%s: expected freq %d, got %d", file, expected.ChangeFrequency, freq)
		}
		if last != expected.LastChanged {
			t.Errorf("%s: expected last %s, got %s", file, expected.LastChanged, last)
		}
		if len(co) != len(expected.CoChanges) {
			t.Errorf("%s: expected %d co-changes, got %d", file, len(expected.CoChanges), len(co))
		}
	}
}

func TestGetHotspots(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	cleanup(t, p)
	defer cleanup(t, p)

	// Setup fixture: Function and File nodes
	setupQuery := `
		CREATE (f1:Function {name: 'Test_HighRiskChurn', id: 'f1', risk_score: 0.9})
		CREATE (file1:File {file: 'Test_churny.go', id: 'file1', change_frequency: 100})
		CREATE (f1)-[:DEFINED_IN]->(file1)

		CREATE (f2:Function {name: 'Test_LowRiskChurn', id: 'f2', risk_score: 0.1})
		CREATE (file2:File {file: 'Test_stable.go', id: 'file2', change_frequency: 1})
		CREATE (f2)-[:DEFINED_IN]->(file2)
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, setupQuery, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to setup fixture: %v", err)
	}

	hotspots, err := p.GetHotspots("Test_.*")
	if err != nil {
		t.Fatalf("GetHotspots failed: %v", err)
	}

	if len(hotspots) != 2 {
		t.Errorf("Expected 2 hotspots, got %d", len(hotspots))
	} else {
		if hotspots[0].Name != "Test_HighRiskChurn" {
			t.Errorf("Expected first hotspot to be Test_HighRiskChurn, got %s", hotspots[0].Name)
		}
	}
}

func TestGetHotspots_MissingData(t *testing.T) {
	p := getProvider(t)
	defer p.Close()
	cleanup(t, p)
	defer cleanup(t, p)

	// Ensure NO risk_score data exists
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, `
		MATCH (f:Function) REMOVE f.risk_score
	`, nil, neo4j.EagerResultTransformer)
	if err != nil {
		t.Fatalf("Failed to clear risk_score data: %v", err)
	}

	// Test
	_, err = p.GetHotspots(".*")
	if err == nil {
		t.Fatal("Expected error when risk_score data is missing, got nil")
	}

	expectedErr := "risk score data is missing. Run 'graphdb enrich-contamination' first"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%v'", expectedErr, err)
	}
}

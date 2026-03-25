package query

import (
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// GetHotspots retrieves functions sorted by risk and file change frequency.
func (p *Neo4jProvider) GetHotspots(modulePattern string) ([]*HotspotResult, error) {
	// Pre-flight check: Is risk_score data present?
	checkQuery := `MATCH (f:Function) WHERE f.risk_score IS NOT NULL RETURN count(f) AS count LIMIT 1`
	checkRes, err := p.executeQuery(checkQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("pre-flight check failed: %w", err)
	}
	if len(checkRes.Records) == 0 {
		return nil, fmt.Errorf("risk score data is missing. Run 'graphdb enrich-contamination' first")
	}
	count, _, _ := neo4j.GetRecordValue[int64](checkRes.Records[0], "count")
	if count == 0 {
		return nil, fmt.Errorf("risk score data is missing. Run 'graphdb enrich-contamination' first")
	}

	query := `
		MATCH (f:Function)-[:DEFINED_IN]->(fi:File)
		WHERE fi.file =~ $pattern
		RETURN f.name as name, fi.file as file, f.risk_score as risk,
		       coalesce(fi.change_frequency, 0) as churn
		ORDER BY (coalesce(f.risk_score, 0) * coalesce(fi.change_frequency, 0)) DESC, f.risk_score DESC
		LIMIT 20
	`
	result, err := p.executeQuery(query, map[string]any{
		"pattern": modulePattern,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to execute GetHotspots query: %w", err)
	}

	hotspots := make([]*HotspotResult, 0, len(result.Records))
	for _, record := range result.Records {
		name, _, _ := neo4j.GetRecordValue[string](record, "name")
		file, _, _ := neo4j.GetRecordValue[string](record, "file")

		var risk float64
		if riskVal, ok := record.Get("risk"); ok && riskVal != nil {
			switch v := riskVal.(type) {
			case float64:
				risk = v
			case int64:
				risk = float64(v)
			case int:
				risk = float64(v)
			}
		}

		var churn int
		if churnVal, ok := record.Get("churn"); ok && churnVal != nil {
			switch v := churnVal.(type) {
			case int64:
				churn = int(v)
			case int:
				churn = v
			}
		}

		hotspots = append(hotspots, &HotspotResult{
			Name:  name,
			File:  file,
			Risk:  risk,
			Churn: churn,
		})
	}

	return hotspots, nil
}

// UpdateFileHistory updates git history metrics for File nodes in batches.
func (p *Neo4jProvider) UpdateFileHistory(metrics map[string]FileHistoryMetrics) error {
	batchSize := 500
	var batch []map[string]any

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}

		query := `
			UNWIND $batch AS row
			MATCH (file:File {file: row.file})
			SET file.change_frequency = row.change_frequency,
			    file.last_changed = row.last_changed,
			    file.co_changes = row.co_changes
		`

		_, err := p.executeQuery(query, map[string]any{
			"batch": batch,
		})

		batch = batch[:0] // reset
		return err
	}

	for filePath, metric := range metrics {
		batch = append(batch, map[string]any{
			"file":             filePath,
			"change_frequency": metric.ChangeFrequency,
			"last_changed":     metric.LastChanged,
			"co_changes":       metric.CoChanges,
		})

		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return fmt.Errorf("failed to flush file history batch: %w", err)
			}
		}
	}

	if err := flush(); err != nil {
		return fmt.Errorf("failed to flush final file history batch: %w", err)
	}

	return nil
}

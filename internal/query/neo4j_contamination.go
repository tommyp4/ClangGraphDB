package query

import (
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// SeedContamination applies heuristic rules to seed initial contamination flags.
func (p *Neo4jProvider) SeedContamination(modulePattern string, rules []ContaminationRule) error {
	for _, rule := range rules {
		var query string
		params := map[string]any{
			"pattern": modulePattern,
			"rule_pattern": rule.Pattern,
		}

		property := rule.Layer + "_contaminated"

		if rule.Type == "file" {
			// Seed based on file path
			query = fmt.Sprintf(`
				MATCH (f:Function)-[:DEFINED_IN]->(file:File)
				WHERE file.file =~ $pattern AND file.file =~ $rule_pattern
				SET f.%s = true
				RETURN count(f) as count
			`, property)
		} else if rule.Type == "function" {
			if rule.Heuristic == "path" {
				// Seed based on function name
				query = fmt.Sprintf(`
					MATCH (f:Function)-[:DEFINED_IN]->(file:File)
					WHERE file.file =~ $pattern AND f.name =~ $rule_pattern
					SET f.%s = true
					RETURN count(f) as count
				`, property)
			} else if rule.Heuristic == "content" {
				// Seed based on function content/body (if available)
				query = fmt.Sprintf(`
					MATCH (f:Function)-[:DEFINED_IN]->(file:File)
					WHERE file.file =~ $pattern AND f.content =~ $rule_pattern
					SET f.%s = true
					RETURN count(f) as count
				`, property)
			}
		}

		if query != "" {
			_, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, params, neo4j.EagerResultTransformer)
			if err != nil {
				return fmt.Errorf("failed to apply contamination rule (%s): %w", rule.Layer, err)
			}
		}
	}

	return nil
}

// PropagateContamination walks the CALLS graph to propagate contamination flags.
// It uses a repetitive approach to ensure all reachable nodes are marked.
func (p *Neo4jProvider) PropagateContamination(layer string) error {
	property := layer + "_contaminated"
	
	// Cypher query to propagate one level at a time.
	// We run this until no more nodes are updated.
	query := fmt.Sprintf(`
		MATCH (n:Function {%s: true})-[:CALLS]->(m:Function)
		WHERE m.%s IS NULL OR m.%s = false
		WITH m LIMIT 5000
		SET m.%s = true
		RETURN count(m) as count
	`, property, property, property, property)

	for {
		res, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, nil, neo4j.EagerResultTransformer)
		if err != nil {
			return fmt.Errorf("failed to propagate contamination for %s: %w", layer, err)
		}

		if len(res.Records) == 0 {
			break
		}
		
		count, _, _ := neo4j.GetRecordValue[int64](res.Records[0], "count")
		if count == 0 {
			break
		}
		// Continue propagating until no more nodes are updated
	}

	return nil
}

// CalculateRiskScores calculates risk scores for functions based on fan-in, fan-out, and contamination.
// Formula: risk_score = normalize(fan_in * 0.4 + fan_out * 0.3 + contamination_layers * 0.3)
func (p *Neo4jProvider) CalculateRiskScores() error {
	// 1. Calculate raw scores and store them temporarily
	// We use the number of incoming CALLS (fan-in), outgoing CALLS (fan-out),
	// and count how many contamination flags are set.
	query := `
		MATCH (f:Function)
		OPTIONAL MATCH (f)<-[:CALLS]-(caller)
		WITH f, count(caller) as fan_in
		OPTIONAL MATCH (f)-[:CALLS]->(callee)
		WITH f, fan_in, count(callee) as fan_out
		WITH f, fan_in, fan_out,
		     (CASE WHEN f.ui_contaminated = true THEN 1 ELSE 0 END +
		      CASE WHEN f.db_contaminated = true THEN 1 ELSE 0 END +
		      CASE WHEN f.io_contaminated = true THEN 1 ELSE 0 END) as contam_count
		SET f.raw_risk_score = (fan_in * 0.4 + fan_out * 0.3 + contam_count * 5.0)
		RETURN max(f.raw_risk_score) as max_score
	`
	res, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, nil, neo4j.EagerResultTransformer)
	if err != nil {
		return fmt.Errorf("failed to calculate raw risk scores: %w", err)
	}

	if len(res.Records) == 0 {
		return nil
	}

	maxScore, _, _ := neo4j.GetRecordValue[float64](res.Records[0], "max_score")
	if maxScore == 0 {
		maxScore = 1.0
	}

	// 2. Normalize scores to [0.0, 1.0]
	normalizeQuery := `
		MATCH (f:Function)
		WHERE f.raw_risk_score IS NOT NULL
		SET f.risk_score = f.raw_risk_score / $max_score
		REMOVE f.raw_risk_score
	`
	_, err = neo4j.ExecuteQuery(p.ctx, p.driver, normalizeQuery, map[string]any{"max_score": maxScore}, neo4j.EagerResultTransformer)
	if err != nil {
		return fmt.Errorf("failed to normalize risk scores: %w", err)
	}

	return nil
}

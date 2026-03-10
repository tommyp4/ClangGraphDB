package query

import (
	"fmt"
	"graphdb/internal/graph"
	"graphdb/internal/loader"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// GetUnextractedFunctions returns functions that haven't had atomic features extracted yet.
func (p *Neo4jProvider) GetUnextractedFunctions(limit int) ([]*graph.Node, error) {
	query := `
		MATCH (n:Function)
		WHERE n.atomic_features IS NULL AND n.file IS NOT NULL AND n.start_line IS NOT NULL
		RETURN n.id as id, n.file as file, n.start_line as start, n.end_line as end
		LIMIT $limit
	`
	result, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, map[string]any{
		"limit": limit,
	}, neo4j.EagerResultTransformer)

	if err != nil {
		return nil, fmt.Errorf("failed to get unextracted functions: %w", err)
	}

	nodes := make([]*graph.Node, 0, len(result.Records))
	for _, record := range result.Records {
		id, _, _ := neo4j.GetRecordValue[string](record, "id")
		file, _, _ := neo4j.GetRecordValue[string](record, "file")
		start, _, _ := neo4j.GetRecordValue[int64](record, "start")
		end, _, _ := neo4j.GetRecordValue[int64](record, "end")

		nodes = append(nodes, &graph.Node{
			ID:    id,
			Label: "Function",
			Properties: map[string]any{
				"file":       file,
				"start_line": start,
				"end_line":   end,
			},
		})
	}
	return nodes, nil
}

// UpdateAtomicFeatures saves the extracted atomic features for a node.
func (p *Neo4jProvider) UpdateAtomicFeatures(id string, features []string) error {
	query := `
		MATCH (n:Function {id: $id})
		SET n.atomic_features = $features
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, map[string]any{
		"id":       id,
		"features": features,
	}, neo4j.EagerResultTransformer)

	if err != nil {
		return fmt.Errorf("failed to update atomic features for %s: %w", id, err)
	}
	return nil
}

// GetUnembeddedNodes returns functions and features that lack an embedding.
func (p *Neo4jProvider) GetUnembeddedNodes(limit int) ([]*graph.Node, error) {
	query := `
		MATCH (n)
		WHERE (n:Function OR n:Feature) AND n.embedding IS NULL
		RETURN n.id as id, labels(n)[0] as label, properties(n) as props
		LIMIT $limit
	`
	result, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, map[string]any{
		"limit": limit,
	}, neo4j.EagerResultTransformer)

	if err != nil {
		return nil, fmt.Errorf("failed to get unembedded nodes: %w", err)
	}

	nodes := make([]*graph.Node, 0, len(result.Records))
	for _, record := range result.Records {
		id, _, _ := neo4j.GetRecordValue[string](record, "id")
		label, _, _ := neo4j.GetRecordValue[string](record, "label")
		props, _, _ := neo4j.GetRecordValue[map[string]any](record, "props")

		nodes = append(nodes, &graph.Node{
			ID:         id,
			Label:      label,
			Properties: sanitizeProperties(props),
		})
	}
	return nodes, nil
}

// UpdateEmbeddings updates the embedding vector for a node.
func (p *Neo4jProvider) UpdateEmbeddings(id string, embedding []float32) error {
	query := `
		MATCH (n {id: $id})
		SET n.embedding = $embedding
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, map[string]any{
		"id":        id,
		"embedding": embedding,
	}, neo4j.EagerResultTransformer)

	if err != nil {
		return fmt.Errorf("failed to update embedding for %s: %w", id, err)
	}
	return nil
}

// GetEmbeddingsOnly returns all IDs and their embeddings from the graph.
func (p *Neo4jProvider) GetEmbeddingsOnly() (map[string][]float32, error) {
	query := `
		MATCH (n)
		WHERE (n:Function OR n:Feature) AND n.embedding IS NOT NULL
		RETURN n.id as id, n.embedding as embedding
	`
	result, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, nil, neo4j.EagerResultTransformer)

	if err != nil {
		return nil, fmt.Errorf("failed to get embeddings: %w", err)
	}

	embeddings := make(map[string][]float32, len(result.Records))
	for _, record := range result.Records {
		id, _, _ := neo4j.GetRecordValue[string](record, "id")
		
		// Neo4j returns float arrays as []any containing float64
		embeddingRaw, _, _ := neo4j.GetRecordValue[[]any](record, "embedding")
		
		emb := make([]float32, len(embeddingRaw))
		for i, v := range embeddingRaw {
			if f64, ok := v.(float64); ok {
				emb[i] = float32(f64)
			}
		}
		
		embeddings[id] = emb
	}
	return embeddings, nil
}

// GetFunctionMetadata returns all functions with minimal properties (id, file, line, end_line, atomic_features) for clustering.
func (p *Neo4jProvider) GetFunctionMetadata() ([]*graph.Node, error) {
	query := `
		MATCH (n:Function)
		RETURN n.id as id, n.name as name, n.file as file, n.line as line, n.end_line as end_line, n.atomic_features as atomic_features
	`
	result, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, nil, neo4j.EagerResultTransformer)

	if err != nil {
		return nil, fmt.Errorf("failed to get function metadata: %w", err)
	}

	nodes := make([]*graph.Node, 0, len(result.Records))
	for _, record := range result.Records {
		id, _, _ := neo4j.GetRecordValue[string](record, "id")
		name, _, _ := neo4j.GetRecordValue[string](record, "name")
		file, _, _ := neo4j.GetRecordValue[string](record, "file")
		line, _, _ := neo4j.GetRecordValue[int64](record, "line")
		endLine, _, _ := neo4j.GetRecordValue[int64](record, "end_line")
		
		var atomicFeatures []string
		if val, found := record.Get("atomic_features"); found && val != nil {
			if rawAf, ok := val.([]any); ok {
				for _, v := range rawAf {
					if s, okStr := v.(string); okStr {
						atomicFeatures = append(atomicFeatures, s)
					}
				}
			}
		}

		nodes = append(nodes, &graph.Node{
			ID:    id,
			Label: "Function",
			Properties: map[string]any{
				"name":            name,
				"file":            file,
				"line":            line,
				"end_line":        endLine,
				"atomic_features": atomicFeatures,
			},
		})
	}
	return nodes, nil
}

// GetUnnamedFeatures returns features without a generated name/summary.
func (p *Neo4jProvider) GetUnnamedFeatures(limit int) ([]*graph.Node, error) {
	query := `
		MATCH (n:Feature)
		WHERE coalesce(n.name, '') = '' OR coalesce(n.summary, '') = ''
		RETURN n.id as id, properties(n) as props
		LIMIT $limit
	`
	result, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, map[string]any{
		"limit": limit,
	}, neo4j.EagerResultTransformer)

	if err != nil {
		return nil, fmt.Errorf("failed to get unnamed features: %w", err)
	}

	nodes := make([]*graph.Node, 0, len(result.Records))
	for _, record := range result.Records {
		id, _, _ := neo4j.GetRecordValue[string](record, "id")
		props, _, _ := neo4j.GetRecordValue[map[string]any](record, "props")

		nodes = append(nodes, &graph.Node{
			ID:         id,
			Label:      "Feature",
			Properties: sanitizeProperties(props),
		})
	}
	return nodes, nil
}

// CountUnnamedFeatures returns the total number of features without a name/summary.
func (p *Neo4jProvider) CountUnnamedFeatures() (int64, error) {
	query := `
		MATCH (n:Feature)
		WHERE coalesce(n.name, '') = '' OR coalesce(n.summary, '') = ''
		RETURN count(n) as total
	`
	result, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, nil, neo4j.EagerResultTransformer)
	if err != nil {
		return 0, fmt.Errorf("failed to count unnamed features: %w", err)
	}
	if len(result.Records) == 0 {
		return 0, nil
	}
	total, _, _ := neo4j.GetRecordValue[int64](result.Records[0], "total")
	return total, nil
}

// UpdateFeatureTopology writes feature nodes and relationships to the graph.
func (p *Neo4jProvider) UpdateFeatureTopology(nodes []*graph.Node, edges []*graph.Edge) error {
	if len(nodes) == 0 && len(edges) == 0 {
		return nil
	}

	session := p.driver.NewSession(p.ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(p.ctx)

	_, err := session.ExecuteWrite(p.ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// 1. Batch insert nodes
		if len(nodes) > 0 {
			nodeBatches := make([]map[string]any, 0, len(nodes))
			for _, n := range nodes {
				props := make(map[string]any)
				if n.Properties != nil {
					for k, v := range n.Properties {
						props[k] = v
					}
				}
				props["id"] = n.ID
				nodeBatches = append(nodeBatches, props)
			}
			query := `
				UNWIND $batch AS row
				MERGE (f:Feature {id: row.id})
				SET f += row
			`
			_, txErr := tx.Run(p.ctx, query, map[string]any{"batch": nodeBatches})
			if txErr != nil {
				return nil, fmt.Errorf("failed to batch load feature nodes: %w", txErr)
			}
		}

		// 2. Batch insert edges
		if len(edges) > 0 {
			// Group edges by type since we need to put the type in the Cypher
			edgeGroups := make(map[string][]map[string]any)
			for _, e := range edges {
				edgeGroups[e.Type] = append(edgeGroups[e.Type], map[string]any{
					"sourceId": e.SourceID,
					"targetId": e.TargetID,
				})
			}

			for relType, batch := range edgeGroups {
				// Ensure label is sanitized
				sanitizedRelType := loader.SanitizeLabel(relType)
				query := fmt.Sprintf(`
					UNWIND $batch AS row
					MATCH (source {id: row.sourceId})
					MATCH (target {id: row.targetId})
					MERGE (source)-[r:%s]->(target)
				`, sanitizedRelType)
				_, txErr := tx.Run(p.ctx, query, map[string]any{"batch": batch})
				if txErr != nil {
					return nil, fmt.Errorf("failed to batch load edges of type %s: %w", relType, txErr)
				}
			}
		}
		return nil, nil
	})

	if err != nil {
		return fmt.Errorf("failed to update feature topology: %w", err)
	}

	return nil
}

// UpdateFeatureSummary saves the generated name and summary for a feature.
func (p *Neo4jProvider) UpdateFeatureSummary(id string, name string, summary string) error {
	query := `
		MATCH (n:Feature {id: $id})
		SET n.name = $name, n.summary = $summary
	`
	_, err := neo4j.ExecuteQuery(p.ctx, p.driver, query, map[string]any{
		"id":      id,
		"name":    name,
		"summary": summary,
	}, neo4j.EagerResultTransformer)

	if err != nil {
		return fmt.Errorf("failed to update feature summary for %s: %w", id, err)
	}
	return nil
}

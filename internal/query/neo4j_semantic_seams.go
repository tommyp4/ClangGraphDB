package query

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// GetSemanticSeams finds pairs of functions within the same file/class that have low semantic similarity.
func (p *Neo4jProvider) GetSemanticSeams(ctx context.Context, similarityThreshold float64) ([]*SemanticSeamResult, error) {
	query := `
      MATCH (container)-[:DEFINES|DEFINED_IN]-(f1:Function),
            (container)-[:DEFINES|DEFINED_IN]-(f2:Function)
      WHERE (container:File OR container:Class)
        AND f1.id < f2.id 
        AND f1.embedding IS NOT NULL 
        AND f2.embedding IS NOT NULL
      WITH container, f1, f2, vector.similarity.cosine(f1.embedding, f2.embedding) as similarity
      WHERE similarity < $threshold
      RETURN coalesce(container.name, container.file, container.id) as container, 
             f1.name as methodA, f2.name as methodB, similarity
      ORDER BY similarity ASC
      LIMIT 50
	`
	params := map[string]any{
		"threshold": similarityThreshold,
	}

	res, err := p.executeQuery(query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get semantic seams: %w", err)
	}

	results := []*SemanticSeamResult{}
	for _, record := range res.Records {
		container, _, _ := neo4j.GetRecordValue[string](record, "container")
		methodA, _, _ := neo4j.GetRecordValue[string](record, "methodA")
		methodB, _, _ := neo4j.GetRecordValue[string](record, "methodB")
		similarity, _, _ := neo4j.GetRecordValue[float64](record, "similarity")

		results = append(results, &SemanticSeamResult{
			Container:  container,
			MethodA:    methodA,
			MethodB:    methodB,
			Similarity: similarity,
		})
	}

	return results, nil
}

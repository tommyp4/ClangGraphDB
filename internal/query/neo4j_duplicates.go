package query

import (
	"fmt"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// FindDuplicates finds pairs of duplicated functions using vector embeddings globally.
func (p *Neo4jProvider) FindDuplicates(similarityThreshold float64, limit int) ([]*DuplicateResult, error) {
	query := `
      // Find Function Duplicates
      MATCH (f1:Function), (f2:Function)
      WHERE f1.id < f2.id 
        AND f1.embedding IS NOT NULL 
        AND f2.embedding IS NOT NULL
      WITH f1, f2, vector.similarity.cosine(f1.embedding, f2.embedding) as similarity
      WHERE similarity >= $threshold
      RETURN f1.name as functionA, f1.id as idA, 
             f2.name as functionB, f2.id as idB, similarity
      ORDER BY similarity DESC
      LIMIT $limit
	`
	params := map[string]any{
		"threshold": similarityThreshold,
		"limit":     limit,
	}

	res, err := p.executeQuery(query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to find duplicates: %w", err)
	}

	results := []*DuplicateResult{}
	for _, record := range res.Records {
		functionA, _, _ := neo4j.GetRecordValue[string](record, "functionA")
		idA, _, _ := neo4j.GetRecordValue[string](record, "idA")
		functionB, _, _ := neo4j.GetRecordValue[string](record, "functionB")
		idB, _, _ := neo4j.GetRecordValue[string](record, "idB")
		similarity, _, _ := neo4j.GetRecordValue[float64](record, "similarity")

		results = append(results, &DuplicateResult{
			FunctionA:  functionA,
			IDA:        idA,
			FunctionB:  functionB,
			IDB:        idB,
			Similarity: similarity,
		})
	}

	return results, nil
}

package query

import (
	"graphdb/internal/graph"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// WhatIf computes the impact of hypothetical node removals.
func (p *Neo4jProvider) WhatIf(targets []string) (*WhatIfResult, error) {
	result := &WhatIfResult{
		SeveredEdges:       []*graph.Edge{},
		OrphanedNodes:      []*graph.Node{},
		CrossBoundaryCalls: []*graph.Edge{},
		SharedState:        []*graph.Node{},
	}

	// 1. Severed Edges
	// Edges from Outside to Inside OR Inside to Outside
	severedQuery := `
		MATCH (n)-[r]->(m)
		WHERE (n.id IN $targets AND NOT m.id IN $targets)
		   OR (NOT n.id IN $targets AND m.id IN $targets)
		RETURN n.id as source, m.id as target, type(r) as type
	`
	res, err := neo4j.ExecuteQuery(p.ctx, p.driver, severedQuery, map[string]any{"targets": targets}, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, err
	}
	for _, record := range res.Records {
		source, _ := record.Get("source")
		target, _ := record.Get("target")
		relType, _ := record.Get("type")
		result.SeveredEdges = append(result.SeveredEdges, &graph.Edge{
			SourceID: source.(string),
			TargetID: target.(string),
			Type:     relType.(string),
		})
	}

	// 2. Cross-Boundary Calls (subset of severed edges)
	// Specifically CALLS from Outside to Inside
	crossQuery := `
		MATCH (n:Function)-[r:CALLS]->(m:Function)
		WHERE NOT n.id IN $targets AND m.id IN $targets
		RETURN n.id as source, m.id as target, type(r) as type
	`
	res, err = neo4j.ExecuteQuery(p.ctx, p.driver, crossQuery, map[string]any{"targets": targets}, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, err
	}
	for _, record := range res.Records {
		source, _ := record.Get("source")
		target, _ := record.Get("target")
		relType, _ := record.Get("type")
		result.CrossBoundaryCalls = append(result.CrossBoundaryCalls, &graph.Edge{
			SourceID: source.(string),
			TargetID: target.(string),
			Type:     relType.(string),
		})
	}

	// 3. Shared State
	// Globals used by both Inside and Outside
	sharedQuery := `
		MATCH (n:Function)-[:USES_GLOBAL]->(g:Global)
		WHERE n.id IN $targets
		WITH g
		MATCH (m:Function)-[:USES_GLOBAL]->(g)
		WHERE NOT m.id IN $targets
		RETURN DISTINCT g
	`
	res, err = neo4j.ExecuteQuery(p.ctx, p.driver, sharedQuery, map[string]any{"targets": targets}, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, err
	}
	for _, record := range res.Records {
		node, _ := record.Get("g")
		neoNode := node.(neo4j.Node)
		id, _ := neoNode.Props["id"].(string)
		label := ""
		if len(neoNode.Labels) > 0 {
			label = neoNode.Labels[0]
		}
		result.SharedState = append(result.SharedState, &graph.Node{
			ID:         id,
			Label:      label,
			Properties: neoNode.Props,
		})
	}

	// 4. Orphaned Nodes
	// Nodes not in targets that only have incoming edges from targets (or nothing)
	// Wait, "unreachable from any non-extracted node" is better.
	// But let's start simple: Nodes whose incoming edges are ALL from targets.
	orphanedQuery := `
		MATCH (m)
		WHERE NOT m.id IN $targets
		  AND NOT m:File // Files aren't really orphaned in this sense
		  AND NOT m:Global // Globals are handled by shared state
		  AND EXISTS { MATCH (n)-[:CALLS]->(m) } // Must have at least one incoming call
		  AND ALL(n IN [(p)-[:CALLS]->(m) | p] WHERE n.id IN $targets)
		RETURN m
	`
	res, err = neo4j.ExecuteQuery(p.ctx, p.driver, orphanedQuery, map[string]any{"targets": targets}, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, err
	}
	for _, record := range res.Records {
		node, _ := record.Get("m")
		neoNode := node.(neo4j.Node)
		id, _ := neoNode.Props["id"].(string)
		label := ""
		if len(neoNode.Labels) > 0 {
			label = neoNode.Labels[0]
		}
		result.OrphanedNodes = append(result.OrphanedNodes, &graph.Node{
			ID:         id,
			Label:      label,
			Properties: neoNode.Props,
		})
	}

	return result, nil
}

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
		AffectedNodes:      []*graph.Node{},
	}

	affectedNodesMap := make(map[string]*graph.Node)

	// 1. Severed Edges
	// Edges from Outside to Inside OR Inside to Outside
	severedQuery := `
		// What-If: Severed Edges
		MATCH (n)-[r]->(m)
		WHERE (n.id IN $targets AND NOT m.id IN $targets)
		   OR (NOT n.id IN $targets AND m.id IN $targets)
		RETURN n as sourceNode, m as targetNode, type(r) as type
	`
	res, err := p.executeQuery(severedQuery, map[string]any{"targets": targets})
	if err != nil {
		return nil, err
	}
	for _, record := range res.Records {
		source, _ := record.Get("sourceNode")
		target, _ := record.Get("targetNode")
		relType, _ := record.Get("type")

		sourceNode := source.(neo4j.Node)
		targetNode := target.(neo4j.Node)

		sourceID := ""
		if id, ok := sourceNode.Props["id"].(string); ok {
			sourceID = id
		} else if name, ok := sourceNode.Props["name"].(string); ok {
			sourceID = name
		}

		targetID := ""
		if id, ok := targetNode.Props["id"].(string); ok {
			targetID = id
		} else if name, ok := targetNode.Props["name"].(string); ok {
			targetID = name
		}

		// Add source node if not already added
		if _, exists := affectedNodesMap[sourceID]; !exists {
			label := ""
			if len(sourceNode.Labels) > 0 {
				label = sourceNode.Labels[0]
			}
			node := &graph.Node{
				ID:         sourceID,
				Label:      label,
				Properties: sanitizeProperties(sourceNode.Props),
			}
			affectedNodesMap[sourceID] = node
			result.AffectedNodes = append(result.AffectedNodes, node)
		}

		// Add target node if not already added
		if _, exists := affectedNodesMap[targetID]; !exists {
			label := ""
			if len(targetNode.Labels) > 0 {
				label = targetNode.Labels[0]
			}
			node := &graph.Node{
				ID:         targetID,
				Label:      label,
				Properties: sanitizeProperties(targetNode.Props),
			}
			affectedNodesMap[targetID] = node
			result.AffectedNodes = append(result.AffectedNodes, node)
		}

		result.SeveredEdges = append(result.SeveredEdges, &graph.Edge{
			SourceID: sourceID,
			TargetID: targetID,
			Type:     relType.(string),
		})
	}

	// 2. Cross-Boundary Calls (subset of severed edges)
	// Specifically CALLS from Outside to Inside
	crossQuery := `
		// What-If: Cross-Boundary Calls
		MATCH (n:Function)-[r:CALLS]->(m:Function)
		WHERE NOT n.id IN $targets AND m.id IN $targets
		RETURN n as sourceNode, m as targetNode, type(r) as type
	`
	res, err = p.executeQuery(crossQuery, map[string]any{"targets": targets})
	if err != nil {
		return nil, err
	}
	for _, record := range res.Records {
		source, _ := record.Get("sourceNode")
		target, _ := record.Get("targetNode")
		relType, _ := record.Get("type")

		sourceNode := source.(neo4j.Node)
		targetNode := target.(neo4j.Node)

		sourceID := ""
		if id, ok := sourceNode.Props["id"].(string); ok {
			sourceID = id
		} else if name, ok := sourceNode.Props["name"].(string); ok {
			sourceID = name
		}

		targetID := ""
		if id, ok := targetNode.Props["id"].(string); ok {
			targetID = id
		} else if name, ok := targetNode.Props["name"].(string); ok {
			targetID = name
		}

		// Add source node if not already added
		if _, exists := affectedNodesMap[sourceID]; !exists {
			label := ""
			if len(sourceNode.Labels) > 0 {
				label = sourceNode.Labels[0]
			}
			node := &graph.Node{
				ID:         sourceID,
				Label:      label,
				Properties: sanitizeProperties(sourceNode.Props),
			}
			affectedNodesMap[sourceID] = node
			result.AffectedNodes = append(result.AffectedNodes, node)
		}

		// Add target node if not already added
		if _, exists := affectedNodesMap[targetID]; !exists {
			label := ""
			if len(targetNode.Labels) > 0 {
				label = targetNode.Labels[0]
			}
			node := &graph.Node{
				ID:         targetID,
				Label:      label,
				Properties: sanitizeProperties(targetNode.Props),
			}
			affectedNodesMap[targetID] = node
			result.AffectedNodes = append(result.AffectedNodes, node)
		}

		result.CrossBoundaryCalls = append(result.CrossBoundaryCalls, &graph.Edge{
			SourceID: sourceID,
			TargetID: targetID,
			Type:     relType.(string),
		})
	}

	// 3. Shared State
	// Globals used by both Inside and Outside
	sharedQuery := `
		// What-If: Shared State
		MATCH (n:Function)-[:USES_GLOBAL]->(g:Global)
		WHERE n.id IN $targets
		WITH g
		MATCH (m:Function)-[:USES_GLOBAL]->(g)
		WHERE NOT m.id IN $targets
		RETURN DISTINCT g
	`
	res, err = p.executeQuery(sharedQuery, map[string]any{"targets": targets})
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
			Properties: sanitizeProperties(neoNode.Props),
		})
	}

	// 4. Orphaned Nodes
	// Nodes not in targets that only have incoming edges from targets (or nothing)
	// Wait, "unreachable from any non-extracted node" is better.
	// But let's start simple: Nodes whose incoming edges are ALL from targets.
	orphanedQuery := `
		// What-If: Orphaned Nodes
		MATCH (m)
		WHERE NOT m.id IN $targets
		  AND NOT m:File // Files aren't really orphaned in this sense
		  AND NOT m:Global // Globals are handled by shared state
		  AND EXISTS { MATCH (n)-[:CALLS]->(m) } // Must have at least one incoming call
		  AND ALL(n IN [(p)-[:CALLS]->(m) | p] WHERE n.id IN $targets)
		RETURN m
	`
	res, err = p.executeQuery(orphanedQuery, map[string]any{"targets": targets})
	if err != nil {
		return nil, err
	}
	for _, record := range res.Records {
		node, _ := record.Get("m")
		neoNode := node.(neo4j.Node)
		id := ""
		if idProp, ok := neoNode.Props["id"].(string); ok {
			id = idProp
		} else if nameProp, ok := neoNode.Props["name"].(string); ok {
			id = nameProp
		}

		label := ""
		if len(neoNode.Labels) > 0 {
			label = neoNode.Labels[0]
		}
		result.OrphanedNodes = append(result.OrphanedNodes, &graph.Node{
			ID:         id,
			Label:      label,
			Properties: sanitizeProperties(neoNode.Props),
		})
	}

	return result, nil
}

package rpg

import (
	"fmt"
	"graphdb/internal/graph"
	"log"
	"sort"
	"strings"
)

// GlobalEmbeddingClusterer wraps an existing Clusterer (like EmbeddingClusterer)
// to provide semantic naming for the resulting clusters, effectively turning
// them into "Latent Domains".
type GlobalEmbeddingClusterer struct {
	Inner                 Clusterer
	Summarizer            Summarizer
	Loader                SourceLoader
	PrecomputedEmbeddings map[string][]float32
}

func (c *GlobalEmbeddingClusterer) Cluster(nodes []graph.Node, domain string) (map[string][]graph.Node, error) {
	// 1. Delegate clustering to the inner clusterer (e.g., K-Means)
	// We pass "root" as the domain, but the inner clusterer usually appends numeric suffixes.
	rawClusters, err := c.Inner.Cluster(nodes, domain)
	if err != nil {
		return nil, err
	}

	namedClusters := make(map[string][]graph.Node)

	// 2. Process each raw cluster to generate a semantic name
	for _, clusterNodes := range rawClusters {
		if len(clusterNodes) == 0 {
			continue
		}

		// Calculate Centroid
		centroid := c.calculateCentroid(clusterNodes)

		// Find top representative nodes (closest to centroid)
		representatives := c.findRepresentatives(clusterNodes, centroid, 5)

		// Collect snippets
		snippets := c.collectSnippets(representatives)

		// Generate Name
		name, _, err := c.Summarizer.Summarize(snippets)
		if err != nil {
			log.Printf("Warning: domain summarization failed: %v", err)
			// Fallback if summarization fails
			name = "Domain-" + GenerateShortUUID()
		}

		// Ensure uniqueness if multiple clusters map to the same name (unlikely but possible)
		originalName := name
		counter := 1
		for {
			if _, exists := namedClusters[name]; !exists {
				break
			}
			name = fmt.Sprintf("%s %d", originalName, counter)
			counter++
		}

		namedClusters[name] = clusterNodes
	}

	return namedClusters, nil
}

func (c *GlobalEmbeddingClusterer) calculateCentroid(nodes []graph.Node) []float32 {
	if len(nodes) == 0 {
		return nil
	}
	
	// Assume all embeddings have same dimension. Find first valid one to get dim.
	var dim int
	for _, n := range nodes {
		if emb, ok := c.PrecomputedEmbeddings[n.ID]; ok {
			dim = len(emb)
			break
		}
	}
	if dim == 0 {
		return nil
	}

	centroid := make([]float32, dim)
	count := 0

	for _, n := range nodes {
		if emb, ok := c.PrecomputedEmbeddings[n.ID]; ok {
			for i := 0; i < dim; i++ {
				centroid[i] += emb[i]
			}
			count++
		}
	}

	if count > 0 {
		for i := 0; i < dim; i++ {
			centroid[i] /= float32(count)
		}
	}
	return centroid
}

type nodeDistance struct {
	Node     graph.Node
	Distance float32
}

func (c *GlobalEmbeddingClusterer) findRepresentatives(nodes []graph.Node, centroid []float32, limit int) []graph.Node {
	if centroid == nil {
		// If no centroid (no embeddings), just return the first few nodes
		if len(nodes) < limit {
			return nodes
		}
		return nodes[:limit]
	}

	distances := make([]nodeDistance, 0, len(nodes))
	for _, n := range nodes {
		if emb, ok := c.PrecomputedEmbeddings[n.ID]; ok {
			dist := cosineDistance(emb, centroid)
			distances = append(distances, nodeDistance{Node: n, Distance: dist})
		} else {
			// Penalty for missing embedding
			distances = append(distances, nodeDistance{Node: n, Distance: 2.0})
		}
	}

	// Sort by distance (ascending)
	sort.Slice(distances, func(i, j int) bool {
		return distances[i].Distance < distances[j].Distance
	})

	result := make([]graph.Node, 0, limit)
	for i := 0; i < len(distances) && i < limit; i++ {
		result = append(result, distances[i].Node)
	}
	return result
}

func (c *GlobalEmbeddingClusterer) collectSnippets(nodes []graph.Node) []string {
	snippets := make([]string, 0, len(nodes))
	for _, n := range nodes {
		// Try to read content from file
		if c.Loader != nil {
			file, okFile := n.Properties["file"].(string)
			line, okLine := getInt(n.Properties["line"])
			endLine, okEnd := getInt(n.Properties["end_line"])
			
			if okFile && okLine && okEnd {
				content, err := c.Loader(file, line, endLine)
				if err == nil && content != "" {
					if len(content) > 1000 {
						content = content[:1000] + "..."
					}
					snippets = append(snippets, content)
					continue
				}
			}
		}

		// Fallback to atomic features
		if af, ok := n.Properties["atomic_features"].([]string); ok && len(af) > 0 {
			snippets = append(snippets, "// Atomic features: "+strings.Join(af, ", "))
		}
	}
	return snippets
}

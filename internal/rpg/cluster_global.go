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

func (c *GlobalEmbeddingClusterer) Cluster(nodes []graph.Node, domain string) ([]ClusterGroup, error) {
	// 1. Delegate clustering to the inner clusterer (e.g., K-Means)
	// We pass "root" as the domain, but the inner clusterer usually appends numeric suffixes.
	rawClusters, err := c.Inner.Cluster(nodes, domain)
	if err != nil {
		return nil, err
	}

	namedClusters := make([]ClusterGroup, 0, len(rawClusters))
	usedNames := make(map[string]bool)

	log.Printf("Global clustering produced %d raw clusters, generating semantic names...", len(rawClusters))

	// 2. Process each raw cluster to generate a semantic name
	clusterIdx := 0
	for _, cluster := range rawClusters {
		clusterIdx++
		if len(cluster.Nodes) == 0 {
			continue
		}

		// Calculate Centroid
		centroid := c.calculateCentroid(cluster.Nodes)

		// Find top representative nodes (closest to centroid)
		representatives := c.findRepresentatives(cluster.Nodes, centroid, 5)

		// Collect snippets
		snippets := c.collectSnippets(representatives)

		// Generate Name
		log.Printf("  Naming domain %d/%d (%d functions)...", clusterIdx, len(rawClusters), len(cluster.Nodes))

		if c.Summarizer == nil {
			// Topology-only mode: use the raw cluster name, Builder will fallback to LCA naming
			namedClusters = append(namedClusters, ClusterGroup{
				Name:        cluster.Name,
				Description: "",
				Nodes:       cluster.Nodes,
			})
			continue
		}

		// Strategy 1: Hierarchical Prompting
		var previouslyNamed []string
		for k := range usedNames {
			previouslyNamed = append(previouslyNamed, k)
		}
		extraContext := strings.Join(previouslyNamed, ", ")

		name, description, err := c.Summarizer.Summarize(snippets, "domain", extraContext)
		if err != nil {
			return nil, fmt.Errorf("domain summarization failed for cluster %d: %w", clusterIdx, err)
		}

		// Ensure uniqueness if multiple clusters map to the same name (unlikely but possible)
		originalName := name
		counter := 1
		for {
			if !usedNames[name] {
				break
			}
			name = fmt.Sprintf("%s %d", originalName, counter)
			counter++
		}
		usedNames[name] = true

		namedClusters = append(namedClusters, ClusterGroup{
			Name:        name,
			Description: description,
			Nodes:       cluster.Nodes,
		})
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
			distances = append(distances, nodeDistance{Node: n, Distance: 2.0})
		}
	}

	// Sort by distance (ascending)
	sort.Slice(distances, func(i, j int) bool {
		return distances[i].Distance < distances[j].Distance
	})

	result := make([]graph.Node, 0, limit)
	
	if len(distances) <= limit {
		for i := 0; i < len(distances); i++ {
			result = append(result, distances[i].Node)
		}
		return result
	}
	
	// Strategy 1: Edge-Aware Sampling (2 from center, 3 from edges)
	centerCount := 2
	edgeCount := limit - centerCount
	for i := 0; i < centerCount; i++ {
		result = append(result, distances[i].Node)
	}
	for i := len(distances) - edgeCount; i < len(distances); i++ {
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
			startLine, okLine := getInt(n.Properties["start_line"])
			endLine, okEnd := getInt(n.Properties["end_line"])

			if okFile && okLine && okEnd {
				content, err := c.Loader(file, startLine, endLine)
				if err == nil && content != "" {
					if len(content) > 1000 {
						content = content[:1000] + "..."
					}
					content = "// File: " + file + "\n" + content
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

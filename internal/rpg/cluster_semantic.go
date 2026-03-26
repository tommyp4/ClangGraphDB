package rpg

import (
	"fmt"
	"graphdb/internal/embedding"
	"graphdb/internal/graph"
	"graphdb/internal/ui"
	"log"
	"math"
	"math/rand"
)

// EmbeddingClusterer groups functions by semantic similarity of their
// atomic_features using embedding vectors and K-Means clustering.
type EmbeddingClusterer struct {
	Embedder              embedding.Embedder
	MaxIterations         int // K-Means iterations; 0 defaults to 50
	PrecomputedEmbeddings map[string][]float32
	KStrategy             func(n int) int // Optional custom K calculation
	Seed                  int64           // Seed for K-Means initialization
	LogLabel              string          // Label for log messages (e.g. "domain", "cluster")
}

func (c *EmbeddingClusterer) Cluster(nodes []graph.Node, domain string) ([]ClusterGroup, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	// If too few nodes, put them all in one cluster
	if len(nodes) <= 3 {
		return []ClusterGroup{{
			Name:  "cluster-" + GenerateShortUUID(),
			Nodes: nodes,
		}}, nil
	}

	// 1. Prepare embeddings
	embeddings := make([][]float32, len(nodes))
	textsToEmbed := make([]string, 0)
	indicesToEmbed := make([]int, 0)

	for i, n := range nodes {
		if val, ok := c.PrecomputedEmbeddings[n.ID]; ok {
			embeddings[i] = val
		} else {
			text := NodeToText(n)
			// Fallback: try text lookup if ID fails (though ID is safer for map key)
			// Actually, let's stick to ID for precomputed map as it's unique.
			// But wait, the previous logic used text content.
			// If the precomputation used NodeToText, and stored by ID, we are good.
			// Let's assume PrecomputedEmbeddings is map[NodeID]Embedding.
			textsToEmbed = append(textsToEmbed, text)
			indicesToEmbed = append(indicesToEmbed, i)
		}
	}

	// 2. Embed missing
	if len(textsToEmbed) > 0 {
		if c.Embedder == nil {
			return nil, fmt.Errorf("missing embeddings for %d nodes and no Embedder provided", len(textsToEmbed))
		}
		newEmbeddings, err := c.Embedder.EmbedBatch(textsToEmbed)
		if err != nil {
			return nil, fmt.Errorf("embedding for clustering failed: %w", err)
		}
		if len(newEmbeddings) != len(textsToEmbed) {
			return nil, fmt.Errorf("expected %d embeddings, got %d", len(textsToEmbed), len(newEmbeddings))
		}
		for j, idx := range indicesToEmbed {
			embeddings[idx] = newEmbeddings[j]
		}
	}

	// 3. Determine K
	var k int
	if c.KStrategy != nil {
		k = c.KStrategy(len(nodes))
	} else {
		// Default: target 3-8 functions per cluster
		k = len(nodes) / 5
	}

	if k < 2 {
		k = 2
	}
	if k > len(nodes)/2 {
		k = len(nodes) / 2
	}

	// 4. Run K-Means
	maxIter := c.MaxIterations
	if maxIter <= 0 {
		maxIter = 50
	}
	label := c.LogLabel
	if label == "" {
		label = "feature"
	}
	log.Printf("Grouping %d functions into %d %ss...", len(nodes), k, label)
	assignments := kmeans(embeddings, k, maxIter, c.Seed, label, domain)

	// 5. Group nodes by cluster assignment
	groups := make([]ClusterGroup, k)
	for j := 0; j < k; j++ {
		groups[j] = ClusterGroup{
			Name:  "cluster-" + GenerateShortUUID(),
			Nodes: make([]graph.Node, 0),
		}
	}

	for i, clusterIdx := range assignments {
		groups[clusterIdx].Nodes = append(groups[clusterIdx].Nodes, nodes[i])
	}

	// Filter out empty groups
	finalGroups := make([]ClusterGroup, 0, k)
	for _, g := range groups {
		if len(g.Nodes) > 0 {
			finalGroups = append(finalGroups, g)
		}
	}

	return finalGroups, nil
}

// kmeans runs K-Means clustering on a set of vectors.
// Returns a slice of cluster assignments (one per input vector).
func kmeans(vectors [][]float32, k int, maxIterations int, seed int64, label string, context string) []int {
	n := len(vectors)
	if n == 0 || k <= 0 {
		return nil
	}
	dim := len(vectors[0])

	// Initialize centroids using K-Means++ initialization
	centroids := kmeansppInit(vectors, k, seed, label, context)

	assignments := make([]int, n)

	status := fmt.Sprintf("Refining %ss", label)
	if context != "" && context != "root" {
		status = fmt.Sprintf("Refining %ss [%s]", label, context)
	}
	pb := ui.NewSpinner(status)

	for iter := 0; iter < maxIterations; iter++ {
		changed := false
		pb.UpdateDescription(fmt.Sprintf("%s (pass %d)", status, iter+1))

		// Assign each vector to nearest centroid
		for i, v := range vectors {
			nearest := 0
			nearestDist := cosineDistance(v, centroids[0])
			for j := 1; j < k; j++ {
				d := cosineDistance(v, centroids[j])
				if d < nearestDist {
					nearestDist = d
					nearest = j
				}
			}
			if assignments[i] != nearest {
				assignments[i] = nearest
				changed = true
			}
		}

		if !changed {
			break
		}

		// Recompute centroids
		newCentroids := make([][]float32, k)
		counts := make([]int, k)
		for j := 0; j < k; j++ {
			newCentroids[j] = make([]float32, dim)
		}
		for i, v := range vectors {
			c := assignments[i]
			counts[c]++
			for d := 0; d < dim; d++ {
				newCentroids[c][d] += v[d]
			}
		}
		for j := 0; j < k; j++ {
			if counts[j] > 0 {
				for d := 0; d < dim; d++ {
					newCentroids[j][d] /= float32(counts[j])
				}
				centroids[j] = newCentroids[j]
			}
		}
	}

	pb.Finish()

	return assignments
}

// kmeansppInit selects initial centroids using K-Means++ algorithm.
func kmeansppInit(vectors [][]float32, k int, seed int64, label string, context string) [][]float32 {
	n := len(vectors)
	centroids := make([][]float32, 0, k)
	rng := rand.New(rand.NewSource(seed))

	// Pick first centroid randomly
	first := rng.Intn(n)
	centroids = append(centroids, vectors[first])

	if k > 1 {
		status := fmt.Sprintf("Seeding %ss", label)
		if context != "" && context != "root" {
			status = fmt.Sprintf("Seeding %ss [%s]", label, context)
		}
		pb := ui.NewProgressBar(int64(k), status)
		pb.Add(1) // for the first one

		for len(centroids) < k {
			// Compute distances to nearest centroid
			dists := make([]float64, n)
			total := 0.0
			for i, v := range vectors {
				minDist := math.MaxFloat64
				for _, c := range centroids {
					d := float64(cosineDistance(v, c))
					if d < minDist {
						minDist = d
					}
				}
				dists[i] = minDist * minDist
				total += dists[i]
			}

			// Weighted random selection
			r := rng.Float64() * total
			cumulative := 0.0
			chosen := 0
			for i, d := range dists {
				cumulative += d
				if cumulative >= r {
					chosen = i
					break
				}
			}
			centroids = append(centroids, vectors[chosen])
			pb.Add(1)
		}
		pb.Finish()
	}

	return centroids
}

// cosineDistance returns 1 - cosine_similarity between two vectors.
func cosineDistance(a, b []float32) float32 {
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 1.0
	}
	sim := dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
	return 1.0 - sim
}

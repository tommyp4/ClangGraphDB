package rpg

import (
	"fmt"
	"graphdb/internal/embedding"
	"graphdb/internal/graph"
	"graphdb/internal/query"
	"graphdb/internal/tools/snippet"
	"log"
	"math"
)

type Orchestrator struct {
	Provider   query.GraphProvider
	Extractor  FeatureExtractor
	Embedder   embedding.Embedder
	Summarizer Summarizer
}

func (o *Orchestrator) RunExtraction(batchSize int) error {
	log.Printf("Starting resumable extraction (batch size: %d)...", batchSize)

	totalProcessed := 0
	for {
		nodes, err := o.Provider.GetUnextractedFunctions(batchSize)
		if err != nil {
			return fmt.Errorf("failed to fetch unextracted functions: %w", err)
		}

		if len(nodes) == 0 {
			break
		}

		for _, node := range nodes {
			name, _ := node.Properties["name"].(string)
			file, _ := node.Properties["file"].(string)
			startLineRaw, _ := node.Properties["start_line"]
			endLineRaw, _ := node.Properties["end_line"]

			startLine := 0
			endLine := 0
			if v, ok := startLineRaw.(int64); ok {
				startLine = int(v)
			} else if v, ok := startLineRaw.(int); ok {
				startLine = v
			}
			if v, ok := endLineRaw.(int64); ok {
				endLine = int(v)
			} else if v, ok := endLineRaw.(int); ok {
				endLine = v
			}

			if file == "" || startLine == 0 || endLine == 0 {
				_ = o.Provider.UpdateAtomicFeatures(node.ID, []string{"unknown"})
				continue
			}

			code, err := snippet.SliceFile(file, startLine, endLine)
			if err != nil {
				log.Printf("Warning: failed to slice file %s:%d-%d: %v", file, startLine, endLine, err)
				_ = o.Provider.UpdateAtomicFeatures(node.ID, []string{"unreadable_source"})
				continue
			}

			descriptors, err := o.Extractor.Extract(code, name)
			if err != nil {
				log.Printf("Warning: failed to extract features for %s: %v", node.ID, err)
				_ = o.Provider.UpdateAtomicFeatures(node.ID, []string{})
				continue
			}

			if len(descriptors) == 0 {
				descriptors = []string{"no_features_detected"}
			}

			err = o.Provider.UpdateAtomicFeatures(node.ID, descriptors)
			if err != nil {
				return fmt.Errorf("failed to update atomic features for %s: %w", node.ID, err)
			}
			totalProcessed++
		}
	}
	log.Printf("Finished extraction for %d functions", totalProcessed)
	return nil
}

func (o *Orchestrator) RunEmbedding(batchSize int) error {
	log.Printf("Starting resumable embedding (batch size: %d)...", batchSize)

	totalProcessed := 0
	for {
		nodes, err := o.Provider.GetUnembeddedNodes(batchSize)
		if err != nil {
			return fmt.Errorf("failed to fetch unembedded nodes: %w", err)
		}

		if len(nodes) == 0 {
			break
		}

		var batchTexts []string
		var batchIDs []string

		for _, node := range nodes {
			text := NodeToText(*node)
			batchTexts = append(batchTexts, text)
			batchIDs = append(batchIDs, node.ID)
		}

		embeddings, err := o.Embedder.EmbedBatch(batchTexts)
		if err != nil {
			log.Printf("Warning: failed to embed batch: %v", err)
			return fmt.Errorf("embedding batch failed: %w", err)
		}

		for i, emb := range embeddings {
			err = o.Provider.UpdateEmbeddings(batchIDs[i], emb)
			if err != nil {
				return fmt.Errorf("failed to update embedding for %s: %w", batchIDs[i], err)
			}
			totalProcessed++
		}
	}

	log.Printf("Finished embedding for %d nodes", totalProcessed)
	return nil
}

func (o *Orchestrator) RunClustering(dir string) error {
	log.Println("Fetching embeddings for clustering...")
	embeddings, err := o.Provider.GetEmbeddingsOnly()
	if err != nil {
		return fmt.Errorf("failed to get embeddings: %w", err)
	}
	log.Printf("Loaded %d embeddings for clustering", len(embeddings))

	clusterer := &EmbeddingClusterer{
		Embedder:              o.Embedder,
		PrecomputedEmbeddings: embeddings,
	}

	innerGlobalClusterer := &EmbeddingClusterer{
		Embedder:              o.Embedder,
		PrecomputedEmbeddings: embeddings,
		KStrategy: func(n int) int {
			if n == 0 {
				return 0
			}
			k := int(math.Sqrt(float64(n) / 10.0))
			if k < 2 {
				return 2
			}
			return k
		},
	}

	globalClusterer := &GlobalEmbeddingClusterer{
		Inner:                 innerGlobalClusterer,
		Summarizer:            o.Summarizer,
		Loader:                snippet.SliceFile,
		PrecomputedEmbeddings: embeddings,
	}

	builder := &Builder{
		Clusterer:       clusterer,
		GlobalClusterer: globalClusterer,
	}

	log.Println("Fetching function metadata for clustering...")
	metadataNodes, err := o.Provider.GetFunctionMetadata()
	if err != nil {
		return fmt.Errorf("failed to get function metadata: %w", err)
	}

	// Filter down to only nodes that have embeddings available
	var functions []graph.Node
	for _, n := range metadataNodes {
		if _, ok := embeddings[n.ID]; ok {
			functions = append(functions, *n)
		}
	}
	
	log.Printf("Starting clustering with %d functions...", len(functions))

	features, edges, err := builder.Build(dir, functions)
	if err != nil {
		return fmt.Errorf("clustering failed: %w", err)
	}

	// Flatten features
	nodes, allEdges := Flatten(features, edges)
	var nodePointers []*graph.Node
	for i := range nodes {
		nodePointers = append(nodePointers, &nodes[i])
	}
	var edgePointers []*graph.Edge
	for i := range allEdges {
		edgePointers = append(edgePointers, &allEdges[i])
	}

	log.Printf("Writing %d feature nodes and %d edges to database...", len(nodePointers), len(edgePointers))
	if err := o.Provider.UpdateFeatureTopology(nodePointers, edgePointers); err != nil {
		return fmt.Errorf("failed to write topology: %w", err)
	}

	log.Println("Finished clustering")
	return nil
}

func (o *Orchestrator) RunSummarization(batchSize int) error {
	log.Printf("Starting resumable summarization (batch size: %d)...", batchSize)

	totalProcessed := 0
	for {
		nodes, err := o.Provider.GetUnnamedFeatures(batchSize)
		if err != nil {
			return fmt.Errorf("failed to fetch unnamed features: %w", err)
		}

		if len(nodes) == 0 {
			break
		}

		enricher := &Enricher{
			Client:   o.Summarizer,
			Embedder: o.Embedder,
			Loader:   snippet.SliceFile,
		}

		for _, node := range nodes {
			domain, err := o.Provider.ExploreDomain(node.ID)
			if err != nil {
				log.Printf("Warning: failed to explore domain for %s: %v", node.ID, err)
				_ = o.Provider.UpdateFeatureSummary(node.ID, "Unknown Feature", "Failed to analyze")
				continue
			}
			
			var memberFuncs []graph.Node
			for _, fn := range domain.Functions {
				memberFuncs = append(memberFuncs, *fn)
			}
			
			f := &Feature{
				ID:   node.ID,
				Name: "",
			}
			
			err = enricher.Enrich(f, memberFuncs)
			if err != nil {
				log.Printf("Warning: failed to enrich %s: %v", node.ID, err)
				_ = o.Provider.UpdateFeatureSummary(node.ID, "Unnamed Feature", "Enrichment failed")
				continue
			}
			
			err = o.Provider.UpdateFeatureSummary(node.ID, f.Name, f.Description)
			if err != nil {
				return fmt.Errorf("failed to update feature summary for %s: %w", node.ID, err)
			}
			totalProcessed++
		}
	}

	log.Printf("Finished summarization for %d features", totalProcessed)
	return nil
}

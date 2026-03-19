package rpg

import (
	"fmt"
	"graphdb/internal/embedding"
	"graphdb/internal/graph"
	"graphdb/internal/query"
	"graphdb/internal/tools/snippet"
	"graphdb/internal/ui"
	"log"
	"math"
)

type Orchestrator struct {
	Provider   query.GraphProvider
	Extractor  FeatureExtractor
	Embedder   embedding.Embedder
	Summarizer Summarizer
	Seed       int64
	Loader     func(string, int, int) (string, error)
}

func (o *Orchestrator) RunExtraction(batchSize int) error {
	log.Printf("Starting resumable extraction (batch size: %d)...", batchSize)

	total, err := o.Provider.CountUnextractedFunctions()
	if err != nil {
		return fmt.Errorf("failed to count unextracted functions: %w", err)
	}
	if total == 0 {
		log.Println("No unextracted functions to process")
		return nil
	}

	pb := ui.NewProgressBar(total, "Extracting features")
	defer pb.Finish()

	consecutiveErrors := 0
	const maxConsecutiveErrors = 5

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
				_ = o.Provider.UpdateAtomicFeatures(node.ID, []string{"unknown"}, false)
				pb.Add(1)
				continue
			}

			loader := o.Loader
			if loader == nil {
				loader = snippet.SliceFile
			}

			code, err := loader(file, startLine, endLine)
			if err != nil {
				log.Printf("Warning: failed to slice file %s:%d-%d: %v", file, startLine, endLine, err)
				_ = o.Provider.UpdateAtomicFeatures(node.ID, []string{"unreadable_source"}, false)
				pb.Add(1)
				continue
			}

			descriptors, isVolatile, err := o.Extractor.Extract(code, name)
			if err != nil {
				log.Printf("Warning: failed to extract features for %s: %v", node.ID, err)
				_ = o.Provider.UpdateAtomicFeatures(node.ID, []string{"extraction_failed"}, false)
				consecutiveErrors++
				if consecutiveErrors >= maxConsecutiveErrors {
					return fmt.Errorf("extraction aborted: too many consecutive errors (last error: %w)", err)
				}
				continue
			}
			consecutiveErrors = 0

			if len(descriptors) == 0 {
				descriptors = []string{"no_features_detected"}
			}

			err = o.Provider.UpdateAtomicFeatures(node.ID, descriptors, isVolatile)
			if err != nil {
				return fmt.Errorf("failed to update atomic features for %s: %w", node.ID, err)
			}
			pb.Add(1)
		}
	}

	log.Println("Finished extraction")
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

		log.Printf("Embedding progress: processed %d nodes...", totalProcessed)
	}

	log.Printf("Finished embedding for %d nodes", totalProcessed)
	return nil
}
// CalculateDomainK determines the number of top-level domains based on unique file count.
func CalculateDomainK(fileCount int) int {
	if fileCount == 0 {
		return 0
	}
	k := int(math.Sqrt(float64(fileCount) / 5.0))
	if k < 5 {
		return 5
	}
	if k > 50 {
		return 50
	}
	return k
}

func (o *Orchestrator) RunClustering(dir string) error {
	log.Println("Clearing existing feature topology...")
	if err := o.Provider.ClearFeatureTopology(); err != nil {
		return fmt.Errorf("failed to clear topology: %w", err)
	}

	log.Println("Fetching embeddings for clustering...")
	embeddings, err := o.Provider.GetEmbeddingsOnly()
	if err != nil {
		return fmt.Errorf("failed to get embeddings: %w", err)
	}
	log.Printf("Loaded %d embeddings for clustering", len(embeddings))

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

	// Count unique files from function metadata
	uniqueFiles := make(map[string]struct{})
	for _, fn := range functions {
		if file, ok := fn.Properties["file"].(string); ok {
			uniqueFiles[file] = struct{}{}
		}
	}
	fileCount := len(uniqueFiles)
	log.Printf("Detected %d unique files across %d functions", fileCount, len(functions))

	clusterer := &EmbeddingClusterer{
		Embedder:              o.Embedder,
		PrecomputedEmbeddings: embeddings,
		Seed:                  o.Seed,
	}

	innerGlobalClusterer := &EmbeddingClusterer{
		Embedder:              o.Embedder,
		PrecomputedEmbeddings: embeddings,
		Seed:                  o.Seed,
		LogLabel:              "domain",
		KStrategy: func(n int) int {
			return CalculateDomainK(fileCount)
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
		OnPhaseStart: func(phaseName string, total int) {
			log.Printf("Clustering phase: %s (%d domains)", phaseName, total)
		},
		OnStepStart: func(stepName string) {
			log.Printf("  Clustering domain: %s...", stepName)
		},
		OnStepEnd: func(stepName string) {
			log.Printf("  Finished domain: %s", stepName)
		},
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

	log.Printf("Writing %d feature nodes and %d edges to database (batch size: 500)...", len(nodePointers), len(edgePointers))
	if err := o.Provider.UpdateFeatureTopology(nodePointers, edgePointers); err != nil {
		return fmt.Errorf("failed to write topology: %w", err)
	}

	log.Println("Finished clustering")
	return nil
}

func (o *Orchestrator) RunSummarization(batchSize int) error {
	log.Printf("Starting resumable summarization (batch size: %d)...", batchSize)

	total, err := o.Provider.CountUnnamedFeatures()
	if err != nil {
		return fmt.Errorf("failed to count unnamed features: %w", err)
	}
	if total == 0 {
		log.Println("No unnamed features to summarize")
		return nil
	}

	pb := ui.NewProgressBar(total, "Summarizing features")
	defer pb.Finish()

	consecutiveErrors := 0
	const maxConsecutiveErrors = 5

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
				return fmt.Errorf("failed to explore domain for %s: %w", node.ID, err)
			}

			var memberFuncs []graph.Node
			for _, fn := range domain.Functions {
				memberFuncs = append(memberFuncs, *fn)
			}

			f := &Feature{
				ID:   node.ID,
				Name: "",
			}

			err = enricher.Enrich(f, memberFuncs, node.Label)
			if err != nil {
				log.Printf("Warning: failed to enrich %s: %v", node.ID, err)
				_ = o.Provider.UpdateFeatureSummary(node.ID, "summarization_failed", "Summarization failed due to LLM error")
				consecutiveErrors++
				if consecutiveErrors >= maxConsecutiveErrors {
					return fmt.Errorf("summarization aborted: too many consecutive errors (last error: %w)", err)
				}
				continue
			}
			consecutiveErrors = 0

			err = o.Provider.UpdateFeatureSummary(node.ID, f.Name, f.Description)
			if err != nil {
				return fmt.Errorf("failed to update feature summary for %s: %w", node.ID, err)
			}
			pb.Add(1)
		}
	}

	log.Println("Finished summarization")
	return nil
}

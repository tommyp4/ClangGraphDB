package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"graphdb/internal/config"
	"graphdb/internal/query"
	"log"
	"os"
	"strings"
)

func handleQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	typePtr := fs.String("type", "", "Query type: search-features, search-similar, hybrid-context, neighbors, impact, globals, coverage, seams, hotspots, explore-domain")
	targetPtr := fs.String("target", "", "Target function name or query text")
	target2Ptr := fs.String("target2", "", "Second target (e.g. for locate-usage)")
	depthPtr := fs.Int("depth", 1, "Traversal depth")
	limitPtr := fs.Int("limit", 10, "Result limit")
	modulePtr := fs.String("module", ".*", "Module pattern for seams")
	layerPtr := fs.String("layer", "ui", "Contamination layer for seams (ui, db, io, all)")
	edgeTypesPtr := fs.String("edge-types", "", "Comma-separated relationship types for traverse")
	directionPtr := fs.String("direction", "outgoing", "Traversal direction: incoming, outgoing, both")

	// Embedder args for 'features' type
	locationPtr := fs.String("location", "us-central1", "GCP Location")
	modelPtr := fs.String("model", "", "Embedding model name")

	fs.Parse(args)

	cfg := config.LoadConfig()
	model := *modelPtr
	if model == "" {
		model = cfg.GeminiEmbeddingModel
	}
	if model == "" {
		model = "gemini-embedding-001"
	}

	if cfg.Neo4jURI == "" {
		log.Fatal("NEO4J_URI environment variable is not set")
	}

	provider, err := setupProvider(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Neo4j: %v", err)
	}
	defer provider.Close()
	var result any

	switch *typePtr {
	case "features": // Alias
		fallthrough
	case "search-features":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'search-features'")
		}
		embedder := setupEmbedder(cfg.GoogleCloudProject, *locationPtr, model, cfg.GeminiEmbeddingDimensions)
		embeddings, err := embedder.EmbedBatch([]string{*targetPtr})
		if err != nil {
			log.Fatalf("Embedding failed: %v", err)
		}
		result, err = provider.SearchFeatures(embeddings[0], *limitPtr)
		if err != nil {
			log.Fatalf("SearchFeatures failed: %v", err)
		}

	case "search-similar":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'search-similar'")
		}
		embedder := setupEmbedder(cfg.GoogleCloudProject, *locationPtr, model, cfg.GeminiEmbeddingDimensions)
		embeddings, err := embedder.EmbedBatch([]string{*targetPtr})
		if err != nil {
			log.Fatalf("Embedding failed: %v", err)
		}
		result, err = provider.SearchSimilarFunctions(embeddings[0], *limitPtr)
		if err != nil {
			log.Fatalf("SearchSimilarFunctions failed: %v", err)
		}

	case "hybrid-context":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'hybrid-context'")
		}
		// 1. Structural Neighbors (Dependency Layer)
		neighbors, err := provider.GetNeighbors(*targetPtr, *depthPtr)
		if err != nil {
			log.Fatalf("Neighbors lookup failed: %v", err)
		}

		// 2. Semantic Search (Dependency Layer)
		embedder := setupEmbedder(cfg.GoogleCloudProject, *locationPtr, model, cfg.GeminiEmbeddingDimensions)
		embeddings, err := embedder.EmbedBatch([]string{*targetPtr})
		if err != nil {
			log.Printf("Warning: Embedding failed for hybrid search: %v", err)
		}

		var similar []*query.FeatureResult
		if len(embeddings) > 0 {
			similar, _ = provider.SearchSimilarFunctions(embeddings[0], *limitPtr)
		}

		result = map[string]interface{}{
			"neighbors": neighbors,
			"similar":   similar,
		}

	case "test-context": // Alias
		fallthrough
	case "neighbors":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'neighbors'")
		}
		result, err = provider.GetNeighbors(*targetPtr, *depthPtr)

	case "impact":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'impact'")
		}
		result, err = provider.GetImpact(*targetPtr, *depthPtr)

	case "globals":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'globals'")
		}
		result, err = provider.GetGlobals(*targetPtr)

	case "coverage":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'coverage'")
		}
		result, err = provider.GetCoverage(*targetPtr)

	case "seams":
		result, err = provider.GetSeams(*modulePtr, *layerPtr)

	case "hotspots":
		result, err = provider.GetHotspots(*modulePtr)

	case "locate-usage":
		if *targetPtr == "" || *target2Ptr == "" {
			log.Fatal("-target and -target2 are required for 'locate-usage'")
		}
		result, err = provider.LocateUsage(*targetPtr, *target2Ptr)

	case "fetch-source":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'fetch-source'")
		}
		source, err := provider.FetchSource(*targetPtr)
		if err != nil {
			log.Fatalf("FetchSource failed: %v", err)
		}
		fmt.Print(source) // Print raw source to stdout
		return

	case "explore-domain":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'explore-domain'")
		}
		result, err = provider.ExploreDomain(*targetPtr)

	case "traverse":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'traverse'")
		}
		dir := query.Outgoing
		switch strings.ToLower(*directionPtr) {
		case "incoming":
			dir = query.Incoming
		case "both":
			dir = query.Both
		}
		result, err = provider.Traverse(*targetPtr, *edgeTypesPtr, dir, *depthPtr)

	case "status":
		commit, err := provider.GetGraphState()
		if err != nil {
			log.Fatalf("Status check failed: %v", err)
		}
		result = map[string]string{
			"commit": commit,
		}

	default:
		log.Fatalf("Unknown or missing query type: %s. Valid types: search-features, search-similar, hybrid-context, neighbors, impact, globals, coverage, seams, explore-domain, status", *typePtr)
	}

	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		log.Fatalf("Failed to encode result: %v", err)
	}
}

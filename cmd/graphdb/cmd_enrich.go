package main

import (
	"flag"
	"graphdb/internal/config"
	"graphdb/internal/rpg"
	"log"
)

func handleEnrichFeatures(args []string) {
	fs := flag.NewFlagSet("enrich-features", flag.ExitOnError)
	dirPtr := fs.String("dir", ".", "Directory to analyze")
	batchSizePtr := fs.Int("batch-size", 20, "Batch size for LLM feature extraction")
	embedBatchSizePtr := fs.Int("embed-batch-size", 100, "Batch size for embedding generation")

	fs.Parse(args)

	cfg := config.LoadConfig()

	loc := cfg.GoogleCloudLocation
	if loc == "" {
		loc = "us-central1"
	}

	model := cfg.GeminiEmbeddingModel
	if model == "" {
		model = "gemini-embedding-001"
	}

	genModel := cfg.GeminiGenerativeModel
	if genModel == "" {
		log.Fatal("GEMINI_GENERATIVE_MODEL is not set. Please set it in your .env file or environment.\n" +
			"Example: export GEMINI_GENERATIVE_MODEL=gemini-3-flash-preview")
	}

	if cfg.GoogleCloudProject == "" {
		log.Fatal("GOOGLE_CLOUD_PROJECT is not set. Please set it in your .env file or environment.\n" +
			"Example: export GOOGLE_CLOUD_PROJECT=my-project-id")
	}

	if cfg.Neo4jURI == "" {
		log.Fatal("NEO4J_URI environment variable is not set")
	}

	log.Println("Connecting to Graph Database...")
	provider, err := setupProvider(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Neo4j: %v", err)
	}
	defer provider.Close()

	extractor := setupExtractor(cfg.GoogleCloudProject, loc, genModel)
	embedder := setupEmbedder(cfg.GoogleCloudProject, loc, model, cfg.GeminiEmbeddingDimensions)
	summarizer := setupSummarizer(cfg.GoogleCloudProject, loc, genModel)

	orchestrator := &rpg.Orchestrator{
		Provider:   provider,
		Extractor:  extractor,
		Embedder:   embedder,
		Summarizer: summarizer,
	}

	log.Println("Starting Database-backed Feature Enrichment...")

	// 1. Atomic Feature Extraction
	if err := orchestrator.RunExtraction(*batchSizePtr); err != nil {
		log.Fatalf("Extraction failed: %v", err)
	}

	// 2. Embedding Generation
	if err := orchestrator.RunEmbedding(*embedBatchSizePtr); err != nil {
		log.Fatalf("Embedding generation failed: %v", err)
	}

	// 3. Out-of-Core Clustering (Topology generation)
	if err := orchestrator.RunClustering(*dirPtr); err != nil {
		log.Fatalf("Clustering failed: %v", err)
	}

	// 4. Summarization
	if err := orchestrator.RunSummarization(*batchSizePtr); err != nil {
		log.Fatalf("Summarization failed: %v", err)
	}

	log.Println("Feature enrichment completed successfully.")
}

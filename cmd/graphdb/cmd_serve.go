package main

import (
	"flag"
	"fmt"
	"graphdb/internal/config"
	"graphdb/internal/ui"
	"log"
	"net/http"
)

func handleServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	portPtr := fs.Int("port", 8080, "Port to run the HTTP server on")
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

	embedder := setupEmbedder(cfg.GoogleCloudProject, *locationPtr, model, cfg.GeminiEmbeddingDimensions)

	server := ui.NewServer(provider, embedder)

	addr := fmt.Sprintf(":%d", *portPtr)
	log.Printf("Starting web visualizer on http://localhost%s\n", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

package main

import (
	"flag"
	"graphdb/internal/config"
	"log"
)

func handleEnrichTests(args []string) {
	fs := flag.NewFlagSet("enrich-tests", flag.ExitOnError)
	fs.Parse(args)

	cfg := config.LoadConfig()

	if cfg.Neo4jURI == "" {
		log.Fatal("NEO4J_URI environment variable is not set")
	}

	provider, err := setupProvider(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Neo4j: %v", err)
	}
	defer provider.Close()

	log.Println("Linking test functions to production functions...")
	if err := provider.LinkTests(); err != nil {
		log.Fatalf("LinkTests failed: %v", err)
	}
	log.Println("Test enrichment completed successfully.")
}

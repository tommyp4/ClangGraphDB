package main

import (
	"flag"
	"graphdb/internal/config"
	"graphdb/internal/query"
	"log"
)

func handleEnrichContamination(args []string) {
	fs := flag.NewFlagSet("enrich-contamination", flag.ExitOnError)
	modulePtr := fs.String("module", ".*", "Regex pattern to filter file paths (e.g., '.*Controllers.*')")

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

	// Default rules for volatility seeding.
	// These seed initial is_volatile flags based on Feathers' legacy seams definition:
	// - External dependencies (IO, Network, DB)
	// - 3rd-party libraries
	// - Non-deterministic functions (Time, Random)
	rules := []query.ContaminationRule{
		// External & Network
		{Layer: "volatility", Type: "function", Pattern: `(?i).*HttpClient.*`, Heuristic: "content"},
		{Layer: "volatility", Type: "function", Pattern: `(?i).*WebRequest.*`, Heuristic: "content"},
		{Layer: "volatility", Type: "function", Pattern: `(?i).*Socket.*`, Heuristic: "content"},
		{Layer: "volatility", Type: "function", Pattern: `(?i).*System\.Net.*`, Heuristic: "content"},
		
		// Database & Storage
		{Layer: "volatility", Type: "function", Pattern: `(?i).*SELECT.*FROM.*`, Heuristic: "content"},
		{Layer: "volatility", Type: "function", Pattern: `(?i).*INSERT.*INTO.*`, Heuristic: "content"},
		{Layer: "volatility", Type: "function", Pattern: `(?i).*UPDATE.*SET.*`, Heuristic: "content"},
		{Layer: "volatility", Type: "function", Pattern: `(?i).*DELETE.*FROM.*`, Heuristic: "content"},
		{Layer: "volatility", Type: "function", Pattern: `(?i).*DbContext.*`, Heuristic: "content"},
		{Layer: "volatility", Type: "function", Pattern: `(?i).*Repository.*`, Heuristic: "content"},
		
		// Non-determinism & Environment
		{Layer: "volatility", Type: "function", Pattern: `(?i).*DateTime\.Now.*`, Heuristic: "content"},
		{Layer: "volatility", Type: "function", Pattern: `(?i).*DateTime\.UtcNow.*`, Heuristic: "content"},
		{Layer: "volatility", Type: "function", Pattern: `(?i).*Guid\.NewGuid.*`, Heuristic: "content"},
		{Layer: "volatility", Type: "function", Pattern: `(?i).*Random.*`, Heuristic: "content"},
		
		// UI & Framework Boundaries
		{Layer: "volatility", Type: "file", Pattern: `(?i).*Controller.*`, Heuristic: "path"},
		{Layer: "volatility", Type: "file", Pattern: `(?i).*View.*`, Heuristic: "path"},
		{Layer: "volatility", Type: "file", Pattern: `(?i).*\.aspx$`, Heuristic: "path"},
		{Layer: "volatility", Type: "file", Pattern: `(?i).*\.cshtml$`, Heuristic: "path"},
	}

	log.Println("Seeding initial volatility flags using heuristic rules...")
	if err := provider.SeedVolatility(*modulePtr, rules); err != nil {
		log.Fatalf("Volatility seeding failed: %v", err)
	}

	log.Printf("Propagating volatility UPWARD through the CALLS graph...")
	if err := provider.PropagateVolatility(); err != nil {
		log.Fatalf("Volatility propagation failed: %v", err)
	}

	log.Println("Calculating risk scores for functions based on volatility...")
	if err := provider.CalculateRiskScores(); err != nil {
		log.Fatalf("Risk score calculation failed: %v", err)
	}

	log.Println("Contamination enrichment completed successfully.")
}

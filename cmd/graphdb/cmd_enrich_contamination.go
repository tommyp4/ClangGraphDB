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

	// Default rules for legacy modernization analysis.
	// These seed initial contamination flags based on common patterns.
	rules := []query.ContaminationRule{
		// UI Layer: Controller and View logic
		{Layer: "ui", Type: "file", Pattern: `(?i).*Controller.*`, Heuristic: "path"},
		{Layer: "ui", Type: "file", Pattern: `(?i).*View.*`, Heuristic: "path"},
		{Layer: "ui", Type: "file", Pattern: `(?i).*Form.*`, Heuristic: "path"},
		{Layer: "ui", Type: "file", Pattern: `(?i).*\.aspx$`, Heuristic: "path"},
		{Layer: "ui", Type: "file", Pattern: `(?i).*\.cshtml$`, Heuristic: "path"},
		
		// DB Layer: Data access patterns and keywords
		{Layer: "db", Type: "function", Pattern: `(?i).*SELECT.*FROM.*`, Heuristic: "content"},
		{Layer: "db", Type: "function", Pattern: `(?i).*INSERT.*INTO.*`, Heuristic: "content"},
		{Layer: "db", Type: "function", Pattern: `(?i).*UPDATE.*SET.*`, Heuristic: "content"},
		{Layer: "db", Type: "function", Pattern: `(?i).*DELETE.*FROM.*`, Heuristic: "content"},
		{Layer: "db", Type: "function", Pattern: `(?i).*DbContext.*`, Heuristic: "content"},
		{Layer: "db", Type: "function", Pattern: `(?i).*Repository.*`, Heuristic: "content"},
		{Layer: "db", Type: "function", Pattern: `(?i).*SqlParameter.*`, Heuristic: "content"},
		
		// IO Layer: External network and service calls
		{Layer: "io", Type: "function", Pattern: `(?i).*HttpClient.*`, Heuristic: "content"},
		{Layer: "io", Type: "function", Pattern: `(?i).*WebRequest.*`, Heuristic: "content"},
		{Layer: "io", Type: "function", Pattern: `(?i).*Socket.*`, Heuristic: "content"},
		{Layer: "io", Type: "function", Pattern: `(?i).*File\.Read.*`, Heuristic: "content"},
		{Layer: "io", Type: "function", Pattern: `(?i).*File\.Write.*`, Heuristic: "content"},
	}

	log.Println("Seeding initial contamination flags using heuristic rules...")
	if err := provider.SeedContamination(*modulePtr, rules); err != nil {
		log.Fatalf("Seeding failed: %v", err)
	}

	layers := []string{"ui", "db", "io"}
	for _, layer := range layers {
		log.Printf("Propagating %s contamination through the CALLS graph...", layer)
		if err := provider.PropagateContamination(layer); err != nil {
			log.Fatalf("Propagation for %s failed: %v", layer, err)
		}
	}

	log.Println("Calculating risk scores for functions...")
	if err := provider.CalculateRiskScores(); err != nil {
		log.Fatalf("Risk score calculation failed: %v", err)
	}

	log.Println("Contamination enrichment completed successfully.")
}

package main

import (
	"fmt"
	"graphdb/internal/config"
	"os"
)

// Version is injected at build time
var Version = "dev"

var (
	ingestCmd              = handleIngest
	enrichCmd              = handleEnrichFeatures
	importCmd              = handleImport
	enrichHistoryCmd       = handleEnrichHistory
	enrichContaminationCmd = handleEnrichContamination
	enrichTestsCmd         = handleEnrichTests
)

func main() {
	// Attempt to load .env file from current or parent directories
	_ = config.LoadEnv()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "ingest":
		ingestCmd(os.Args[2:])
	case "query":
		handleQuery(os.Args[2:])
	case "enrich-features":
		enrichCmd(os.Args[2:])
	case "enrich-contamination":
		handleEnrichContamination(os.Args[2:])
	case "enrich-history":
		handleEnrichHistory(os.Args[2:])
	case "enrich-tests":
		handleEnrichTests(os.Args[2:])
	case "import":
		importCmd(os.Args[2:])
	case "serve":
		handleServe(os.Args[2:])
	case "build-all":
		handleBuildAll(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("graphdb version %s\n", Version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf("GraphDB Skill CLI (Version: %s)\n", Version)
	fmt.Println("Usage: graphdb <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  ingest                 Parse code and generate graph nodes/edges (JSONL)")
	fmt.Println("  enrich-features        Build the RPG (Repository Planning Graph) Intent Layer")
	fmt.Println("  enrich-contamination   Identify seams and propagate contamination layers")
	fmt.Println("  enrich-history         Analyze git history to find hotspots and co-changes")
	fmt.Println("  enrich-tests           Link tests to production functions")
	fmt.Println("  import                 Import JSONL files into Neo4j")
	fmt.Println("  query                  Query the graph (structural or semantic)")
	fmt.Println("  serve                  Start the HTTP server and D3 visualizer")
	fmt.Println("  build-all              One-shot: Ingest -> Import -> All Enrichment Phases")
	fmt.Println("  version                Show version info")
	fmt.Println("\nRun 'graphdb <command> --help' for command-specific options.")
}

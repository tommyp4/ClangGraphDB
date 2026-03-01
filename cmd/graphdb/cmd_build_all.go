package main

import (
	"flag"
	"fmt"
)

func handleBuildAll(args []string) {
	fmt.Println("🚀 Starting GraphDB Build-All Sequence...")
	fmt.Println("========================================")

	fs := flag.NewFlagSet("build-all", flag.ExitOnError)
	dirPtr := fs.String("dir", ".", "Directory to process")
	cleanPtr := fs.Bool("clean", true, "Clean DB before import")
	nodesPtr := fs.String("nodes", "nodes.jsonl", "Intermediate output file for nodes")
	edgesPtr := fs.String("edges", "edges.jsonl", "Intermediate output file for edges")
	fs.Parse(args)

	// 1. Ingest
	fmt.Println("\n[Phase 1/3] Ingesting Codebase...")
	ingestArgs := []string{"-dir", *dirPtr, "-nodes", *nodesPtr, "-edges", *edgesPtr}
	ingestCmd(ingestArgs)

	// 2. Import Structural Graph
	fmt.Println("\n[Phase 2/3] Importing to Neo4j...")
	importArgs1 := []string{"-nodes", *nodesPtr, "-edges", *edgesPtr}
	if *cleanPtr {
		importArgs1 = append(importArgs1, "-clean")
	}
	importCmd(importArgs1)

	// 3. Enrich
	fmt.Println("\n[Phase 3/3] Enriching Features (in-database)...")
	enrichArgs := []string{"-dir", *dirPtr}
	enrichCmd(enrichArgs)

	fmt.Println("\n✅ Build-All Sequence Complete!")
}

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

	// 3. Enrich Features
	fmt.Println("\n[Phase 3/6] Enriching Features (in-database)...")
	enrichArgs := []string{"-dir", *dirPtr}
	enrichCmd(enrichArgs)

	// 4. Enrich History
	fmt.Println("\n[Phase 4/6] Enriching Git History...")
	historyArgs := []string{"-dir", *dirPtr}
	enrichHistoryCmd(historyArgs)

	// 5. Enrich Contamination
	fmt.Println("\n[Phase 5/6] Enriching Contamination/Risk...")
	contaminationArgs := []string{"-module", ".*"}
	enrichContaminationCmd(contaminationArgs)

	// 6. Enrich Tests
	fmt.Println("\n[Phase 6/6] Linking Tests...")
	testArgs := []string{}
	enrichTestsCmd(testArgs)

	fmt.Println("\n✅ Build-All Sequence Complete!")
}

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func handleClangBuildAll(args []string) {
	fmt.Println("Starting Clang GraphDB Build-All Sequence...")
	fmt.Println("=============================================")

	fs := flag.NewFlagSet("clang-build-all", flag.ExitOnError)
	slnPtr := fs.String("sln", "", "Path to .sln file (required)")
	configPtr := fs.String("config", "Debug|Win32", "Build configuration")
	outputPtr := fs.String("output", ".", "Output directory for intermediate files")
	extractorPtr := fs.String("extractor", "", "Path to clang-extractor.exe")
	verbosePtr := fs.Bool("verbose", false, "Verbose output")
	fs.Parse(args)

	if *slnPtr == "" {
		fmt.Println("ERROR: -sln is required")
		os.Exit(1)
	}

	nodesPath := filepath.Join(*outputPtr, "nodes.jsonl")
	edgesPath := filepath.Join(*outputPtr, "edges.jsonl")

	// 1. Clang Ingest
	fmt.Println("\n[Phase 1/6] Clang Ingest (parse .sln + extract C++ graph)...")
	ingestArgs := []string{"-sln", *slnPtr, "-config", *configPtr, "-output", *outputPtr}
	if *extractorPtr != "" {
		ingestArgs = append(ingestArgs, "-extractor", *extractorPtr)
	}
	if *verbosePtr {
		ingestArgs = append(ingestArgs, "-verbose")
	}
	handleClangIngest(ingestArgs)

	// 2. Import to Neo4j
	fmt.Println("\n[Phase 2/6] Importing to Neo4j...")
	importArgs := []string{"-nodes", nodesPath, "-edges", edgesPath}
	importCmd(importArgs)

	// 2.5 Cleanup
	fmt.Println("\nCleaning up intermediate JSONL files...")
	if err := os.Remove(nodesPath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to remove %s: %v\n", nodesPath, err)
	}
	if err := os.Remove(edgesPath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to remove %s: %v\n", edgesPath, err)
	}

	// Determine repo root for enrichment
	slnAbs, _ := filepath.Abs(*slnPtr)
	repoDir := filepath.Dir(filepath.Dir(slnAbs))

	// 3. Enrich Features
	fmt.Println("\n[Phase 3/6] Enriching Features...")
	enrichCmd([]string{"-dir", repoDir})

	// 4. Enrich History
	fmt.Println("\n[Phase 4/6] Enriching Git History...")
	enrichHistoryCmd([]string{"-dir", repoDir})

	// 5. Enrich Contamination
	fmt.Println("\n[Phase 5/6] Enriching Contamination/Risk...")
	enrichContaminationCmd([]string{})

	// 6. Enrich Tests
	fmt.Println("\n[Phase 6/6] Linking Tests...")
	enrichTestsCmd([]string{})

	fmt.Println("\nClang Build-All Sequence Complete!")
}

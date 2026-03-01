package main

import (
	"bufio"
	"context"
	"flag"
	"graphdb/internal/config"
	"graphdb/internal/ingest"
	"graphdb/internal/storage"
	"graphdb/internal/ui"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func handleIngest(args []string) {
	flags := flag.NewFlagSet("ingest", flag.ExitOnError)
	dirPtr := flags.String("dir", ".", "Directory to walk (ignored if -file-list is used)")
	fileListPtr := flags.String("file-list", "", "Path to a file containing a list of files to process")
	workersPtr := flags.Int("workers", 4, "Number of workers")
	outputPtr := flags.String("output", "graph.jsonl", "Output file path (combined)")
	nodesPtr := flags.String("nodes", "", "Output file path for nodes")
	edgesPtr := flags.String("edges", "", "Output file path for edges")

	flags.Parse(args)

	cfg := config.LoadConfig()

	loc := cfg.GoogleCloudLocation
	if loc == "" {
		loc = "us-central1"
	}

	model := cfg.GeminiEmbeddingModel
	if model == "" {
		model = "gemini-embedding-001"
	}

	var emitter storage.Emitter
	if *nodesPtr != "" || *edgesPtr != "" {
		if *nodesPtr == "" || *edgesPtr == "" {
			log.Fatalf("Both -nodes and -edges must be provided for split output")
		}
		nodeFile, err := os.Create(*nodesPtr)
		if err != nil {
			log.Fatalf("Failed to create nodes file: %v", err)
		}
		edgeFile, err := os.Create(*edgesPtr)
		if err != nil {
			log.Fatalf("Failed to create edges file: %v", err)
		}
		emitter = storage.NewSplitJSONLEmitter(nodeFile, edgeFile)
	} else {
		// Setup Combined Emitter
		outFile, err := os.Create(*outputPtr)
		if err != nil {
			log.Fatalf("Failed to create output file: %v", err)
		}
		emitter = storage.NewJSONLEmitter(outFile)
	}
	defer emitter.Close()

	// Setup Embedder
	embedder := setupEmbedder(cfg.GoogleCloudProject, loc, model, cfg.GeminiEmbeddingDimensions)

	// Setup Walker
	walker := ingest.NewWalker(*workersPtr, embedder, emitter)

	// Count files first for progress bar
	var totalFiles int64
	if *fileListPtr != "" {
		log.Printf("Counting files in list %s...", *fileListPtr)
		file, err := os.Open(*fileListPtr)
		if err != nil {
			log.Fatalf("Failed to open file list: %v", err)
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if scanner.Text() != "" {
				totalFiles++
			}
		}
		file.Close()
	} else {
		log.Printf("Counting files in %s...", *dirPtr)
		var err error
		totalFiles, err = walker.Count(context.Background(), *dirPtr)
		if err != nil {
			log.Fatalf("Failed to count files: %v", err)
		}
	}
	log.Printf("Found %d files to process", totalFiles)

	pb := ui.NewProgressBar(totalFiles, "Processing files")
	walker.WorkerPool.OnProgress = func() {
		pb.Add(1)
	}
	defer pb.Finish()

	// Context with Cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Received shutdown signal...")
		cancel()
	}()

	// Run
	start := time.Now()

	if *fileListPtr != "" {
		log.Printf("Starting ingestion from file list %s with %d workers...", *fileListPtr, *workersPtr)
		file, err := os.Open(*fileListPtr)
		if err != nil {
			log.Fatalf("Failed to open file list: %v", err)
		}
		defer file.Close()

		walker.WorkerPool.Start()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			path := scanner.Text()
			if path != "" {
				walker.WorkerPool.Submit(*dirPtr, path)
			}
		}
		walker.WorkerPool.Stop()
	} else {
		log.Printf("Starting walk on %s with %d workers...", *dirPtr, *workersPtr)
		if err := walker.Run(ctx, *dirPtr); err != nil {
			log.Fatalf("Walker failed: %v", err)
		}
	}

	log.Printf("Done in %v.", time.Since(start))
}

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"graphdb/internal/config"
	"graphdb/internal/graph"
	"graphdb/internal/ingest"
	"graphdb/internal/loader"
	"graphdb/internal/query"
	"graphdb/internal/rpg"
	"graphdb/internal/storage"
	"graphdb/internal/ui"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Version is injected at build time
var Version = "dev"

var (
	ingestCmd = handleIngest
	enrichCmd = handleEnrichFeatures
	importCmd = handleImport
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
	case "import":
		importCmd(os.Args[2:])
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
	fmt.Println("  ingest           Parse code and generate graph nodes/edges (JSONL)")
	fmt.Println("  enrich-features  Build the RPG (Repository Planning Graph) Intent Layer")
	fmt.Println("  import           Import JSONL files into Neo4j")
	fmt.Println("  query            Query the graph (structural or semantic)")
	fmt.Println("  build-all        One-shot: Ingest -> Enrich -> Import")
	fmt.Println("  version          Show version info")
	fmt.Println("\nRun 'graphdb <command> --help' for command-specific options.")
}

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
func handleImport(args []string) {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	nodesPtr := fs.String("nodes", "", "Path to nodes JSONL file")
	edgesPtr := fs.String("edges", "", "Path to edges JSONL file")
	inputPtr := fs.String("input", "", "Path to combined JSONL file (nodes + edges)")
	batchSizePtr := fs.Int("batch-size", 500, "Batch size for insertion")
	cleanPtr := fs.Bool("clean", false, "Wipe database before importing")

	    fs.Parse(args)
	
	    if *inputPtr == "" && (*nodesPtr == "" || *edgesPtr == "") {
	            log.Fatal("Either -input or both -nodes and -edges must be provided")
	    }
	
	        cfg := config.LoadConfig()
	        if cfg.Neo4jURI == "" {
		log.Fatal("NEO4J_URI environment variable is not set")
	}

	driver, err := neo4j.NewDriverWithContext(cfg.Neo4jURI, neo4j.BasicAuth(cfg.Neo4jUser, cfg.Neo4jPassword, ""))
	if err != nil {
		log.Fatalf("Failed to create Neo4j driver: %v", err)
	}
	defer driver.Close(context.Background())

	loader := loader.NewNeo4jLoader(driver, "neo4j", cfg.GeminiEmbeddingDimensions) // Default DB name

	ctx := context.Background()

	// 1. Clean Database (Phase 3)
	if *cleanPtr {
		log.Println("Wiping database...")
		if err := loader.Wipe(ctx); err != nil {
			log.Fatalf("Failed to wipe database: %v", err)
		}
	}

	// 2. Apply Constraints
	log.Println("Applying schema constraints...")
	if err := loader.ApplyConstraints(ctx); err != nil {
		log.Printf("Warning: failed to apply constraints: %v", err)
	}

	// 3. Load Nodes
	var nodeFiles []string
	if *inputPtr != "" {
		nodeFiles = append(nodeFiles, *inputPtr)
	}
	if *nodesPtr != "" {
		nodeFiles = append(nodeFiles, *nodesPtr)
	}

	for _, path := range nodeFiles {
		log.Printf("Importing nodes from %s...", path)
		if err := processBatches(path, *batchSizePtr, func(batch []json.RawMessage) error {
			var nodes []graph.Node
			for _, raw := range batch {
				var flat map[string]interface{}
				if err := json.Unmarshal(raw, &flat); err != nil {
					continue
				}

				// Heuristic: Edges have "source"
				if _, ok := flat["source"]; ok {
					continue // It's an edge
				}

				id, _ := flat["id"].(string)
				label, _ := flat["type"].(string)

				if id == "" {
					continue
				}

				// Remove ID and Type from properties
				delete(flat, "id")
				delete(flat, "type")

				n := graph.Node{
					ID:         id,
					Label:      label,
					Properties: flat,
				}
				nodes = append(nodes, n)
			}
			return loader.BatchLoadNodes(ctx, nodes)
		}); err != nil {
			log.Fatalf("Failed to import nodes: %v", err)
		}
	}

	// 3. Load Edges
	var edgeFiles []string
	if *inputPtr != "" {
		edgeFiles = append(edgeFiles, *inputPtr)
	}
	if *edgesPtr != "" {
		edgeFiles = append(edgeFiles, *edgesPtr)
	}

	for _, path := range edgeFiles {
		log.Printf("Importing edges from %s...", path)
		if err := processBatches(path, *batchSizePtr, func(batch []json.RawMessage) error {
			var edges []graph.Edge
			for _, raw := range batch {
				var flat map[string]interface{}
				if err := json.Unmarshal(raw, &flat); err != nil {
					continue
				}

				// Heuristic: Edges have "source"
				if _, ok := flat["source"]; !ok {
					continue // It's a node
				}

				src, _ := flat["source"].(string)
				tgt, _ := flat["target"].(string)
				typ, _ := flat["type"].(string)

				if src == "" || tgt == "" {
					continue
				}

				e := graph.Edge{
					SourceID: src,
					TargetID: tgt,
					Type:     typ,
				}
				edges = append(edges, e)
			}
			return loader.BatchLoadEdges(ctx, edges)
		}); err != nil {
			log.Fatalf("Failed to import edges: %v", err)
		}
	}

	log.Println("Import complete.")

	// 5. Update Graph State (Commit Hash)
	// Try to get current git commit
	if commit, err := getGitCommit(); err == nil && commit != "" {
		log.Printf("Updating graph state with commit %s...", commit)
		if err := loader.UpdateGraphState(ctx, commit); err != nil {
			log.Printf("Warning: failed to update graph state: %v", err)
		}
	}
}

func getGitCommit() (string, error) {
	// Simple git rev-parse HEAD
	// In a real CLI, we might use the git library or exec
	// Since we are inside the repo, exec is fine
	cmd := "git"
	args := []string{"rev-parse", "HEAD"}

	out, err := execCommand(cmd, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Wrapper for testing/mocking if needed
var execCommand = func(name string, arg ...string) ([]byte, error) {
	c := exec.Command(name, arg...)
	return c.Output()
}

func processBatches(path string, batchSize int, process func([]json.RawMessage) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var scanner *bufio.Scanner
	if fi, err := f.Stat(); err == nil {
		pb := ui.NewProgressBar(fi.Size(), fmt.Sprintf("Importing %s", filepath.Base(path)))
		pb.SetFormat(ui.FormatBytesFn)
		defer pb.Finish()
		reader := &ui.ByteReader{Reader: f, Pb: pb}
		scanner = bufio.NewScanner(reader)
	} else {
		scanner = bufio.NewScanner(f)
	}

	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var batch []json.RawMessage
	for scanner.Scan() {
		line := scanner.Bytes()
		// Copy slice because scanner reuses it
		item := make([]byte, len(line))
		copy(item, line)

		batch = append(batch, item)

		if len(batch) >= batchSize {
			if err := process(batch); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := process(batch); err != nil {
			return err
		}
	}

	return scanner.Err()
}




func handleQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	typePtr := fs.String("type", "", "Query type: search-features, search-similar, hybrid-context, neighbors, impact, globals, seams, explore-domain")
	targetPtr := fs.String("target", "", "Target function name or query text")
	target2Ptr := fs.String("target2", "", "Second target (e.g. for locate-usage)")
	depthPtr := fs.Int("depth", 1, "Traversal depth")
	limitPtr := fs.Int("limit", 10, "Result limit")
	modulePtr := fs.String("module", ".*", "Module pattern for seams")
	edgeTypesPtr := fs.String("edge-types", "", "Comma-separated relationship types for traverse")
	directionPtr := fs.String("direction", "outgoing", "Traversal direction: incoming, outgoing, both")

	// Embedder args for 'features' type
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
	    defer provider.Close()
	var result any

	switch *typePtr {
	case "features": // Alias
		fallthrough
	case "search-features":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'search-features'")
		}
		embedder := setupEmbedder(cfg.GoogleCloudProject, *locationPtr, model, cfg.GeminiEmbeddingDimensions)
		embeddings, err := embedder.EmbedBatch([]string{*targetPtr})
		if err != nil {
			log.Fatalf("Embedding failed: %v", err)
		}
		result, err = provider.SearchFeatures(embeddings[0], *limitPtr)
		if err != nil {
			log.Fatalf("SearchFeatures failed: %v", err)
		}

	case "search-similar":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'search-similar'")
		}
		embedder := setupEmbedder(cfg.GoogleCloudProject, *locationPtr, model, cfg.GeminiEmbeddingDimensions)
		embeddings, err := embedder.EmbedBatch([]string{*targetPtr})
		if err != nil {
			log.Fatalf("Embedding failed: %v", err)
		}
		result, err = provider.SearchSimilarFunctions(embeddings[0], *limitPtr)
		if err != nil {
			log.Fatalf("SearchSimilarFunctions failed: %v", err)
		}

	case "hybrid-context":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'hybrid-context'")
		}
		// 1. Structural Neighbors (Dependency Layer)
		neighbors, err := provider.GetNeighbors(*targetPtr, *depthPtr)
		if err != nil {
			log.Fatalf("Neighbors lookup failed: %v", err)
		}

		// 2. Semantic Search (Dependency Layer)
		embedder := setupEmbedder(cfg.GoogleCloudProject, *locationPtr, model, cfg.GeminiEmbeddingDimensions)
		embeddings, err := embedder.EmbedBatch([]string{*targetPtr})
		if err != nil {
			log.Printf("Warning: Embedding failed for hybrid search: %v", err)
		}

		var similar []*query.FeatureResult
		if len(embeddings) > 0 {
			similar, _ = provider.SearchSimilarFunctions(embeddings[0], *limitPtr)
		}

		result = map[string]interface{}{
			"neighbors": neighbors,
			"similar":   similar,
		}

	case "test-context": // Alias
		fallthrough
	case "neighbors":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'neighbors'")
		}
		result, err = provider.GetNeighbors(*targetPtr, *depthPtr)

	case "impact":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'impact'")
		}
		result, err = provider.GetImpact(*targetPtr, *depthPtr)

	case "globals":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'globals'")
		}
		result, err = provider.GetGlobals(*targetPtr)

	case "seams":
		result, err = provider.GetSeams(*modulePtr)

	case "locate-usage":
		if *targetPtr == "" || *target2Ptr == "" {
			log.Fatal("-target and -target2 are required for 'locate-usage'")
		}
		result, err = provider.LocateUsage(*targetPtr, *target2Ptr)

	case "fetch-source":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'fetch-source'")
		}
		source, err := provider.FetchSource(*targetPtr)
		if err != nil {
			log.Fatalf("FetchSource failed: %v", err)
		}
		fmt.Print(source) // Print raw source to stdout
		return

	case "explore-domain":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'explore-domain'")
		}
		result, err = provider.ExploreDomain(*targetPtr)

	case "traverse":
		if *targetPtr == "" {
			log.Fatal("-target is required for 'traverse'")
		}
		dir := query.Outgoing
		switch strings.ToLower(*directionPtr) {
		case "incoming":
			dir = query.Incoming
		case "both":
			dir = query.Both
		}
		result, err = provider.Traverse(*targetPtr, *edgeTypesPtr, dir, *depthPtr)

	case "status":
		commit, err := provider.GetGraphState()
		if err != nil {
			log.Fatalf("Status check failed: %v", err)
		}
		result = map[string]string{
			"commit": commit,
		}

	default:
		log.Fatalf("Unknown or missing query type: %s. Valid types: search-features, search-similar, hybrid-context, neighbors, impact, globals, seams, explore-domain, status", *typePtr)
	}

	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		log.Fatalf("Failed to encode result: %v", err)
	}
}

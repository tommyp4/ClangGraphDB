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
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
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
		handleIngest(os.Args[2:])
	case "query":
		handleQuery(os.Args[2:])
	case "enrich-features":
		handleEnrichFeatures(os.Args[2:])
	case "import":
		handleImport(os.Args[2:])
	case "build-all":
		handleBuildAll(os.Args[2:])
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: graphdb <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  ingest           Parse code and generate graph nodes/edges (JSONL)")
	fmt.Println("  enrich-features  Build the RPG (Repository Planning Graph) Intent Layer")
	fmt.Println("  import           Import JSONL files into Neo4j")
	fmt.Println("  query            Query the graph (structural or semantic)")
	fmt.Println("  build-all        One-shot: Ingest -> Enrich -> Import")
	fmt.Println("\nRun 'graphdb <command> --help' for command-specific options.")
}

func handleBuildAll(args []string) {
	// Minimal flag parsing for build-all, or just pass through relevant flags?
	// For now, let's keep it simple: run the standard sequence with default filenames.
	// Users can still override config via ENV vars.

	fmt.Println("🚀 Starting GraphDB Build-All Sequence...")
	fmt.Println("========================================")

	// 1. Ingest
	fmt.Println("\n[Phase 1/3] Ingesting Codebase...")
	// Default to current dir, standard output
	ingestArgs := []string{"-dir", ".", "-output", "graph.jsonl"}
	// Allow user to override dir if they passed it to build-all?
	// For simplicity, let's assume build-all runs on current dir unless we implement full arg parsing.
	// Let's at least support -dir if provided.
	fs := flag.NewFlagSet("build-all", flag.ExitOnError)
	dirPtr := fs.String("dir", ".", "Directory to process")
	cleanPtr := fs.Bool("clean", true, "Clean DB before import")
	fs.Parse(args)

	ingestArgs = []string{"-dir", *dirPtr, "-output", "graph.jsonl"}
	handleIngest(ingestArgs)

	// 2. Enrich
	fmt.Println("\n[Phase 2/3] Enriching Features...")
	// Use semantic clustering by default for better quality
	enrichArgs := []string{"-dir", *dirPtr, "-input", "graph.jsonl", "-output", "rpg.jsonl"}
	handleEnrichFeatures(enrichArgs)

	// 3. Import
	fmt.Println("\n[Phase 3/3] Importing to Neo4j...")
	importArgs := []string{"-input", "rpg.jsonl"}
	if *cleanPtr {
		importArgs = append(importArgs, "-clean")
	}
	handleImport(importArgs)

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
				walker.WorkerPool.Submit(path)
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
	inputPtr := fs.String("input", "graph.jsonl", "Input graph file")
	outputPtr := fs.String("output", "rpg.jsonl", "Output file for RPG nodes and edges")
	batchSizePtr := fs.Int("batch-size", 20, "Batch size for LLM feature extraction")
	// Embedding batch size
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

	log.Println("Starting feature enrichment...")

	// 1. Load Functions from graph.jsonl
	functions, err := loadFunctions(*inputPtr)
	if err != nil {
		log.Fatalf("Failed to load functions: %v", err)
	}
	log.Printf("Loaded %d functions from %s", len(functions), *inputPtr)

	// 2. Extract atomic features per function
	extractor := setupExtractor(cfg.GoogleCloudProject, loc)
	log.Printf("Extracting atomic features (batch size: %d)...", *batchSizePtr)

	pb := ui.NewProgressBar(int64(len(functions)), "Extracting features")

	for i := range functions {
		fn := &functions[i]
		name, _ := fn.Properties["name"].(string)
		code, _ := fn.Properties["content"].(string)

		descriptors, err := extractor.Extract(code, name)
		if err != nil {
			// Skip logging to avoid breaking progress bar
			continue
		}
		fn.Properties["atomic_features"] = descriptors
		pb.Add(1)
	}
	pb.Finish()
	log.Printf("Extracted atomic features for %d functions", len(functions))

	// 3. Setup Builder & Clusterer
	embedder := setupEmbedder(cfg.GoogleCloudProject, loc, model, cfg.GeminiEmbeddingDimensions)

	// PRE-CALCULATION STEP
	log.Println("Pre-calculating embeddings for clustering...")
	precomputed := make(map[string][]float32)

	// Batch process functions for embedding
	embedPb := ui.NewProgressBar(int64(len(functions)), "Generating Embeddings")
	batchSize := *embedBatchSizePtr

	var batchTexts []string
	var batchIDs []string

	for i, fn := range functions {
		text := rpg.NodeToText(fn)
		batchTexts = append(batchTexts, text)
		batchIDs = append(batchIDs, fn.ID)

		if len(batchTexts) >= batchSize || i == len(functions)-1 {
			embeddings, err := embedder.EmbedBatch(batchTexts)
			if err != nil {
				log.Printf("Warning: Failed to embed batch: %v", err)
			} else {
				for j, emb := range embeddings {
					precomputed[batchIDs[j]] = emb
				}
			}
			embedPb.Add(int64(len(batchTexts)))
			batchTexts = batchTexts[:0]
			batchIDs = batchIDs[:0]
		}
	}
	embedPb.Finish()
	log.Printf("Generated embeddings for %d functions", len(precomputed))

	clusterer := &rpg.EmbeddingClusterer{
		Embedder:              embedder,
		PrecomputedEmbeddings: precomputed,
	}

	// Create Global Clusterer for initial domain discovery
	globalClusterer := &rpg.EmbeddingClusterer{
		Embedder:              embedder,
		PrecomputedEmbeddings: precomputed,
		KStrategy: func(n int) int {
			if n == 0 {
				return 0
			}
			// Rule of thumb: sqrt(N/10)
			k := int(math.Sqrt(float64(n) / 10.0))
			if k < 2 {
				return 2
			}
			return k
		},
	}
	log.Println("Using Global Discovery Mode (semantic clustering)")

	builder := &rpg.Builder{
		Discoverer: &rpg.DirectoryDomainDiscoverer{
			BaseDirs: []string{"."},
		},
		Clusterer:       clusterer,
		GlobalClusterer: globalClusterer,
	}

	var clusterPb *ui.ProgressBar
	builder.OnPhaseStart = func(phaseName string, total int) {
		if total > 0 {
			clusterPb = ui.NewProgressBar(int64(total), phaseName)
			clusterPb.Add(0) // Render initial state
		}
	}
	builder.OnStepStart = func(stepName string) {
		if clusterPb != nil {
			clusterPb.UpdateDescription(fmt.Sprintf("Clustering %s", stepName))
		}
	}
	builder.OnStepEnd = func(stepName string) {
		if clusterPb != nil {
			clusterPb.Add(1)
		}
	}

	// 4. Build Feature Hierarchy
	features, edges, err := builder.Build(*dirPtr, functions)
	if clusterPb != nil {
		clusterPb.Finish()
	}
	if err != nil {
		log.Fatalf("Failed to build features: %v", err)
	}

	// 5. Setup Enricher
	summarizer := setupSummarizer(cfg.GoogleCloudProject, loc)
	enricher := &rpg.Enricher{
		Client:   summarizer,
		Embedder: embedder,
	}

	// 6. Enrich Features (recursively, using scoped member functions)
	var totalFeatures int64
	var countFeatures func(f *rpg.Feature)
	countFeatures = func(f *rpg.Feature) {
		totalFeatures++
		for _, child := range f.Children {
			countFeatures(child)
		}
	}
	for i := range features {
		countFeatures(&features[i])
	}

	pb = ui.NewProgressBar(totalFeatures, "Enriching features")

	var enrichAll func(f *rpg.Feature)
	enrichAll = func(f *rpg.Feature) {
		if err := enricher.Enrich(f, f.MemberFunctions); err != nil {
			// Skip logging to keep PB clean
		}
		pb.Add(1)
		for _, child := range f.Children {
			enrichAll(child)
		}
	}
	for i := range features {
		enrichAll(&features[i])
	}
	pb.Finish()

	// 7. Flatten for Persistence
	nodes, allEdges := rpg.Flatten(features, edges)

	// 8. Persistence (Emit to storage)
	outFile, err := os.Create(*outputPtr)
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	defer outFile.Close()
	emitter := storage.NewJSONLEmitter(outFile)
	defer emitter.Close()

	for i := range nodes {
		if err := emitter.EmitNode(&nodes[i]); err != nil {
			log.Printf("Warning: failed to emit node: %v", err)
		}
	}
	for i := range allEdges {
		if err := emitter.EmitEdge(&allEdges[i]); err != nil {
			log.Printf("Warning: failed to emit edge: %v", err)
		}
	}

	log.Printf("Successfully emitted %d nodes and %d edges to %s", len(nodes), len(allEdges), *outputPtr)
}

func handleImport(args []string) {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	nodesPtr := fs.String("nodes", "", "Path to nodes JSONL file")
	edgesPtr := fs.String("edges", "", "Path to edges JSONL file")
	inputPtr := fs.String("input", "", "Path to combined JSONL file (nodes + edges)")
	batchSizePtr := fs.Int("batch-size", 500, "Batch size for insertion")
	cleanPtr := fs.Bool("clean", false, "Wipe database before importing")

	fs.Parse(args)

	if *nodesPtr == "" && *edgesPtr == "" && *inputPtr == "" {
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

func loadFunctions(path string) ([]graph.Node, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var nodes []graph.Node
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var raw map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
			continue
		}

		// Check if it is a node and a Function
		if typeVal, ok := raw["type"].(string); ok && typeVal == "Function" {
			// Reconstruct node
			id, _ := raw["id"].(string)
			node := graph.Node{
				ID:         id,
				Label:      "Function",
				Properties: raw,
			}
			nodes = append(nodes, node)
		}
	}
	return nodes, scanner.Err()
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

	provider, err := query.NewNeo4jProvider(cfg)
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

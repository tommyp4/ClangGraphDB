package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"clang-graphdb/internal/clangast"
	"clang-graphdb/internal/config"
	"clang-graphdb/internal/vcxproj"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func handleClangIncremental(args []string) {
	flags := flag.NewFlagSet("clang-incremental", flag.ExitOnError)
	slnPtr := flags.String("sln", "", "Path to .sln file (required)")
	configPtr := flags.String("config", "Debug|Win32", "Build configuration")
	outputPtr := flags.String("output", ".", "Output directory for nodes.jsonl and edges.jsonl")
	workersPtr := flags.Int("workers", 16, "Number of parallel clang workers")
	verbosePtr := flags.Bool("verbose", false, "Verbose output")
	sincePtr := flags.String("since", "HEAD~1", "Git ref to diff against (commit, branch, tag)")
	dryRun := flags.Bool("dry-run", false, "Show affected files without extracting or updating Neo4j")

	flags.Parse(args)

	if *slnPtr == "" {
		log.Fatal("clang-incremental: -sln is required")
	}

	start := time.Now()

	// Load .env for Neo4j credentials
	config.LoadEnv()
	cfg := config.LoadConfig()
	if cfg.Neo4jURI == "" {
		log.Fatal("NEO4J_URI not set. Incremental mode requires Neo4j connection.")
	}

	repoRoot := findRepoRoot(*slnPtr)
	log.Printf("Repo root: %s", repoRoot)

	// Step 1: Git diff to find changed files
	log.Printf("[Step 1/4] Finding changed files since %s...", *sincePtr)
	changedFiles := gitDiffFiles(repoRoot, *sincePtr)
	if len(changedFiles) == 0 {
		log.Printf("No changed files found. Nothing to do.")
		return
	}
	log.Printf("  %d files changed", len(changedFiles))

	// Separate into headers and source files
	var changedHeaders, changedSources []string
	for _, f := range changedFiles {
		ext := strings.ToLower(filepath.Ext(f))
		switch ext {
		case ".h", ".hpp", ".hxx", ".inl":
			changedHeaders = append(changedHeaders, f)
		case ".cpp", ".c", ".cc", ".cxx":
			changedSources = append(changedSources, f)
		}
	}
	log.Printf("  %d headers, %d source files", len(changedHeaders), len(changedSources))

	// Step 2: Query Neo4j for transitive includers of changed headers
	log.Printf("[Step 2/4] Finding affected files via INCLUDES graph...")

	driver, err := neo4j.NewDriverWithContext(cfg.Neo4jURI, neo4j.BasicAuth(cfg.Neo4jUser, cfg.Neo4jPassword, ""))
	if err != nil {
		log.Fatalf("Failed to connect to Neo4j: %v", err)
	}
	defer driver.Close(context.Background())

	affectedFiles := findAffectedFiles(driver, changedHeaders, changedSources, *verbosePtr)
	log.Printf("  %d total affected files (changed + transitive includers)", len(affectedFiles))

	if *verbosePtr {
		for _, f := range affectedFiles {
			log.Printf("    %s", f)
		}
	}

	if *dryRun {
		log.Printf("Dry run complete. %d files would be re-extracted.", len(affectedFiles))
		return
	}

	// Step 3: Delete old data for affected files from Neo4j
	log.Printf("[Step 3/4] Deleting old graph data for %d affected files...", len(affectedFiles))
	deleted := deleteAffectedData(driver, affectedFiles)
	log.Printf("  Deleted %d nodes and their relationships", deleted)

	// Step 4: Re-extract affected files and import
	log.Printf("[Step 4/4] Re-extracting %d files...", len(affectedFiles))

	// Parse solution to get compile commands for affected files
	sol, err := vcxproj.ParseSolution(*slnPtr)
	if err != nil {
		log.Fatalf("Failed to parse solution: %v", err)
	}

	parts := strings.SplitN(*configPtr, "|", 2)
	configName := parts[0]
	platform := "Win32"
	if len(parts) == 2 {
		platform = parts[1]
	}

	var parsedProjects []*vcxproj.ParsedProject
	for _, sp := range sol.CppProjects() {
		projPath := sol.ResolveProjectPath(sp)
		if _, err := os.Stat(projPath); err != nil {
			continue
		}
		resolver := vcxproj.NewVarResolver()
		resolver.SetupDefaults(filepath.Dir(projPath), sol.Dir, configName, platform)
		proj, err := vcxproj.ParseVcxproj(projPath, *configPtr, resolver)
		if err != nil {
			continue
		}
		proj.GUID = sp.GUID
		if proj.Name == "" {
			proj.Name = sp.Name
		}
		cfg := proj.Configs[*configPtr]
		if cfg != nil && strings.ToLower(cfg.CLRSupport) == "true" {
			continue
		}
		parsedProjects = append(parsedProjects, proj)
	}

	allCommands := vcxproj.GenerateCompileCommands(parsedProjects, *configPtr)

	// Filter to only affected files
	affectedSet := make(map[string]bool)
	for _, f := range affectedFiles {
		affectedSet[strings.ToLower(filepath.ToSlash(f))] = true
	}

	var commands []vcxproj.CompileCommand
	for _, cmd := range allCommands {
		relPath := strings.ToLower(filepath.ToSlash(makeRelativePath(cmd.File, repoRoot)))
		if affectedSet[relPath] {
			commands = append(commands, cmd)
		}
	}
	log.Printf("  Matched %d compile commands for affected files", len(commands))

	if len(commands) == 0 {
		log.Printf("No compile commands matched affected files. Done.")
		return
	}

	// Extract AST
	clangPath := findClangCL()
	if clangPath == "" {
		log.Fatal("clang-cl.exe not found.")
	}

	nodesPath := filepath.Join(*outputPtr, "nodes_incremental.jsonl")
	edgesPath := filepath.Join(*outputPtr, "edges_incremental.jsonl")

	os.MkdirAll(*outputPtr, 0755)

	nodesFile, err := os.Create(nodesPath)
	if err != nil {
		log.Fatalf("Failed to create nodes file: %v", err)
	}
	defer nodesFile.Close()

	edgesFile, err := os.Create(edgesPath)
	if err != nil {
		log.Fatalf("Failed to create edges file: %v", err)
	}
	defer edgesFile.Close()

	extractor := clangast.NewExtractor(clangPath, repoRoot, nodesFile, edgesFile)
	extractor.Workers = *workersPtr
	extractor.Verbose = *verbosePtr

	// Emit File nodes
	for _, cmd := range commands {
		relPath := makeRelativePath(cmd.File, repoRoot)
		fileID := "File:" + relPath
		data, _ := json.Marshal(map[string]string{
			"id":   fileID,
			"type": "File",
			"name": relPath,
			"file": relPath,
		})
		data = append(data, '\n')
		nodesFile.Write(data)
	}

	succeeded, failed := extractor.Run(commands)
	log.Printf("  AST extraction: %d succeeded, %d failed", succeeded, failed)
	log.Printf("  Nodes: %d, Edges: %d", extractor.NodeCount(), extractor.EdgeCount())

	// Extract includes
	log.Printf("  Extracting #include relationships...")
	extractor.ExtractIncludes(commands)
	log.Printf("  Total edges after includes: %d", extractor.EdgeCount())

	// Import into Neo4j
	log.Printf("Importing into Neo4j...")
	importJSONL(driver, nodesPath, edgesPath)

	log.Printf("Incremental update complete in %v.", time.Since(start))
	log.Printf("  Changed: %d files, Affected: %d files, Extracted: %d files",
		len(changedFiles), len(affectedFiles), len(commands))
}

func gitDiffFiles(repoRoot, since string) []string {
	cmd := exec.Command("git", "diff", "--name-only", since)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		log.Printf("Warning: git diff failed: %v", err)
		return nil
	}

	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

func findAffectedFiles(driver neo4j.DriverWithContext, changedHeaders, changedSources []string, verbose bool) []string {
	ctx := context.Background()
	affected := make(map[string]bool)

	// All changed source files are directly affected
	for _, f := range changedSources {
		affected[f] = true
	}

	// For each changed header, find all files that transitively include it
	if len(changedHeaders) > 0 {
		session := driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		// Convert header paths to File: IDs
		var headerIDs []string
		for _, h := range changedHeaders {
			headerIDs = append(headerIDs, "File:"+filepath.ToSlash(h))
		}

		// Transitive INCLUDES query: find all files that directly or indirectly
		// include any of the changed headers
		query := `
			UNWIND $headerIds AS headerId
			MATCH (includer:File)-[:INCLUDES*1..]->(header:CodeElement {id: headerId})
			RETURN DISTINCT includer.file AS file
		`

		result, err := session.Run(ctx, query, map[string]any{"headerIds": headerIDs})
		if err != nil {
			log.Printf("Warning: Neo4j INCLUDES query failed: %v", err)
			// Fall back: just include changed headers as affected
			for _, h := range changedHeaders {
				affected[h] = true
			}
		} else {
			for result.Next(ctx) {
				file, ok := result.Record().Get("file")
				if ok && file != nil {
					affected[file.(string)] = true
				}
			}
		}

		// The changed headers themselves are also affected
		for _, h := range changedHeaders {
			affected[h] = true
		}

		if verbose {
			log.Printf("  Headers changed: %d, transitive includers found: %d",
				len(changedHeaders), len(affected)-len(changedSources)-len(changedHeaders))
		}
	}

	var result []string
	for f := range affected {
		result = append(result, f)
	}
	return result
}

func deleteAffectedData(driver neo4j.DriverWithContext, affectedFiles []string) int64 {
	ctx := context.Background()
	session := driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	var totalDeleted int64

	// Delete in batches to avoid transaction size limits
	batchSize := 100
	for i := 0; i < len(affectedFiles); i += batchSize {
		end := i + batchSize
		if end > len(affectedFiles) {
			end = len(affectedFiles)
		}
		batch := affectedFiles[i:end]

		// Delete all nodes (and their relationships) where file matches
		query := `
			UNWIND $files AS filePath
			MATCH (n:CodeElement)
			WHERE n.file = filePath
			DETACH DELETE n
			RETURN count(n) AS deleted
		`

		res, err := session.Run(ctx, query, map[string]any{"files": batch})
		if err != nil {
			log.Printf("Warning: delete batch failed: %v", err)
			continue
		}
		if res.Next(ctx) {
			if d, ok := res.Record().Get("deleted"); ok {
				totalDeleted += d.(int64)
			}
		}
	}

	return totalDeleted
}

func importJSONL(driver neo4j.DriverWithContext, nodesPath, edgesPath string) {
	ctx := context.Background()
	session := driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	// Import nodes
	nodesData, err := os.ReadFile(nodesPath)
	if err != nil {
		log.Printf("Warning: failed to read nodes: %v", err)
		return
	}

	var nodeBatch []map[string]any
	for _, line := range strings.Split(string(nodesData), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var node map[string]any
		if json.Unmarshal([]byte(line), &node) == nil {
			if _, hasSource := node["source"]; hasSource {
				continue // skip edges
			}
			nodeBatch = append(nodeBatch, node)
		}

		if len(nodeBatch) >= 500 {
			flushNodes(session, ctx, nodeBatch)
			nodeBatch = nodeBatch[:0]
		}
	}
	if len(nodeBatch) > 0 {
		flushNodes(session, ctx, nodeBatch)
	}

	// Import edges
	edgesData, err := os.ReadFile(edgesPath)
	if err != nil {
		log.Printf("Warning: failed to read edges: %v", err)
		return
	}

	edgesByType := make(map[string][]map[string]any)
	for _, line := range strings.Split(string(edgesData), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var edge map[string]any
		if json.Unmarshal([]byte(line), &edge) == nil {
			if _, hasSource := edge["source"]; !hasSource {
				continue // skip nodes
			}
			edgeType, _ := edge["type"].(string)
			if edgeType == "" {
				edgeType = "RELATED_TO"
			}
			edgesByType[edgeType] = append(edgesByType[edgeType], edge)
		}
	}

	for edgeType, edges := range edgesByType {
		for i := 0; i < len(edges); i += 500 {
			end := i + 500
			if end > len(edges) {
				end = len(edges)
			}
			flushEdges(session, ctx, edgeType, edges[i:end])
		}
	}

	log.Printf("  Import complete")
}

func flushNodes(session neo4j.SessionWithContext, ctx context.Context, nodes []map[string]any) {
	// Group by type/label
	byLabel := make(map[string][]map[string]any)
	for _, n := range nodes {
		label, _ := n["type"].(string)
		if label == "" {
			label = "Generic"
		}
		byLabel[label] = append(byLabel[label], n)
	}

	for label, batch := range byLabel {
		safeLabel := sanitizeLabelName(label)
		query := fmt.Sprintf(`
			UNWIND $batch AS row
			MERGE (n:CodeElement {id: row.id})
			SET n += row
			SET n:%s
		`, safeLabel)

		_, err := session.Run(ctx, query, map[string]any{"batch": batch})
		if err != nil {
			log.Printf("Warning: node import failed for label %s: %v", label, err)
		}
	}
}

func flushEdges(session neo4j.SessionWithContext, ctx context.Context, edgeType string, edges []map[string]any) {
	var batch []map[string]any
	for _, e := range edges {
		src, _ := e["source"].(string)
		tgt, _ := e["target"].(string)
		if src != "" && tgt != "" {
			batch = append(batch, map[string]any{"sourceId": src, "targetId": tgt})
		}
	}

	safeType := sanitizeLabelName(edgeType)
	query := fmt.Sprintf(`
		UNWIND $batch AS row
		MATCH (source:CodeElement {id: row.sourceId})
		MATCH (target:CodeElement) WHERE target.id = row.targetId OR target.fqn = row.targetId
		MERGE (source)-[r:%s]->(target)
	`, safeType)

	_, err := session.Run(ctx, query, map[string]any{"batch": batch})
	if err != nil {
		log.Printf("Warning: edge import failed for type %s: %v", edgeType, err)
	}
}

func sanitizeLabelName(label string) string {
	var sb strings.Builder
	for _, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"clang-graphdb/internal/clangast"
	"clang-graphdb/internal/vcxproj"
)

func handleClangIngest(args []string) {
	flags := flag.NewFlagSet("clang-ingest", flag.ExitOnError)
	slnPtr := flags.String("sln", "", "Path to .sln file (required)")
	configPtr := flags.String("config", "Debug|Win32", "Build configuration")
	outputPtr := flags.String("output", ".", "Output directory for nodes.jsonl and edges.jsonl")
	noExtract := flags.Bool("no-extract", false, "Only generate compile_commands.json, skip Clang extraction")
	workersPtr := flags.Int("workers", 4, "Number of parallel clang workers")
	verbosePtr := flags.Bool("verbose", false, "Verbose output")
	includesPtr := flags.Bool("includes", true, "Extract #include relationships")
	projectFilter := flags.String("project", "", "Only process this project name (for testing)")

	flags.Parse(args)

	if *slnPtr == "" {
		log.Fatal("clang-ingest: -sln is required")
	}

	start := time.Now()

	// Step 1: Parse solution
	log.Printf("[Step 1/5] Parsing solution: %s", *slnPtr)
	sol, err := vcxproj.ParseSolution(*slnPtr)
	if err != nil {
		log.Fatalf("Failed to parse solution: %v", err)
	}

	cppProjects := sol.CppProjects()
	log.Printf("  Found %d total projects, %d C++ projects", len(sol.Projects), len(cppProjects))

	// Step 2: Parse each vcxproj
	log.Printf("[Step 2/5] Parsing .vcxproj files...")
	parts := strings.SplitN(*configPtr, "|", 2)
	configName := parts[0]
	platform := "Win32"
	if len(parts) == 2 {
		platform = parts[1]
	}

	guidMap := sol.BuildGUIDMap()
	var parsedProjects []*vcxproj.ParsedProject
	skippedCLR := 0
	parseErrors := 0

	for _, sp := range cppProjects {
		projPath := sol.ResolveProjectPath(sp)
		if _, err := os.Stat(projPath); err != nil {
			if *verbosePtr {
				log.Printf("  [SKIP] %s: file not found", sp.Name)
			}
			continue
		}

		resolver := vcxproj.NewVarResolver()
		resolver.SetupDefaults(filepath.Dir(projPath), sol.Dir, configName, platform)

		proj, err := vcxproj.ParseVcxproj(projPath, *configPtr, resolver)
		if err != nil {
			log.Printf("  [ERROR] %s: %v", sp.Name, err)
			parseErrors++
			continue
		}

		proj.GUID = sp.GUID
		if proj.Name == "" {
			proj.Name = sp.Name
		}

		config := proj.Configs[*configPtr]
		if config != nil && strings.ToLower(config.CLRSupport) == "true" {
			if *verbosePtr {
				log.Printf("  [SKIP-CLR] %s: entire project is CLR", sp.Name)
			}
			skippedCLR++
			continue
		}

		parsedProjects = append(parsedProjects, proj)
	}

	log.Printf("  Parsed %d projects (%d CLR-skipped, %d errors)", len(parsedProjects), skippedCLR, parseErrors)

	if *projectFilter != "" {
		var filtered []*vcxproj.ParsedProject
		for _, p := range parsedProjects {
			if strings.EqualFold(p.Name, *projectFilter) {
				filtered = append(filtered, p)
			}
		}
		if len(filtered) == 0 {
			log.Fatalf("Project %q not found among parsed projects", *projectFilter)
		}
		parsedProjects = filtered
		var filteredCpp []vcxproj.SolutionProject
		for _, sp := range cppProjects {
			if strings.EqualFold(sp.Name, *projectFilter) {
				filteredCpp = append(filteredCpp, sp)
			}
		}
		cppProjects = filteredCpp
		log.Printf("  Filtered to project: %s (%d projects)", *projectFilter, len(parsedProjects))
	}

	// Step 3: Generate compile_commands.json
	log.Printf("[Step 3/5] Generating compile_commands.json...")
	commands := vcxproj.GenerateCompileCommands(parsedProjects, *configPtr)
	log.Printf("  Generated %d compile commands", len(commands))

	ccPath := filepath.Join(*outputPtr, "compile_commands.json")
	if err := os.MkdirAll(*outputPtr, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}
	if err := vcxproj.WriteCompileCommands(commands, ccPath); err != nil {
		log.Fatalf("Failed to write compile_commands.json: %v", err)
	}
	log.Printf("  Written to %s", ccPath)

	if *noExtract {
		log.Printf("Done (--no-extract). compile_commands.json generated in %v.", time.Since(start))
		return
	}

	// Find clang-cl (MSVC-compatible driver)
	clangPath := findClangCL()
	if clangPath == "" {
		log.Fatal("clang-cl.exe not found. Install LLVM or add clang-cl to PATH.")
	}
	log.Printf("  Using clang: %s", clangPath)

	repoRoot := findRepoRoot(*slnPtr)
	log.Printf("  Repo root: %s", repoRoot)

	// Step 4: Run AST extraction via clang JSON dump
	log.Printf("[Step 4/5] Extracting AST from %d files (%d workers)...", len(commands), *workersPtr)

	nodesPath := filepath.Join(*outputPtr, "nodes.jsonl")
	edgesPath := filepath.Join(*outputPtr, "edges.jsonl")

	nodesFile, err := os.Create(nodesPath)
	if err != nil {
		log.Fatalf("Failed to create nodes.jsonl: %v", err)
	}
	defer nodesFile.Close()

	edgesFile, err := os.Create(edgesPath)
	if err != nil {
		log.Fatalf("Failed to create edges.jsonl: %v", err)
	}
	defer edgesFile.Close()

	extractor := clangast.NewExtractor(clangPath, repoRoot, nodesFile, edgesFile)
	extractor.Workers = *workersPtr
	extractor.Verbose = *verbosePtr

	// Emit File nodes for all source files first
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

	// Extract #include relationships
	if *includesPtr {
		log.Printf("  Extracting #include relationships...")
		extractor.ExtractIncludes(commands)
		log.Printf("  Total edges after includes: %d", extractor.EdgeCount())
	}

	// Step 5: Append project-level graph data
	log.Printf("[Step 5/5] Appending project graph data...")

	nodeEnc := json.NewEncoder(nodesFile)
	edgeEnc := json.NewEncoder(edgesFile)

	projectNodes := 0
	projectEdges := 0

	for _, proj := range parsedProjects {
		config := proj.Configs[*configPtr]
		configType := ""
		if config != nil {
			configType = config.ConfigurationType
		}

		projID := "Project:" + proj.Name

		nodeEnc.Encode(map[string]interface{}{
			"id":          projID,
			"type":        "Project",
			"name":        proj.Name,
			"file":        makeRelativePath(proj.VcxprojPath, repoRoot),
			"config_type": configType,
			"guid":        proj.GUID,
		})
		projectNodes++

		for _, sf := range proj.SourceFiles {
			if sf.IsCLR {
				continue
			}
			relPath := makeRelativePath(sf.AbsolutePath, repoRoot)
			fileID := "File:" + relPath
			edgeEnc.Encode(map[string]string{
				"source": projID,
				"target": fileID,
				"type":   "PROJECT_CONTAINS",
			})
			projectEdges++
		}
	}

	for _, sp := range cppProjects {
		projID := "Project:" + sp.Name
		for _, depGUID := range sp.Dependencies {
			if depName, ok := guidMap[depGUID]; ok {
				depID := "Project:" + depName
				edgeEnc.Encode(map[string]string{
					"source": projID,
					"target": depID,
					"type":   "PROJECT_DEPENDS_ON",
				})
				projectEdges++
			}
		}
	}

	for _, proj := range parsedProjects {
		projID := "Project:" + proj.Name
		for _, ref := range proj.ProjectReferences {
			depName := ref.Name
			if depName == "" {
				if n, ok := guidMap[ref.GUID]; ok {
					depName = n
				}
			}
			if depName != "" {
				depID := "Project:" + depName
				edgeEnc.Encode(map[string]string{
					"source": projID,
					"target": depID,
					"type":   "PROJECT_DEPENDS_ON",
				})
				projectEdges++
			}
		}
	}

	log.Printf("  Added %d project nodes, %d project edges", projectNodes, projectEdges)
	log.Printf("Done in %v.", time.Since(start))
}

func findClangCL() string {
	if p, err := exec.LookPath("clang-cl"); err == nil {
		return p
	}

	candidates := []string{
		`C:\Program Files\LLVM\bin\clang-cl.exe`,
		`C:\Program Files (x86)\LLVM\bin\clang-cl.exe`,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func findRepoRoot(slnPath string) string {
	abs, _ := filepath.Abs(slnPath)
	dir := filepath.Dir(abs)

	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return filepath.Dir(filepath.Dir(abs))
}

func makeRelativePath(absPath, repoRoot string) string {
	rel, err := filepath.Rel(repoRoot, absPath)
	if err != nil {
		return absPath
	}
	return filepath.ToSlash(rel)
}

# Feature Implementation Plan: Global File Logging

## 🔍 Analysis & Context
*   **Objective:** Add a persistent file-based logging mechanism to the `graphdb` Go binary to facilitate post-execution debugging without losing console output.
*   **Affected Files:** 
    *   `test/e2e/cli_test.go`
    *   `cmd/graphdb/main.go`
*   **Key Dependencies:** Standard `log`, `os`, `io`, and `strings` packages.
*   **Risks/Edge Cases:** 
    *   Breaking subcommand flag parsing (mitigated by extracting the global flag from `os.Args` manually before delegating to subcommands).
    *   Interfering with command stdout like JSON query outputs (mitigated by only redirecting `log`, which outputs to `stderr` by default).
    *   Interfering with progress bars (mitigated as progress bars use explicit `fmt.Fprintf(os.Stderr)` and won't be captured by the `log` redirection, keeping the log file clean of `\r` frame animation spam).

## 📋 Micro-Step Checklist
- [x] Phase 1: The Verification Harness
  - [x] Step 1.A: Add E2E test for the global `--log-file` flag in `cli_test.go`. (Status: ✅ Implemented)
- [x] Phase 2: Implementation of Global Logging
  - [x] Step 2.A: Extract `--log-file` flag and `GRAPHDB_LOG_FILE` env var in `main.go`. (Status: ✅ Implemented)
  - [x] Step 2.B: Configure standard logger with `io.MultiWriter` and enhanced `Lshortfile` flags. (Status: ✅ Implemented)
  - [x] Step 2.C: Update CLI usage documentation to surface the new feature. (Status: ✅ Implemented)
- [x] Phase 3: Verification
  - [x] Step 3.A: Run the new E2E test to confirm behavior and ensure no regressions. (Status: ✅ Verified)

## 📝 Step-by-Step Implementation Details

### Prerequisites
Ensure you are in the `graphdb-skill` root directory.

#### Phase 1: The Verification Harness
1.  **Step 1.A (The E2E Test):** Create an automated test to verify the `--log-file` argument works, captures standard log formatting, and doesn't break subcommand arguments.
    *   *Target File:* `test/e2e/cli_test.go`
    *   *Exact Change:* Add a new test function `TestCLI_GlobalLogFile` at the bottom of the file:

```go
func TestCLI_GlobalLogFile(t *testing.T) {
	cliPath := buildCLI(t)
	root := getRepoRoot(t)

	logFilePath := filepath.Join(root, "test_global.log")
	defer os.Remove(logFilePath)

	// Run an ingest command that will quickly fail or complete, but we pass the new global flag
	cmd := exec.Command(cliPath, "--log-file="+logFilePath, "ingest", "-dir", "nonexistent_dir_for_test")
	cmd.Env = append(os.Environ(), "GRAPHDB_MOCK_ENABLED=true")
	_ = cmd.Run() // It's okay if it fails (due to nonexistent dir), we just care that it logs

	// Verify log file exists
	content, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("Log file was not created: %v", err)
	}

	// Verify log content has standard log formatting (timestamp) and contains an expected message
	outStr := string(content)
	if !strings.Contains(outStr, "nonexistent_dir_for_test") && !strings.Contains(outStr, "Found") && !strings.Contains(outStr, "failed") {
		t.Errorf("Log file missing expected content. Got: %s", outStr)
	}
	
	// Check that we see shortfile info (main.go or cmd_ingest.go)
	if !strings.Contains(outStr, ".go:") {
		t.Errorf("Log file missing file/line numbers. Got: %s", outStr)
	}
}
```

#### Phase 2: Implementation of Global Logging
1.  **Step 2.A (Extract Flag & Set Up Logging):** Modify `main.go` to extract the global flag before command routing. By filtering `--log-file` out of the arguments array early, subcommands won't fail with "unknown flag" errors when parsing their specific `FlagSet`.
    *   *Target File:* `cmd/graphdb/main.go`
    *   *Exact Change:* Update the `import` block to include `"io"`, `"log"`, and `"strings"`. Then replace the beginning of `main()` (up to the `switch` statement) with the following logic:

```go
func main() {
	// Attempt to load .env file from current or parent directories
	_ = config.LoadEnv()

	// Extract global logging flags before subcommand routing
	var logFile string
	var args []string

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "--log-file=") || strings.HasPrefix(arg, "-log-file=") {
			parts := strings.SplitN(arg, "=", 2)
			logFile = parts[1]
		} else if (arg == "--log-file" || arg == "-log-file") && i+1 < len(os.Args) {
			logFile = os.Args[i+1]
			i++ // skip next arg
		} else {
			args = append(args, arg)
		}
	}

	// Fallback to environment variable
	if logFile == "" {
		logFile = os.Getenv("GRAPHDB_LOG_FILE")
	}

	// Configure logging if requested
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("Failed to open log file %s: %v", logFile, err)
		}
		defer f.Close()

		// Write to both standard error (default) and the log file
		mw := io.MultiWriter(os.Stderr, f)
		log.SetOutput(mw)
		// Add timestamp, microseconds, and file/line number for robust debugging
		log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	}

	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "ingest":
		ingestCmd(cmdArgs)
	case "query":
		handleQuery(cmdArgs)
	case "enrich-features":
		enrichCmd(cmdArgs)
	case "enrich-contamination":
		handleEnrichContamination(cmdArgs)
	case "enrich-history":
		handleEnrichHistory(cmdArgs)
	case "enrich-tests":
		handleEnrichTests(cmdArgs)
	case "import":
		importCmd(cmdArgs)
	case "serve":
		handleServe(cmdArgs)
	case "build-all":
		handleBuildAll(cmdArgs)
	case "version", "--version", "-v":
		fmt.Printf("graphdb version %s\n", Version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}
```

2.  **Step 2.B (Update Usage Docs):** Update the `printUsage` function to reflect the new global capabilities.
    *   *Target File:* `cmd/graphdb/main.go`
    *   *Exact Change:* Modify the `printUsage()` function to include the `[global options]` hint and document the `--log-file` argument.

```go
func printUsage() {
	fmt.Printf("GraphDB Skill CLI (Version: %s)\n", Version)
	fmt.Println("Usage: graphdb [global options] <command> [options]")
	fmt.Println("\nGlobal Options:")
	fmt.Println("  --log-file <path>      Output standard logs to the specified file (or set GRAPHDB_LOG_FILE env var)")
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
```

#### Phase 3: The Verification
1.  **Step 3.A (Verify Functionality):** Run the new harness.
    *   *Action:* Run `go test ./test/e2e/ -v -run TestCLI_GlobalLogFile`.
    *   *Success:* Test passes, validating that `test_global.log` is properly created, formatted, and doesn't break existing subcommand logic.
    *   *Action:* Run all unit tests to ensure no regressions: `go test ./...`.

## 🎯 Success Criteria
*   The `--log-file` global flag correctly configures logging to the given file across all subcommands.
*   Log output is dual-written to `os.Stderr` and the requested file simultaneously.
*   File output includes detailed traces (`log.Lshortfile`) for precise post-execution debugging.
*   Progress bar text animation correctly continues *only* on stdout/stderr without filling the log file with `\r` carriage return frame resets.
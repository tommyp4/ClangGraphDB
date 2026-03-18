# Campaign 3.9: Unknown Domain Fix (LLM Validation & Error Handling)

**Goal:** Fix the silent fallback to "Unknown Domain" when Vertex AI client fails due to missing GOOGLE_CLOUD_PROJECT. Implement early validation, proper error handling, and graceful degradation options.

## 📋 Todo Checklist

- [ ] **Phase 1: Early Validation & Fail-Fast**
  - [ ] Add GCP validation in `cmd/graphdb/main.go`
  - [ ] Create validation helper function
  - [ ] Fail fast for LLM-dependent commands
  - [ ] Add test coverage

- [ ] **Phase 2: Error Propagation & User Feedback**
  - [ ] Fix silent error swallowing in `GlobalEmbeddingClusterer`
  - [ ] Add proper error reporting to enrichment pipeline
  - [ ] Implement structured error messages with actionable guidance

- [ ] **Phase 3: Graceful Degradation Options**
  - [ ] Add `--skip-llm` flag for LLM-free operation
  - [ ] Implement mock/fallback summarizer for testing
  - [ ] Update documentation with credential requirements

- [ ] **Phase 4: Client Validation & Health Checks**
  - [ ] Add Vertex AI client validation
  - [ ] Implement health check before starting enrichment
  - [ ] Add retry logic with exponential backoff

- [ ] Final Review and Testing

## 🔍 Analysis & Investigation

### Root Cause Analysis

The current implementation has multiple failure points that lead to the "Unknown Domain" issue:

1. **No Validation of GOOGLE_CLOUD_PROJECT**
   - `config.LoadConfig()` returns an empty string when not set
   - `genai.NewClient` accepts empty project ID without immediate error
   - Failure only occurs at runtime during API calls

2. **Silent Error Handling in GlobalEmbeddingClusterer**
   - Lines 46-51 in `cluster_global.go` catch summarization errors
   - Falls back to "Unknown Domain" without user notification
   - No error is propagated up the call stack

3. **Missing Description Cascade**
   - When summarization fails, description is empty
   - Empty descriptions cause embedding generation to be skipped
   - Results in Features with no semantic information

4. **No User Guidance**
   - Users aren't informed about missing credentials
   - No clear error message about what went wrong
   - No option to proceed without LLM features

### Current Code Flow

```
main.go:handleEnrichFeatures()
  ├── setupExtractor(cfg.GoogleCloudProject, loc)  // No validation
  ├── setupSummarizer(cfg.GoogleCloudProject, loc)  // No validation
  ├── GlobalEmbeddingClusterer.Cluster()
  │   └── Summarizer.Summarize()  // Fails here
  │       └── Returns error (silently caught)
  └── Feature nodes created with "Unknown Domain N"
```

### Dependencies & Integration Points

- **Config Loading:** `internal/config/loader.go`
- **Vertex Setup:** `cmd/graphdb/setup_prod.go`
- **Global Clusterer:** `internal/rpg/cluster_global.go`
- **Enricher:** `internal/rpg/enrich.go`
- **Main Entry Points:** `cmd/graphdb/main.go`

## 📝 Implementation Plan

### Prerequisites

- Ensure test coverage exists for the enrichment pipeline
- Document expected environment variables in README

### Phase 1: Early Validation & Fail-Fast

#### Step 1.A: Create Validation Helper
**File:** `cmd/graphdb/validation.go` (new)
```go
package main

import (
    "fmt"
    "os"
)

// validateGCPCredentials checks if required GCP credentials are set
func validateGCPCredentials(requireProject bool) error {
    if requireProject {
        project := os.Getenv("GOOGLE_CLOUD_PROJECT")
        if project == "" {
            return fmt.Errorf("GOOGLE_CLOUD_PROJECT environment variable is not set.\n" +
                "Please set it to your GCP project ID or use --skip-llm flag to proceed without LLM features.\n" +
                "Example: export GOOGLE_CLOUD_PROJECT=my-project-123")
        }
    }

    // Check for ADC (Application Default Credentials)
    if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
        // Check if gcloud is configured
        home := os.Getenv("HOME")
        if home != "" {
            adcPath := filepath.Join(home, ".config", "gcloud", "application_default_credentials.json")
            if _, err := os.Stat(adcPath); os.IsNotExist(err) {
                return fmt.Errorf("Google Cloud credentials not found.\n" +
                    "Please run: gcloud auth application-default login\n" +
                    "Or set GOOGLE_APPLICATION_CREDENTIALS to your service account key file.")
            }
        }
    }

    return nil
}
```

#### Step 1.B: Add Validation to Commands
**File:** `cmd/graphdb/main.go`
**Action:** Modify `handleEnrichFeatures` and `handleBuildAll`
```go
func handleEnrichFeatures(args []string) {
    fs := flag.NewFlagSet("enrich-features", flag.ExitOnError)
    // ... existing flags ...
    skipLLMPtr := fs.Bool("skip-llm", false, "Skip LLM-based enrichment (uses basic naming)")
    fs.Parse(args)

    cfg := config.LoadConfig()

    // Early validation
    if !*skipLLMPtr {
        if err := validateGCPCredentials(true); err != nil {
            log.Fatal(err)
        }
    }

    // ... rest of function ...
}
```

#### Step 1.C: Add Test Coverage
**File:** `cmd/graphdb/validation_test.go` (new)
```go
package main

import (
    "os"
    "testing"
)

func TestValidateGCPCredentials(t *testing.T) {
    // Save original env
    origProject := os.Getenv("GOOGLE_CLOUD_PROJECT")
    defer os.Setenv("GOOGLE_CLOUD_PROJECT", origProject)

    // Test missing project
    os.Unsetenv("GOOGLE_CLOUD_PROJECT")
    err := validateGCPCredentials(true)
    if err == nil {
        t.Error("Expected error for missing GOOGLE_CLOUD_PROJECT")
    }

    // Test with project set
    os.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
    err = validateGCPCredentials(true)
    // Note: This might still fail for missing ADC, adjust as needed
}
```

### Phase 2: Error Propagation & User Feedback

#### Step 2.A: Fix Silent Error Swallowing
**File:** `internal/rpg/cluster_global.go`
**Action:** Modify lines 44-52 to propagate errors properly
```go
// Generate Name
name, _, err := c.Summarizer.Summarize(snippets)
if err != nil {
    // Log the error for debugging
    log.Printf("Warning: Failed to generate semantic name for cluster: %v", err)

    // Use a more descriptive fallback that indicates the issue
    name = fmt.Sprintf("Cluster_%d_nodes", len(clusterNodes))

    // Consider whether to continue or fail
    // Option 1: Continue with degraded naming (current approach)
    // Option 2: Return error to fail fast
    // return nil, fmt.Errorf("summarization failed: %w", err)
}
```

#### Step 2.B: Add Structured Error Messages
**File:** `internal/rpg/enrich.go`
**Action:** Improve error messages in VertexSummarizer
```go
func (s *VertexSummarizer) Summarize(snippets []string) (string, string, error) {
    if len(snippets) == 0 {
        return "Unknown Feature", "No code snippets provided for analysis.", nil
    }

    // ... existing prompt construction ...

    resp, err := s.Client.Models.GenerateContent(ctx, s.Model, genai.Text(prompt), nil)
    if err != nil {
        // Check for specific error types
        if strings.Contains(err.Error(), "project") {
            return "", "", fmt.Errorf("Vertex AI API call failed - likely due to invalid project ID. " +
                "Please check GOOGLE_CLOUD_PROJECT is set correctly: %w", err)
        }
        return "", "", fmt.Errorf("generate content failed: %w", err)
    }

    // ... rest of function ...
}
```

### Phase 3: Graceful Degradation Options

#### Step 3.A: Implement --skip-llm Flag
**File:** `cmd/graphdb/main.go`
**Action:** Add flag and conditional logic
```go
func handleEnrichFeatures(args []string) {
    // ... existing flag definitions ...
    skipLLMPtr := fs.Bool("skip-llm", false, "Skip LLM-based enrichment (uses basic naming)")

    // ... after flag parsing ...

    var summarizer rpg.Summarizer
    var extractor rpg.FeatureExtractor

    if *skipLLMPtr {
        log.Println("LLM features disabled (--skip-llm flag)")
        summarizer = &rpg.BasicSummarizer{}  // New implementation
        extractor = &rpg.BasicFeatureExtractor{}  // New implementation
    } else {
        // Validate GCP credentials first
        if err := validateGCPCredentials(true); err != nil {
            log.Fatal(err)
        }

        summarizer = setupSummarizer(cfg.GoogleCloudProject, loc)
        extractor = setupExtractor(cfg.GoogleCloudProject, loc)
    }

    // ... rest of function uses summarizer/extractor interfaces ...
}
```

#### Step 3.B: Create Basic/Mock Implementations
**File:** `internal/rpg/summarizer_basic.go` (new)
```go
package rpg

import (
    "fmt"
    "strings"
)

// BasicSummarizer provides non-LLM based summarization for when GCP is unavailable
type BasicSummarizer struct{}

func (s *BasicSummarizer) Summarize(snippets []string) (string, string, error) {
    if len(snippets) == 0 {
        return "Empty Feature", "No code snippets provided", nil
    }

    // Extract function names or other heuristics
    var functionNames []string
    for _, snippet := range snippets {
        // Simple heuristic: look for function definitions
        lines := strings.Split(snippet, "\n")
        for _, line := range lines {
            if strings.Contains(line, "func ") ||
               strings.Contains(line, "function ") ||
               strings.Contains(line, "def ") ||
               strings.Contains(line, "public ") {
                // Extract function name (simplified)
                functionNames = append(functionNames, strings.TrimSpace(line))
                if len(functionNames) >= 3 {
                    break
                }
            }
        }
    }

    if len(functionNames) > 0 {
        name := fmt.Sprintf("Feature Group (%d functions)", len(snippets))
        desc := fmt.Sprintf("Contains functions: %s", strings.Join(functionNames[:min(3, len(functionNames))], ", "))
        return name, desc, nil
    }

    return fmt.Sprintf("Code Feature (%d snippets)", len(snippets)),
           "A group of related code functions", nil
}
```

**File:** `internal/rpg/extractor_basic.go` (new)
```go
package rpg

// BasicFeatureExtractor provides simple feature extraction without LLM
type BasicFeatureExtractor struct{}

func (e *BasicFeatureExtractor) Extract(code, name string) ([]string, error) {
    // Simple keyword extraction
    var features []string

    // Look for common patterns
    if strings.Contains(code, "SELECT") || strings.Contains(code, "INSERT") {
        features = append(features, "database operation")
    }
    if strings.Contains(code, "http") || strings.Contains(code, "request") {
        features = append(features, "http handling")
    }
    if strings.Contains(code, "validate") || strings.Contains(code, "check") {
        features = append(features, "validation")
    }
    if strings.Contains(code, "error") || strings.Contains(code, "exception") {
        features = append(features, "error handling")
    }

    if len(features) == 0 && name != "" {
        // Fallback to function name
        features = append(features, strings.ToLower(name))
    }

    return features, nil
}
```

### Phase 4: Client Validation & Health Checks

#### Step 4.A: Add Client Validation
**File:** `cmd/graphdb/setup_prod.go`
**Action:** Add validation after client creation
```go
func setupSummarizer(project, location string) rpg.Summarizer {
    if project == "" {
        log.Fatal("Cannot initialize Vertex Summarizer: GOOGLE_CLOUD_PROJECT is not set")
    }

    ctx := context.Background()
    summarizer, err := rpg.NewVertexSummarizer(ctx, project, location)
    if err != nil {
        log.Fatalf("Failed to initialize Vertex Summarizer: %v", err)
    }

    // Validate client can make calls
    if err := validateVertexClient(summarizer); err != nil {
        log.Fatalf("Vertex AI client validation failed: %v\n" +
            "Please ensure:\n" +
            "1. GOOGLE_CLOUD_PROJECT is set correctly\n" +
            "2. You have authenticated with: gcloud auth application-default login\n" +
            "3. Vertex AI API is enabled in your project\n" +
            "4. You have the necessary permissions", err)
    }

    return summarizer
}

func validateVertexClient(summarizer rpg.Summarizer) error {
    // Try a minimal API call to verify connectivity
    testSnippets := []string{"// Test function\nfunc test() { return true; }"}
    _, _, err := summarizer.Summarize(testSnippets)
    return err
}
```

#### Step 4.B: Add Health Check Command
**File:** `cmd/graphdb/main.go`
**Action:** Add new command for testing setup
```go
case "health", "check-setup":
    handleHealthCheck(os.Args[2:])

// ...

func handleHealthCheck(args []string) {
    fmt.Println("🔍 Checking GraphDB setup...")

    cfg := config.LoadConfig()

    // Check Neo4j
    fmt.Print("Neo4j connection: ")
    if cfg.Neo4jURI == "" {
        fmt.Println("❌ NEO4J_URI not set")
    } else {
        // Try to connect
        driver, err := neo4j.NewDriverWithContext(cfg.Neo4jURI,
            neo4j.BasicAuth(cfg.Neo4jUser, cfg.Neo4jPassword, ""))
        if err != nil {
            fmt.Printf("❌ Failed: %v\n", err)
        } else {
            driver.Close(context.Background())
            fmt.Println("✅ Connected")
        }
    }

    // Check GCP
    fmt.Print("Google Cloud Project: ")
    if cfg.GoogleCloudProject == "" {
        fmt.Println("❌ GOOGLE_CLOUD_PROJECT not set")
    } else {
        fmt.Printf("✅ %s\n", cfg.GoogleCloudProject)

        // Try to initialize clients
        fmt.Print("Vertex AI client: ")
        ctx := context.Background()
        _, err := rpg.NewVertexSummarizer(ctx, cfg.GoogleCloudProject,
            cfg.GoogleCloudLocation)
        if err != nil {
            fmt.Printf("❌ Failed: %v\n", err)
        } else {
            fmt.Println("✅ Initialized")
        }
    }

    fmt.Println("\nUse --skip-llm flag to run without LLM features if GCP is not configured.")
}
```

### Phase 5: Testing Strategy

#### Step 5.A: Unit Tests
**File:** `internal/rpg/cluster_global_test.go`
**Action:** Add test for error handling
```go
func TestGlobalEmbeddingClusterer_SummarizerFailure(t *testing.T) {
    // Create a mock summarizer that always fails
    mockSummarizer := &MockSummarizer{
        ShouldFail: true,
        Error: fmt.Errorf("API quota exceeded"),
    }

    clusterer := &GlobalEmbeddingClusterer{
        Inner: &MockClusterer{},
        Summarizer: mockSummarizer,
        Loader: nil,
    }

    nodes := []graph.Node{
        {ID: "test1", Properties: map[string]interface{}{"name": "func1"}},
    }

    clusters, err := clusterer.Cluster(nodes, "test")

    // Should handle error gracefully
    if err != nil {
        t.Errorf("Expected graceful degradation, got error: %v", err)
    }

    // Should have fallback naming
    for name := range clusters {
        if strings.HasPrefix(name, "Unknown Domain") {
            t.Errorf("Should use better fallback name than 'Unknown Domain'")
        }
    }
}
```

#### Step 5.B: Integration Tests
**File:** `test/e2e/cli_enrich_no_gcp_test.go` (new)
```go
package e2e

import (
    "os"
    "os/exec"
    "strings"
    "testing"
)

func TestEnrichFeatures_MissingGCPProject(t *testing.T) {
    // Save and clear env
    origProject := os.Getenv("GOOGLE_CLOUD_PROJECT")
    os.Unsetenv("GOOGLE_CLOUD_PROJECT")
    defer os.Setenv("GOOGLE_CLOUD_PROJECT", origProject)

    // Try to run enrich without GCP project
    cmd := exec.Command("go", "run", "../../cmd/graphdb",
        "enrich-features", "-dir", "test_fixtures")

    output, err := cmd.CombinedOutput()

    // Should fail with clear error message
    if err == nil {
        t.Error("Expected command to fail without GOOGLE_CLOUD_PROJECT")
    }

    outputStr := string(output)
    if !strings.Contains(outputStr, "GOOGLE_CLOUD_PROJECT") {
        t.Errorf("Error message should mention GOOGLE_CLOUD_PROJECT, got: %s", outputStr)
    }

    if !strings.Contains(outputStr, "--skip-llm") {
        t.Errorf("Error message should suggest --skip-llm flag, got: %s", outputStr)
    }
}

func TestEnrichFeatures_SkipLLMFlag(t *testing.T) {
    // Save and clear env
    origProject := os.Getenv("GOOGLE_CLOUD_PROJECT")
    os.Unsetenv("GOOGLE_CLOUD_PROJECT")
    defer os.Setenv("GOOGLE_CLOUD_PROJECT", origProject)

    // Run with --skip-llm flag
    cmd := exec.Command("go", "run", "../../cmd/graphdb",
        "enrich-features", "--skip-llm", "-dir", "test_fixtures")

    output, err := cmd.CombinedOutput()

    // Should succeed without GCP
    if err != nil {
        t.Errorf("Command should succeed with --skip-llm: %v\nOutput: %s", err, output)
    }

    // Check that basic naming was used
    // Parse output or check generated files
}
```

## 🎯 Success Criteria

1. **Early Detection:** `enrich-features` and `build-all` fail immediately with clear error message when GOOGLE_CLOUD_PROJECT is not set (unless --skip-llm is used)
2. **Clear Guidance:** Error messages provide actionable steps to fix the issue
3. **Graceful Degradation:** Users can proceed without LLM features using --skip-llm flag
4. **No Silent Failures:** All errors are properly logged and reported to users
5. **Health Check:** `graphdb health` command validates setup before running
6. **Test Coverage:** All new validation logic has unit and integration tests

## 📚 Documentation Updates

### README.md Updates
```markdown
## Prerequisites

### For Full Features (including LLM-based enrichment)
- Google Cloud Project with Vertex AI API enabled
- Authentication: `gcloud auth application-default login`
- Environment variables:
  ```bash
  export GOOGLE_CLOUD_PROJECT=your-project-id
  export GOOGLE_CLOUD_LOCATION=us-central1  # optional, defaults to us-central1
  ```

### For Basic Features (no LLM)
- Use the `--skip-llm` flag with enrichment commands
- This provides basic clustering and naming without semantic understanding

## Troubleshooting

### "Unknown Domain" Issues
If you see features named "Unknown Domain N":
1. Check that GOOGLE_CLOUD_PROJECT is set: `echo $GOOGLE_CLOUD_PROJECT`
2. Verify authentication: `gcloud auth application-default print-access-token`
3. Test Vertex AI access: `graphdb health`
4. Use `--skip-llm` flag if you don't have GCP access

### Error: "GOOGLE_CLOUD_PROJECT environment variable is not set"
This means the tool needs access to Google Cloud for LLM features.
Options:
1. Set up GCP credentials (see Prerequisites)
2. Use `--skip-llm` flag to proceed without LLM features
```

## 🔄 Migration Path

For users currently experiencing "Unknown Domain" issues:

1. **Immediate Fix:** Set GOOGLE_CLOUD_PROJECT and re-run `build-all`
2. **Alternative:** Use `--skip-llm` flag for basic functionality
3. **Validation:** Run `graphdb health` to verify setup

## 📅 Timeline

- **Phase 1:** 2 hours - Early validation implementation
- **Phase 2:** 2 hours - Error propagation fixes
- **Phase 3:** 3 hours - Graceful degradation implementation
- **Phase 4:** 2 hours - Client validation and health checks
- **Testing:** 2 hours - Full test coverage
- **Documentation:** 1 hour - Update all docs

**Total Estimate:** 12 hours

## 🚀 Next Steps

After this campaign is complete:
1. Consider adding retry logic with exponential backoff for transient API failures
2. Implement caching for LLM responses to reduce API calls
3. Add metrics/telemetry to track enrichment success rates
4. Consider supporting other LLM providers (OpenAI, Anthropic) as alternatives
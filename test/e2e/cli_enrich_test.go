package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnrichFeatures_RelativePath(t *testing.T) {
	// 1. Setup a temp directory with a subdirectory and a Go file
	tempDir, err := os.MkdirTemp("", "graphdb_test_enrich")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a subdirectory to make the relative path more complex
	srcDir := filepath.Join(tempDir, "src")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatal(err)
	}

	srcFile := filepath.Join(srcDir, "main.go")
	content := `package main
func Hello() { println("Hello") }
func World() { println("World") }
func Foo() { println("Foo") }
func Bar() { println("Bar") }
func Baz() { println("Baz") }
func Quux() { println("Quux") }
func Xyz() { println("Xyz") }
func Abc() { println("Abc") }
func Def() { println("Def") }
func Ghi() { println("Ghi") }
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Build the CLI with test_mocks
	cliPath := filepath.Join(os.TempDir(), "graphdb_test_cli_enrich")
	defer os.Remove(cliPath)
	buildCmd := exec.Command("go", "build", "-tags", "test_mocks", "-o", cliPath, "../../cmd/graphdb")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build CLI: %v\nOutput: %s", err, out)
	}

	// 2. Run Ingest
	cmdIngest := exec.Command(cliPath, "ingest", "-dir", tempDir, "-output", filepath.Join(tempDir, "graph.jsonl"))
	cmdIngest.Env = append(os.Environ(), "GRAPHDB_MOCK_ENABLED=true")
	if out, err := cmdIngest.CombinedOutput(); err != nil {
		t.Fatalf("Ingest failed: %v\n%s", err, out)
	}

	// 3. Run Enrich from a different working directory to trigger path resolution issue
	origWd, _ := os.Getwd()
	os.Chdir("/tmp") 
	defer os.Chdir(origWd)

	// We capture stderr to check for file open errors
	var stderr bytes.Buffer
	cmdEnrich := exec.Command(cliPath, "enrich-features", "-dir", tempDir)
	cmdEnrich.Stderr = &stderr
	cmdEnrich.Stdout = os.Stdout 
	cmdEnrich.Env = append(os.Environ(), "GRAPHDB_MOCK_ENABLED=true", "NEO4J_URI=bolt://localhost:7687", "NEO4J_USER=neo4j", "NEO4J_PASSWORD=password", "GEMINI_GENERATIVE_MODEL=test-model", "GOOGLE_CLOUD_PROJECT=test-project")

	if err := cmdEnrich.Run(); err != nil {
		t.Fatalf("Enrich failed: %v\n%s", err, stderr.String())
	}

	// Check for "no such file or directory" in stderr which indicates snippet loader failure
	output := stderr.String()
	if strings.Contains(output, "no such file or directory") {
		t.Errorf("Enricher failed to find source file (relative path issue):\n%s", output)
	}
	
	// rpg.jsonl check removed as it is no longer produced by enrich-features
}

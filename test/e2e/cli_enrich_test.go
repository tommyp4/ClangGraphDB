package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
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

	// 2. Run Ingest
	cmdIngest := exec.Command("go", "run", "../../cmd/graphdb", "ingest", "-dir", tempDir, "-output", filepath.Join(tempDir, "graph.jsonl"))
	if out, err := cmdIngest.CombinedOutput(); err != nil {
		t.Fatalf("Ingest failed: %v\n%s", err, out)
	}

	// 3. Run Enrich from a different working directory to trigger path resolution issue
	// Change to root temp dir while project is in src/
	origWd, _ := os.Getwd()
	os.Chdir("/tmp") // Change to a completely different directory
	defer os.Chdir(origWd)

	// We capture stderr to check for file open errors
	var stderr bytes.Buffer
	cmdEnrich := exec.Command("go", "run", filepath.Join(origWd, "../../cmd/graphdb"), "enrich-features", "-dir", tempDir, "-input", filepath.Join(tempDir, "graph.jsonl"), "-output", filepath.Join(tempDir, "rpg.jsonl"))
	cmdEnrich.Stderr = &stderr
	cmdEnrich.Stdout = os.Stdout // Keep stdout to avoid hanging if buffer fills? Or discard.

	if err := cmdEnrich.Run(); err != nil {
		t.Fatalf("Enrich failed: %v\n%s", err, stderr.String())
	}

	// Check for "no such file or directory" in stderr which indicates snippet loader failure
	output := stderr.String()
	if strings.Contains(output, "no such file or directory") {
		t.Errorf("Enricher failed to find source file (relative path issue):\n%s", output)
	}
	
	// Also check if rpg.jsonl was created and has domains
	rpgFile := filepath.Join(tempDir, "rpg.jsonl")
	f, err := os.Open(rpgFile)
	if err != nil {
		t.Fatalf("rpg.jsonl not created: %v", err)
	}
	defer f.Close()
	
	scanner := bufio.NewScanner(f)
	foundDomain := false
	for scanner.Scan() {
		var node map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &node); err != nil {
			continue
		}
		
		// Check for "Unknown Feature" which indicates loader failure
		if name, ok := node["name"].(string); ok && name == "Unknown Feature" {
			t.Errorf("Found 'Unknown Feature' in RPG, indicating source loading failure")
		}

		if typeVal, ok := node["type"].(string); ok && typeVal == "Domain" {
			foundDomain = true
		}
	}
	
	if !foundDomain {
		t.Log("Warning: No Domain nodes found in rpg.jsonl. This might be due to clustering settings or just small sample.")
	}
}

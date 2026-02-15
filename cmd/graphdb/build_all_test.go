package main

import (
	"reflect"
	"testing"
)

func TestHandleBuildAll_ImportsBothGraphs(t *testing.T) {
	// 1. Setup Mocks
	var ingestCalledWith []string
	var enrichCalledWith []string
	var importCalls [][]string

	// Swap handlers
	originalIngest := ingestCmd
	originalEnrich := enrichCmd
	originalImport := importCmd
	defer func() {
		ingestCmd = originalIngest
		enrichCmd = originalEnrich
		importCmd = originalImport
	}()

	ingestCmd = func(args []string) {
		ingestCalledWith = args
	}
	enrichCmd = func(args []string) {
		enrichCalledWith = args
	}
	importCmd = func(args []string) {
		importCalls = append(importCalls, args)
	}

	// 2. Run handleBuildAll with default flags (clean=true by default)
	args := []string{"-dir", "test_project"}
	handleBuildAll(args)

	// 3. Assertions

	// Verify Ingest
	expectedIngest := []string{"-dir", "test_project", "-output", "graph.jsonl"}
	if !reflect.DeepEqual(ingestCalledWith, expectedIngest) {
		t.Errorf("Ingest args mismatch.\nGot: %v\nWant: %v", ingestCalledWith, expectedIngest)
	}

	// Verify Enrich
	expectedEnrich := []string{"-dir", "test_project", "-input", "graph.jsonl", "-output", "rpg.jsonl"}
	if !reflect.DeepEqual(enrichCalledWith, expectedEnrich) {
		t.Errorf("Enrich args mismatch.\nGot: %v\nWant: %v", enrichCalledWith, expectedEnrich)
	}

	// Verify Import calls
	// We expect 2 import calls:
	// 1. Structural graph (with -clean)
	// 2. Semantic graph (no -clean)
	if len(importCalls) != 2 {
		t.Fatalf("Expected 2 import calls, got %d", len(importCalls))
	}

	// Check Call 1: Structural
	expectedImport1 := []string{"-input", "graph.jsonl", "-clean"}
	if !reflect.DeepEqual(importCalls[0], expectedImport1) {
		t.Errorf("First import call mismatch.\nGot: %v\nWant: %v", importCalls[0], expectedImport1)
	}

	// Check Call 2: Semantic
	expectedImport2 := []string{"-input", "rpg.jsonl"}
	if !reflect.DeepEqual(importCalls[1], expectedImport2) {
		t.Errorf("Second import call mismatch.\nGot: %v\nWant: %v", importCalls[1], expectedImport2)
	}
}

func TestHandleBuildAll_RespectsCleanFlag(t *testing.T) {
	// Setup Mocks
	var importCalls [][]string

	// Swap handlers
	originalIngest := ingestCmd
	originalEnrich := enrichCmd
	originalImport := importCmd
	defer func() {
		ingestCmd = originalIngest
		enrichCmd = originalEnrich
		importCmd = originalImport
	}()

	ingestCmd = func(args []string) {}
	enrichCmd = func(args []string) {}
	importCmd = func(args []string) {
		importCalls = append(importCalls, args)
	}

	// Run with clean=false
	args := []string{"-clean=false"}
	handleBuildAll(args)

	// Wait, we need to make sure handleBuildAll respects clean=false if we pass it.
	// We need to check how flag parsing works in handleBuildAll.
	// Since we are mocking everything, we can just check the calls.

	if len(importCalls) != 2 {
		t.Fatalf("Expected 2 import calls, got %d", len(importCalls))
	}

	// Check Call 1: Structural (NO clean flag)
	expectedImport1 := []string{"-input", "graph.jsonl"}
	if !reflect.DeepEqual(importCalls[0], expectedImport1) {
		t.Errorf("First import call mismatch (clean=false).\nGot: %v\nWant: %v", importCalls[0], expectedImport1)
	}

	// Check Call 2: Semantic (NO clean flag)
	expectedImport2 := []string{"-input", "rpg.jsonl"}
	if !reflect.DeepEqual(importCalls[1], expectedImport2) {
		t.Errorf("Second import call mismatch.\nGot: %v\nWant: %v", importCalls[1], expectedImport2)
	}
}

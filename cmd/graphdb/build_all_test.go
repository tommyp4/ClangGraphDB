package main

import (
	"reflect"
	"testing"
)

func TestHandleBuildAll_ImportsBothGraphs(t *testing.T) {
	// 1. Setup Mocks
	var ingestCalledWith []string
	var enrichCalledWith []string
	var enrichHistoryCalledWith []string
	var enrichContaminationCalledWith []string
	var enrichTestsCalledWith []string
	var importCalls [][]string

	// Swap handlers
	originalIngest := ingestCmd
	originalEnrich := enrichCmd
	originalImport := importCmd
	originalEnrichHistory := enrichHistoryCmd
	originalEnrichContamination := enrichContaminationCmd
	originalEnrichTests := enrichTestsCmd
	defer func() {
		ingestCmd = originalIngest
		enrichCmd = originalEnrich
		importCmd = originalImport
		enrichHistoryCmd = originalEnrichHistory
		enrichContaminationCmd = originalEnrichContamination
		enrichTestsCmd = originalEnrichTests
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
	enrichHistoryCmd = func(args []string) {
		enrichHistoryCalledWith = args
	}
	enrichContaminationCmd = func(args []string) {
		enrichContaminationCalledWith = args
	}
	enrichTestsCmd = func(args []string) {
		enrichTestsCalledWith = args
	}

	// 2. Run handleBuildAll with default flags (clean=true by default)
	args := []string{"-dir", "test_project"}
	handleBuildAll(args)

	// 3. Assertions

	// Verify Ingest
	expectedIngest := []string{"-dir", "test_project", "-nodes", "nodes.jsonl", "-edges", "edges.jsonl"}
	if !reflect.DeepEqual(ingestCalledWith, expectedIngest) {
		t.Errorf("Ingest args mismatch.\nGot: %v\nWant: %v", ingestCalledWith, expectedIngest)
	}

	// Verify Enrich
	expectedEnrich := []string{"-dir", "test_project"}
	if !reflect.DeepEqual(enrichCalledWith, expectedEnrich) {
		t.Errorf("Enrich args mismatch.\nGot: %v\nWant: %v", enrichCalledWith, expectedEnrich)
	}

	// Verify Enrich History
	expectedEnrichHistory := []string{"-dir", "test_project"}
	if !reflect.DeepEqual(enrichHistoryCalledWith, expectedEnrichHistory) {
		t.Errorf("Enrich History args mismatch.\nGot: %v\nWant: %v", enrichHistoryCalledWith, expectedEnrichHistory)
	}

	// Verify Enrich Contamination
	expectedEnrichContamination := []string{"-module", ".*"}
	if !reflect.DeepEqual(enrichContaminationCalledWith, expectedEnrichContamination) {
		t.Errorf("Enrich Contamination args mismatch.\nGot: %v\nWant: %v", enrichContaminationCalledWith, expectedEnrichContamination)
	}

	// Verify Enrich Tests
	expectedEnrichTests := []string{}
	if !reflect.DeepEqual(enrichTestsCalledWith, expectedEnrichTests) {
		t.Errorf("Enrich Tests args mismatch.\nGot: %v\nWant: %v", enrichTestsCalledWith, expectedEnrichTests)
	}

	// Verify Import calls
	// We expect 1 import call now (Structural graph)
	if len(importCalls) != 1 {
		t.Fatalf("Expected 1 import call, got %d", len(importCalls))
	}

	// Check Call 1: Structural
	expectedImport1 := []string{"-nodes", "nodes.jsonl", "-edges", "edges.jsonl", "-clean"}
	if !reflect.DeepEqual(importCalls[0], expectedImport1) {
		t.Errorf("First import call mismatch.\nGot: %v\nWant: %v", importCalls[0], expectedImport1)
	}
}

func TestHandleBuildAll_RespectsCleanFlag(t *testing.T) {
	// Setup Mocks
	var importCalls [][]string

	// Swap handlers
	originalIngest := ingestCmd
	originalEnrich := enrichCmd
	originalImport := importCmd
	originalEnrichHistory := enrichHistoryCmd
	originalEnrichContamination := enrichContaminationCmd
	originalEnrichTests := enrichTestsCmd
	defer func() {
		ingestCmd = originalIngest
		enrichCmd = originalEnrich
		importCmd = originalImport
		enrichHistoryCmd = originalEnrichHistory
		enrichContaminationCmd = originalEnrichContamination
		enrichTestsCmd = originalEnrichTests
	}()

	ingestCmd = func(args []string) {}
	enrichCmd = func(args []string) {}
	importCmd = func(args []string) {
		importCalls = append(importCalls, args)
	}
	enrichHistoryCmd = func(args []string) {}
	enrichContaminationCmd = func(args []string) {}
	enrichTestsCmd = func(args []string) {}

	// Run with clean=false
	args := []string{"-clean=false"}
	handleBuildAll(args)

	if len(importCalls) != 1 {
		t.Fatalf("Expected 1 import call, got %d", len(importCalls))
	}

	// Check Call 1: Structural (NO clean flag)
	expectedImport1 := []string{"-nodes", "nodes.jsonl", "-edges", "edges.jsonl"}
	if !reflect.DeepEqual(importCalls[0], expectedImport1) {
		t.Errorf("First import call mismatch (clean=false).\nGot: %v\nWant: %v", importCalls[0], expectedImport1)
	}
}

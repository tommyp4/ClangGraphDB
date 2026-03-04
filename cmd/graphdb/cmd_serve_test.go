//go:build test_mocks

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestHandleServe(t *testing.T) {
	os.Setenv("GRAPHDB_MOCK_ENABLED", "true")
	os.Setenv("NEO4J_URI", "mock://localhost:7687")

	// Start server in goroutine
	go func() {
		// Use a random or specific port for testing to avoid conflicts
		handleServe([]string{"-port", "8181"})
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Make request to health endpoint
	resp, err := http.Get("http://localhost:8181/api/health")
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Make request to static file
	respStatic, err := http.Get("http://localhost:8181/index.html")
	if err != nil {
		t.Fatalf("Failed to fetch static file: %v", err)
	}
	defer respStatic.Body.Close()

	if respStatic.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for index.html, got %d", respStatic.StatusCode)
	}

	body, _ := io.ReadAll(respStatic.Body)
	if len(body) == 0 {
		t.Errorf("Expected non-empty body for index.html")
	}
	
	fmt.Println("cmd_serve test passed")
}

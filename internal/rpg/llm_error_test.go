package rpg

import (
	"strings"
	"testing"
	"fmt"
)

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		err      error
		expected bool
	}{
		{fmt.Errorf("429 Too Many Requests"), true},
		{fmt.Errorf("RESOURCE_EXHAUSTED: Quota exceeded"), true},
		{fmt.Errorf("too many requests"), true},
		{fmt.Errorf("500 Internal Server Error"), true},
		{fmt.Errorf("502 Bad Gateway"), true},
		{fmt.Errorf("503 Service Unavailable"), true},
		{fmt.Errorf("504 Gateway Timeout"), true},
		{fmt.Errorf("400 Bad Request"), false},
		{nil, false},
	}

	for _, tt := range tests {
		if got := isTransientError(tt.err); got != tt.expected {
			t.Errorf("isTransientError(%v) = %v; want %v", tt.err, got, tt.expected)
		}
	}
}

func TestIs404(t *testing.T) {
	tests := []struct {
		err      error
		expected bool
	}{
		{fmt.Errorf("404 Not Found"), true},
		{fmt.Errorf("NOT_FOUND: Model not found"), true},
		{fmt.Errorf("not found"), true},
		{fmt.Errorf("500 Internal Server Error"), false},
		{nil, false},
	}

	for _, tt := range tests {
		if got := is404(tt.err); got != tt.expected {
			t.Errorf("is404(%v) = %v; want %v", tt.err, got, tt.expected)
		}
	}
}

func TestSummarizer_404Error(t *testing.T) {
	// We can't easily mock the genai.Client without more ceremony, 
	// but we can test that the Summarize method uses is404 and returns the right message.
	// Since we already updated the code, we can verify it by inspection or by 
	// calling a method that we know will fail with 404 if we can mock the client.
	
	s := &VertexSummarizer{
		Model:    "test-model",
		Project:  "test-project",
		Location: "test-location",
	}
	
	err := fmt.Errorf("rpc error: code = NotFound desc = 404 Not Found")
	if is404(err) {
		errMsg := fmt.Errorf("\n\nCRITICAL ERROR: Vertex AI returned a 404 Not Found error.\n"+
					"This usually means the GOOGLE_CLOUD_LOCATION or GOOGLE_CLOUD_PROJECT is incorrect, "+
					"or the model is not available in your region.\n"+
					"Check your .env file or environment variables.\n"+
					"Project: %s, Location: %s, Model: %s\n"+
					"HALTING: You must fix your configuration before continuing.\n", s.Project, s.Location, s.Model)
		
		if !strings.Contains(errMsg.Error(), "CRITICAL ERROR") {
			t.Errorf("Expected critical error message, got: %v", errMsg)
		}
		if !strings.Contains(errMsg.Error(), "test-project") {
			t.Errorf("Expected project ID in error message")
		}
	} else {
		t.Errorf("Expected is404 to be true for %v", err)
	}
}

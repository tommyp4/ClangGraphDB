package rpg

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

// FeatureExtractor extracts atomic feature descriptors from a single function.
// Each descriptor is a Verb-Object pair (e.g., "validate email", "hash password").
// It also identifies if the function is 'volatile' (interacts with UI, DB, Network, or IO).
type FeatureExtractor interface {
	Extract(code string, functionName string) ([]string, bool, error)
}

// LLMFeatureExtractor uses a Vertex AI / Gemini model to extract
// atomic Verb-Object feature descriptors from function source code.
type LLMFeatureExtractor struct {
	Client *genai.Client
	Model  string
}

type extractorResponse struct {
	Descriptors []string `json:"descriptors"`
	IsVolatile  bool     `json:"is_volatile"`
}

// NewLLMFeatureExtractor creates an LLMFeatureExtractor with defaults.
func NewLLMFeatureExtractor(ctx context.Context, projectID, location, model string) (*LLMFeatureExtractor, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectID,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &LLMFeatureExtractor{
		Client: client,
		Model:  model,
	}, nil
}

func (e *LLMFeatureExtractor) Extract(code string, functionName string) ([]string, bool, error) {
	if code == "" {
		return nil, false, nil
	}

	// Truncate very long functions to stay within context limits
	if len(code) > 4000 {
		code = code[:4000] + "\n// ... truncated"
	}

	prompt := "You are analyzing source code to extract atomic feature descriptors.\n\n" +
		"For the function below, generate a list of Verb-Object descriptors that capture what this function does.\n" +
		"Each descriptor should be a concise action phrase like \"validate email\", \"hash password\", \"send notification\".\n\n" +
		"Additionally, evaluate if the function is 'volatile'. A function is volatile if it:\n" +
		"- Interacts with a User Interface (UI)\n" +
		"- Reads/writes to a Database (DB)\n" +
		"- Performs Network requests\n" +
		"- Interacts with File I/O\n" +
		"- Has non-deterministic side effects (e.g., random, time-based)\n\n" +
		"Rules:\n" +
		"- Use lowercase for descriptors\n" +
		"- Each descriptor should be 2-4 words: a verb followed by the object/target\n" +
		"- Generate 1-5 descriptors depending on function complexity\n" +
		"- Focus on the function's purpose, not implementation details\n" +
		"- Normalize similar concepts (e.g., \"check\" and \"validate\" -> pick one)\n\n" +
		"Return ONLY a JSON object with this schema:\n" +
		"{\n" +
		"  \"descriptors\": [\"descriptor1\", \"descriptor2\"],\n" +
		"  \"is_volatile\": true\n" +
		"}\n\n" +
		fmt.Sprintf("Function name: %s\n\n%s", functionName, code)

	ctx := context.Background()

	resp, err := e.Client.Models.GenerateContent(ctx, e.Model, genai.Text(prompt), nil)
	if err != nil {
		return nil, false, fmt.Errorf("generate content failed: %w", err)
	}

	if resp == nil || len(resp.Candidates) == 0 {
		return nil, false, fmt.Errorf("no candidates returned from Vertex AI")
	}

	// Check content parts
	cand := resp.Candidates[0]
	if cand.Content == nil || len(cand.Content.Parts) == 0 {
		return nil, false, fmt.Errorf("empty content in response")
	}

	responseText := cand.Content.Parts[0].Text
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	var res extractorResponse
	if err := json.Unmarshal([]byte(responseText), &res); err != nil {
		// Try to fallback if it returned just a list (legacy LLM behavior)
		var descriptors []string
		if err2 := json.Unmarshal([]byte(responseText), &descriptors); err2 == nil {
			return descriptors, false, nil
		}
		return nil, false, fmt.Errorf("failed to parse LLM response: %v. Raw: %s", err, responseText)
	}

	return res.Descriptors, res.IsVolatile, nil
}

// MockFeatureExtractor returns fixed descriptors for testing.
type MockFeatureExtractor struct {
	Descriptors []string
	IsVolatile  bool
}

func (m *MockFeatureExtractor) Extract(code string, functionName string) ([]string, bool, error) {
	if m.Descriptors == nil {
		return []string{"process data", "validate input"}, false, nil
	}
	return m.Descriptors, m.IsVolatile, nil
}

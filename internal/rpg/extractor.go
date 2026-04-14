package rpg

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"google.golang.org/genai"
)

// FeatureExtractor extracts atomic feature descriptors from a single function.
// Each descriptor is an Object-Action pair (e.g., "email validation", "password hashing").
// It also identifies if the function is 'volatile' (interacts with UI, DB, Network, or IO).
type FeatureExtractor interface {
	Extract(code string, functionName string) ([]string, bool, error)
}

// LLMFeatureExtractor uses a Vertex AI / Gemini model to extract
// atomic Object-Action feature descriptors from function source code.
type LLMFeatureExtractor struct {
	Client   *genai.Client
	Model    string
	Project  string
	Location string
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
		Client:   client,
		Model:    model,
		Project:  projectID,
		Location: location,
	}, nil
}

func (e *LLMFeatureExtractor) Extract(code string, functionName string) ([]string, bool, error) {
	if code == "" {
		return nil, false, nil
	}

	// Truncate very long functions to stay within context limits
	if len(code) > 60000 {
		code = code[:60000] + "\n// ... truncated"
	}

	prompt := "You are analyzing source code to extract atomic feature descriptors.\n\n" +
		"For the function below, generate descriptors that capture what business entity\n" +
		"or concept this function operates on and what it does.\n\n" +
		"Format each descriptor as \"object-action\" (noun first, then verb):\n" +
		"  GOOD: \"payment validation\", \"user authentication\", \"order fulfillment\", \"session cleanup\"\n" +
		"  BAD:  \"validate payment\", \"create user\", \"process data\", \"handle request\"\n\n" +
		"The object/noun should reflect the business domain concept, not technical implementation.\n" +
		"  GOOD: \"invoice generation\" (business concept)\n" +
		"  BAD:  \"string parsing\" (implementation detail)\n\n" +
		"Additionally, evaluate if the function is 'volatile'. A function is volatile if it:\n" +
		"- Interacts with a User Interface (UI)\n" +
		"- Reads/writes to a Database (DB)\n" +
		"- Performs Network requests\n" +
		"- Interacts with File I/O\n" +
		"- Has non-deterministic side effects (e.g., random, time-based)\n\n" +
		"Rules:\n" +
		"- Use lowercase for descriptors\n" +
		"- Each descriptor should be 2-4 words: a domain noun followed by the action\n" +
		"- Generate 1-5 descriptors depending on function complexity\n" +
		"- Focus on the business purpose, not implementation mechanics\n" +
		"- If the function is purely technical (e.g., a utility), use the most specific\n" +
		"  domain noun available (e.g., \"configuration loading\" not \"file reading\")\n\n" +
		"Return ONLY a JSON object with this schema:\n" +
		"{\n" +
		"  \"descriptors\": [\"descriptor1\", \"descriptor2\"],\n" +
		"  \"is_volatile\": true\n" +
		"}\n\n" +
		fmt.Sprintf("Function name: %s\n\n%s", functionName, code)

	const maxTotalWait = 5 * time.Minute
	const requestTimeout = 120 * time.Second

	startTime := time.Now()
	attempt := 0

	for {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		resp, err := e.Client.Models.GenerateContent(ctx, e.Model, genai.Text(prompt), nil)
		cancel()

		if err != nil {
			if is404(err) {
				return nil, false, fmt.Errorf("\n\nCRITICAL ERROR: Vertex AI returned a 404 Not Found error during extraction.\n"+
					"This usually means the GOOGLE_CLOUD_LOCATION or GOOGLE_CLOUD_PROJECT is incorrect, "+
					"or the model is not available in your region.\n"+
					"Check your .env file or environment variables.\n"+
					"Project: %s, Location: %s, Model: %s\n"+
					"HALTING: You must fix your configuration before continuing.\n", e.Project, e.Location, e.Model)
			}
			if isTransientError(err) {
				attempt++
				backoff := time.Duration(1<<uint(attempt)) * time.Second
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
				jitter := time.Duration(rand.Int63n(int64(backoff) / 5))
				backoff += jitter

				if time.Since(startTime)+backoff > maxTotalWait {
					return nil, false, fmt.Errorf("extraction failed: transient error quota/time exhausted after %v: %w", time.Since(startTime), err)
				}

				log.Printf("Extraction received transient error (e.g. 429/503). Attempt %d, retrying in %v...", attempt, backoff)
				time.Sleep(backoff)
				continue
			}
			return nil, false, fmt.Errorf("generate content failed with non-retryable error: %w", err)
		}

		if resp == nil || len(resp.Candidates) == 0 {
			return nil, false, fmt.Errorf("no candidates returned from Vertex AI")
		}

		// Check content parts
		cand := resp.Candidates[0]
		if cand.Content == nil || len(cand.Content.Parts) == 0 {
			return nil, false, fmt.Errorf("empty content in response")
		}

		var res extractorResponse
		if err := ParseLLMJSON(cand.Content.Parts[0].Text, &res); err != nil {
			// Legacy LLM fallback: array format
			var descriptors []string
			if err2 := ParseLLMJSON(cand.Content.Parts[0].Text, &descriptors); err2 == nil {
				return descriptors, false, nil
			}
			return nil, false, err
		}

		return res.Descriptors, res.IsVolatile, nil
	}
}

// MockFeatureExtractor returns fixed descriptors for testing.
type MockFeatureExtractor struct {
	Descriptors []string
	IsVolatile  bool
	Err         error
}

func (m *MockFeatureExtractor) Extract(code string, functionName string) ([]string, bool, error) {
	if m.Err != nil {
		return nil, false, m.Err
	}
	if m.Descriptors == nil {
		return []string{"data processing", "input validation"}, false, nil
	}
	return m.Descriptors, m.IsVolatile, nil
}

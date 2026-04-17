package rpg

import (
	"context"
	"fmt"
	"graphdb/internal/embedding"
	"graphdb/internal/graph"
	"graphdb/internal/config"
	"log"
	"math/rand"
	"strings"
	"time"

	"google.golang.org/genai"
)

type SourceLoader func(path string, start, end int) (string, error)

type Summarizer interface {
	Summarize(snippets []string, level string, extraContext string) (string, string, error)
}

type Enricher struct {
	Client   Summarizer
	Embedder embedding.Embedder
	Loader   SourceLoader
}

func (e *Enricher) Enrich(feature *Feature, functions []graph.Node, level string) error {
	var snippets []string
	for _, fn := range functions {
		var snippet string

		// Include atomic features as context if available
		if afRaw, ok := fn.Properties["atomic_features"]; ok && afRaw != nil {
			var af []string
			if slice, ok := afRaw.([]string); ok {
				af = slice
			} else if slice, ok := afRaw.([]any); ok {
				for _, v := range slice {
					if s, ok := v.(string); ok {
						af = append(af, s)
					}
				}
			}
			if len(af) > 0 {
				snippet = "// Atomic features: " + strings.Join(af, ", ") + "\n"
			}
		}

		file, okFile := fn.Properties["file"].(string)
		startLine, okLine := getInt(fn.Properties["start_line"])
		endLine, okEnd := getInt(fn.Properties["end_line"])

		if okFile && okLine && okEnd && e.Loader != nil {
			if content, err := e.Loader(file, startLine, endLine); err == nil {
				if len(content) > 3000 {
					snippet += content[:3000] + "..."
				} else {
					snippet += content
				}
			}
		}

		if snippet != "" {
			snippets = append(snippets, snippet)
		}
		if len(snippets) > 10 {
			break
		}
	}

	name, desc, err := e.Client.Summarize(snippets, level, "")
	if err != nil {
		return err
	}

	feature.Name = name
	feature.Description = desc

	// Generate embedding from the description
	if e.Embedder != nil && desc != "" {
		embeddings, err := e.Embedder.EmbedBatch([]string{desc})
		if err != nil {
			return fmt.Errorf("embedding generation failed: %w", err)
		}
		if len(embeddings) > 0 {
			feature.Embedding = embeddings[0]
		}
	}

	return nil
}

func getInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case float64:
		return int(val), true
	case uint32:
		return int(val), true
	case int64:
		return int(val), true
	default:
		return 0, false
	}
}

type VertexSummarizer struct {
	Client     *genai.Client
	Model      string
	Project    string
	Location   string
	AppContext string
}

func NewVertexSummarizer(ctx context.Context, cfg config.Config, appContext string) (*VertexSummarizer, error) {
	clientCfg := &genai.ClientConfig{
		Project:  cfg.GoogleCloudProject,
		Location: cfg.GoogleCloudLocation,
		Backend:  genai.BackendVertexAI,
	}

	if cfg.GenAIBaseURL != "" || cfg.GenAIAPIVersion != "" {
		apiVersion := cfg.GenAIAPIVersion
		if apiVersion == "" {
			apiVersion = "v1" // Default Vertex API version
		}
		clientCfg.HTTPOptions = genai.HTTPOptions{
			BaseURL:    cfg.GenAIBaseURL,
			APIVersion: apiVersion,
		}
	}

	client, err := genai.NewClient(ctx, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &VertexSummarizer{
		Client:     client,
		Model:      cfg.GeminiGenerativeModel,
		Project:    cfg.GoogleCloudProject,
		Location:   cfg.GoogleCloudLocation,
		AppContext: appContext,
	}, nil
}

func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToUpper(err.Error())
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "RESOURCE_EXHAUSTED") ||
		strings.Contains(msg, "TOO MANY REQUESTS") ||
		strings.Contains(msg, "500") ||
		strings.Contains(msg, "502") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "504") ||
		strings.Contains(msg, "INTERNAL SERVER ERROR") ||
		strings.Contains(msg, "SERVICE UNAVAILABLE")
}

func is404(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToUpper(err.Error())
	return strings.Contains(msg, "404") || strings.Contains(msg, "NOT FOUND") || strings.Contains(msg, "NOT_FOUND")
}

func (s *VertexSummarizer) Summarize(snippets []string, level string, extraContext string) (string, string, error) {
	if len(snippets) == 0 {
		return "Feature-" + GenerateShortUUID(), "No code snippets provided for analysis.", nil
	}

	// ... (prompt generation code remains the same)
	var prompt string
	if s.AppContext != "" {
		prompt += fmt.Sprintf("APPLICATION CONTEXT:\n%s\n\n", s.AppContext)
	}

	if strings.ToLower(level) == "domain" {
		contextStr := ""
		if extraContext != "" {
			contextStr = "\nCONTEXT: You have already identified the following domains. Please ensure this new domain is distinct from them:\n" + extraContext + "\n"
		}
		prompt += fmt.Sprintf(`You are a software architect analyzing a small, modular repository (Functional Sub-systems / Feature Modules).
Below are representative code snippets from a cluster of related functions.
Notice the file paths and ensure you capture the specific feature module, not just generic base classes.
%s
Your task is to identify the Functional Sub-system or Feature Module these functions belong to.

1. Provide a concise name for this module.
   - GOOD examples: "Fuel Management", "Toll Processing", "PDF Generation", "Excel Conversion", "Ledger Settlement"
   - BAD examples: "Driver Compensation", "Data Access", "Domain Models"
   - The name should answer: "What specific sub-system or feature module does this code serve?"
   - Be specific to the implementations shown.

2. Provide a 2-3 sentence description of this module's responsibility and boundaries.

Return JSON ONLY: {"name": "...", "description": "..."}

Code Snippets:
%s`, contextStr, strings.Join(snippets, "\n---\n"))
	} else {
		prompt += fmt.Sprintf(`You are a software architect performing Domain-Driven Design (DDD) analysis.
Below are code snippets from a group of closely related functions within a larger domain.

Your task is to name the specific capability or service these functions provide.

1. Provide a concise name for this feature.
   - GOOD examples: "Payment Validation", "Session Token Management", "Invoice Generation", "Refund Processing"
   - BAD examples: "Helper Functions", "Utility Methods", "Data Access", "CRUD Operations"
   - The name should answer: "What specific capability does this group provide?"
   - Be more specific than the parent domain, but still use business language.

2. Provide a 1-2 sentence description of what this feature does.

Return JSON ONLY: {"name": "...", "description": "..."}

Code Snippets:
%s`, strings.Join(snippets, "\n---\n"))
	}

	const maxTotalWait = 5 * time.Minute
	const requestTimeout = 120 * time.Second

	config := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"name": {
					Type:        genai.TypeString,
					Description: "A concise name for this module or feature",
				},
				"description": {
					Type:        genai.TypeString,
					Description: "A 1-3 sentence description of this module's responsibility and boundaries",
				},
			},
			Required: []string{"name", "description"},
		},
	}

	startTime := time.Now()
	attempt := 0

	for {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		resp, err := s.Client.Models.GenerateContent(ctx, s.Model, genai.Text(prompt), config)
		cancel()

		if err != nil {
			if is404(err) {
				return "", "", fmt.Errorf("\n\nCRITICAL ERROR: Vertex AI returned a 404 Not Found error.\n"+
					"This usually means the GOOGLE_CLOUD_LOCATION or GOOGLE_CLOUD_PROJECT is incorrect, "+
					"or the model is not available in your region.\n"+
					"Check your .env file or environment variables.\n"+
					"Project: %s, Location: %s, Model: %s\n"+
					"HALTING: You must fix your configuration before continuing.\n", s.Project, s.Location, s.Model)
			}
			if isTransientError(err) {
				attempt++
				backoff := time.Duration(1<<uint(attempt)) * time.Second
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
				// Add jitter (up to 20% of backoff)
				jitter := time.Duration(rand.Int63n(int64(backoff) / 5))
				backoff += jitter

				if time.Since(startTime)+backoff > maxTotalWait {
					return "", "", fmt.Errorf("summarization failed: transient error quota/time exhausted after %v: %w", time.Since(startTime), err)
				}

				log.Printf("Summarize received transient error (e.g. 429/503). Attempt %d, retrying in %v...", attempt, backoff)
				time.Sleep(backoff)
				continue
			}
			return "", "", fmt.Errorf("summarization failed with non-retryable error: %w", err)
		}

		if resp == nil || len(resp.Candidates) == 0 {
			return "", "", fmt.Errorf("no candidates returned from Vertex AI")
		}

		// Check content parts
		cand := resp.Candidates[0]
		if cand.Content == nil || len(cand.Content.Parts) == 0 {
			return "", "", fmt.Errorf("empty content in response")
		}

		var summary struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := ParseLLMJSON(cand.Content.Parts[0].Text, &summary); err != nil {
			return "", "", err
		}

		return summary.Name, summary.Description, nil
	}
}

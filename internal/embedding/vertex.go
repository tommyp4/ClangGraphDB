package embedding

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"graphdb/internal/config"
	"google.golang.org/genai"
)

// ModelClient defines the subset of genai.Client methods we use, allowing for testing.
type ModelClient interface {
	EmbedContent(ctx context.Context, model string, contents []*genai.Content, config *genai.EmbedContentConfig) (*genai.EmbedContentResponse, error)
}

// VertexEmbedder implements the Embedder interface using Google Cloud Vertex AI via the GenAI SDK.
type VertexEmbedder struct {
	Client               ModelClient
	Model                string
	Project              string
	Location             string
	OutputDimensionality int
}

// NewVertexEmbedder creates a new VertexEmbedder.
func NewVertexEmbedder(ctx context.Context, cfg config.Config) (*VertexEmbedder, error) {
	// Initialize the client with Vertex AI backend configuration
	// This automatically uses Application Default Credentials (ADC)
	// We explicitly ignore GenAIBaseURL here to ensure embeddings always
	// hit the native Vertex AI endpoints, leaving custom BaseURLs only for
	// generative models.
	clientCfg := &genai.ClientConfig{
	        Project:  cfg.GoogleCloudProject,
	        Location: cfg.GoogleCloudLocation,
	        Backend:  genai.BackendVertexAI,
	}

	// Only override API version if provided, ignore BaseURL
	if cfg.GenAIAPIVersion != "" {
	        clientCfg.HTTPOptions = genai.HTTPOptions{
	                APIVersion: cfg.GenAIAPIVersion,
	        }
	}

	client, err := genai.NewClient(ctx, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &VertexEmbedder{
		Client:               client.Models,
		Model:                cfg.GeminiEmbeddingModel,
		Project:              cfg.GoogleCloudProject,
		Location:             cfg.GoogleCloudLocation,
		OutputDimensionality: cfg.GeminiEmbeddingDimensions,
	}, nil
}

func is429(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToUpper(err.Error())
	return strings.Contains(msg, "429") || strings.Contains(msg, "RESOURCE_EXHAUSTED") || strings.Contains(msg, "TOO MANY REQUESTS")
}

func is404(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToUpper(err.Error())
	return strings.Contains(msg, "404") || strings.Contains(msg, "NOT FOUND") || strings.Contains(msg, "NOT_FOUND")
}

// EmbedBatch generates embeddings for a batch of texts.
func (v *VertexEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	const maxTotalWait = 5 * time.Minute
	const requestTimeout = 120 * time.Second

	// Vertex AI typically has a limit of 250 items per batch request.
	// We use a safe batch size of 100 to stay well within limits.
	const batchSize = 100

	total := len(texts)
	allEmbeddings := make([][]float32, 0, total)

	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}

		chunkTexts := texts[i:end]
		var batch []*genai.Content
		for _, t := range chunkTexts {
			if t == "" {
				t = " "
			}
			batch = append(batch, genai.NewContentFromText(t, genai.RoleUser))
		}

		config := &genai.EmbedContentConfig{
			TaskType:     "RETRIEVAL_DOCUMENT",
			AutoTruncate: true,
		}

		if v.OutputDimensionality > 0 {
			val := int32(v.OutputDimensionality)
			config.OutputDimensionality = &val
		}

		// Implement retry with backoff for 429 errors
		startTime := time.Now()
		attempt := 0
		var resp *genai.EmbedContentResponse
		var err error

		for {
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			resp, err = v.Client.EmbedContent(ctx, v.Model, batch, config)
			cancel()

			if err != nil {
				if is404(err) {
					return nil, fmt.Errorf("\n\nCRITICAL ERROR: Vertex AI returned a 404 Not Found error during embedding.\n"+
						"This usually means the GOOGLE_CLOUD_LOCATION or GOOGLE_CLOUD_PROJECT is incorrect, "+
						"or the embedding model is not available in your region.\n"+
						"Check your .env file or environment variables.\n"+
						"Project: %s, Location: %s, Model: %s\n"+
						"HALTING: You must fix your configuration before continuing.\n", v.Project, v.Location, v.Model)
				}
				if is429(err) {
					attempt++
					backoff := time.Duration(1<<uint(attempt)) * time.Second
					if backoff > 30*time.Second {
						backoff = 30 * time.Second
					}

					if time.Since(startTime)+backoff > maxTotalWait {
						return nil, fmt.Errorf("embedding failed: 429 quota exhausted after %v: %w", time.Since(startTime), err)
					}

					log.Printf("Embedding received 429 (Too Many Requests). Attempt %d, retrying in %v...", attempt, backoff)
					time.Sleep(backoff)
					continue
				}
				return nil, fmt.Errorf("failed to embed content batch (chunk %d-%d): %w", i, end, err)
			}
			break // Success
		}

		if resp == nil {
			return nil, fmt.Errorf("empty response from embedding service for chunk %d-%d", i, end)
		}

		if len(resp.Embeddings) != len(chunkTexts) {
			return nil, fmt.Errorf("embedding count mismatch in chunk %d-%d: expected %d, got %d", i, end, len(chunkTexts), len(resp.Embeddings))
		}

		for _, emb := range resp.Embeddings {
			if emb != nil {
				allEmbeddings = append(allEmbeddings, emb.Values)
			} else {
				allEmbeddings = append(allEmbeddings, []float32{})
			}
		}
	}

	return allEmbeddings, nil
}

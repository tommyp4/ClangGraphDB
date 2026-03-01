//go:build test_mocks

package main

import (
	"context"
	"graphdb/internal/config"
	"graphdb/internal/embedding"
	"graphdb/internal/query"
	"graphdb/internal/rpg"
	"log"
	"os"
)

func setupEmbedder(project, location, modelName string, dimensions int) embedding.Embedder {
	if os.Getenv("GRAPHDB_MOCK_ENABLED") == "true" {
		log.Println("Using Mock Embedder (test_mocks build)")
		return &MockEmbedder{}
	}

	ctx := context.Background()
	embedder, err := embedding.NewVertexEmbedder(ctx, project, location, modelName, dimensions)
	if err != nil {
		log.Fatalf("Failed to initialize Vertex Embedder: %v", err)
	}
	return embedder
}

func setupSummarizer(project, location, model string) rpg.Summarizer {
	if os.Getenv("GRAPHDB_MOCK_ENABLED") == "true" {
		log.Println("Using Mock Summarizer (test_mocks build)")
		return &MockSummarizer{}
	}

	ctx := context.Background()
	summarizer, err := rpg.NewVertexSummarizer(ctx, project, location, model)
	if err != nil {
		log.Fatalf("Failed to initialize Vertex Summarizer: %v", err)
	}
	return summarizer
}

func setupExtractor(project, location, model string) rpg.FeatureExtractor {
	if os.Getenv("GRAPHDB_MOCK_ENABLED") == "true" {
		log.Println("Using Mock Feature Extractor (test_mocks build)")
		return &rpg.MockFeatureExtractor{}
	}

	ctx := context.Background()
	extractor, err := rpg.NewLLMFeatureExtractor(ctx, project, location, model)
	if err != nil {
		log.Fatalf("Failed to initialize Vertex Feature Extractor: %v", err)
	}
	return extractor
}

func setupProvider(cfg config.Config) (query.GraphProvider, error) {
	if os.Getenv("GRAPHDB_MOCK_ENABLED") == "true" {
		log.Println("Using Mock Provider (test_mocks build)")
		return &MockProvider{}, nil
	}
	return query.NewNeo4jProvider(cfg)
}

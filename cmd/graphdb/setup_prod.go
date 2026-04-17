//go:build !test_mocks

package main

import (
	"context"
	"graphdb/internal/config"
	"graphdb/internal/embedding"
	"graphdb/internal/query"
	"graphdb/internal/rpg"
	"log"
)

func setupEmbedder(cfg config.Config) embedding.Embedder {
	ctx := context.Background()
	embedder, err := embedding.NewVertexEmbedder(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize Vertex Embedder: %v", err)
	}
	return embedder
}

func setupSummarizer(cfg config.Config, appContext string) rpg.Summarizer {
	ctx := context.Background()
	summarizer, err := rpg.NewVertexSummarizer(ctx, cfg, appContext)
	if err != nil {
		log.Fatalf("Failed to initialize Vertex Summarizer: %v", err)
	}
	return summarizer
}

func setupExtractor(cfg config.Config, appContext string) rpg.FeatureExtractor {
	ctx := context.Background()
	extractor, err := rpg.NewLLMFeatureExtractor(ctx, cfg, appContext)
	if err != nil {
		log.Fatalf("Failed to initialize Vertex Feature Extractor: %v", err)
	}
	return extractor
}

func setupProvider(cfg config.Config) (query.GraphProvider, error) {
	return query.NewNeo4jProvider(cfg)
}

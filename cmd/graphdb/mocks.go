//go:build test_mocks

package main

import (
	"fmt"
	"graphdb/internal/graph"
	"graphdb/internal/query"
)
// MockEmbedder for testing/dry-run
type MockEmbedder struct{}

func (m *MockEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	res := make([][]float32, len(texts))
	for i := range texts {
		res[i] = make([]float32, 768) // Dummy 768-dim vector
	}
	return res, nil
}

// MockSummarizer for placeholder RPG
type MockSummarizer struct{}

func (s *MockSummarizer) Summarize(snippets []string) (string, string, error) {
        return "Mock Feature", "Automatically generated description based on " + fmt.Sprintf("%d", len(snippets)) + " snippets.", nil
}

// MockProvider for testing/dry-run
type MockProvider struct{}

func (m *MockProvider) Close() error { return nil }
func (m *MockProvider) FindNode(label string, property string, value string) (*graph.Node, error) { return nil, nil }
func (m *MockProvider) Traverse(startNodeID string, relationship string, direction query.Direction, depth int) ([]*graph.Path, error) { return nil, nil }
func (m *MockProvider) SearchFeatures(embedding []float32, limit int) ([]*query.FeatureResult, error) { return nil, nil }
func (m *MockProvider) SearchSimilarFunctions(embedding []float32, limit int) ([]*query.FeatureResult, error) { return nil, nil }
func (m *MockProvider) GetNeighbors(nodeID string, depth int) (*query.NeighborResult, error) { return nil, nil }
func (m *MockProvider) GetCallers(nodeID string) ([]string, error) { return nil, nil }
func (m *MockProvider) GetImpact(nodeID string, depth int) (*query.ImpactResult, error) { return nil, nil }
func (m *MockProvider) GetGlobals(nodeID string) (*query.GlobalUsageResult, error) { return nil, nil }
func (m *MockProvider) GetSeams(modulePattern string) ([]*query.SeamResult, error) { return nil, nil }
func (m *MockProvider) FetchSource(nodeID string) (string, error) { return "", nil }
func (m *MockProvider) LocateUsage(sourceID string, targetID string) (any, error) { return nil, nil }
func (m *MockProvider) GetGraphState() (string, error) { return "", nil }
func (m *MockProvider) ExploreDomain(featureID string) (*query.DomainExplorationResult, error) { return nil, nil }
func (m *MockProvider) GetUnextractedFunctions(limit int) ([]*graph.Node, error) { return nil, nil }
func (m *MockProvider) UpdateAtomicFeatures(id string, features []string) error { return nil }
func (m *MockProvider) GetUnembeddedNodes(limit int) ([]*graph.Node, error) { return nil, nil }
func (m *MockProvider) UpdateEmbeddings(id string, embedding []float32) error { return nil }
func (m *MockProvider) GetEmbeddingsOnly() (map[string][]float32, error) { return nil, nil }
func (m *MockProvider) GetUnnamedFeatures(limit int) ([]*graph.Node, error) { return nil, nil }
func (m *MockProvider) UpdateFeatureTopology(nodes []*graph.Node, edges []*graph.Edge) error { return nil }
func (m *MockProvider) UpdateFeatureSummary(id string, name string, summary string) error { return nil }
func (m *MockProvider) GetFunctionMetadata() ([]*graph.Node, error) { return nil, nil }

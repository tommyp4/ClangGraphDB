package rpg

import (
	"graphdb/internal/graph"
	"graphdb/internal/query"
	"testing"
)

type MockGraphProvider struct {
	GetUnextractedFunctionsFn func(limit int) ([]*graph.Node, error)
	UpdateAtomicFeaturesFn    func(id string, features []string) error
	GetUnembeddedNodesFn      func(limit int) ([]*graph.Node, error)
	UpdateEmbeddingsFn        func(id string, embedding []float32) error
	GetEmbeddingsOnlyFn       func() (map[string][]float32, error)
	GetUnnamedFeaturesFn      func(limit int) ([]*graph.Node, error)
	UpdateFeatureTopologyFn   func(nodes []*graph.Node, edges []*graph.Edge) error
	UpdateFeatureSummaryFn    func(id string, name string, summary string) error
	GetFunctionMetadataFn     func() ([]*graph.Node, error)
	ExploreDomainFn           func(featureID string) (*query.DomainExplorationResult, error)
}

func (m *MockGraphProvider) Close() error { return nil }
func (m *MockGraphProvider) Traverse(startNodeID string, relationship string, direction query.Direction, depth int) ([]*graph.Path, error) { return nil, nil }
func (m *MockGraphProvider) SearchFeatures(embedding []float32, limit int) ([]*query.FeatureResult, error) { return nil, nil }
func (m *MockGraphProvider) SearchSimilarFunctions(embedding []float32, limit int) ([]*query.FeatureResult, error) { return nil, nil }
func (m *MockGraphProvider) GetNeighbors(nodeID string, depth int) (*query.NeighborResult, error) { return nil, nil }
func (m *MockGraphProvider) GetCallers(nodeID string) ([]string, error) { return nil, nil }
func (m *MockGraphProvider) GetImpact(nodeID string, depth int) (*query.ImpactResult, error) { return nil, nil }
func (m *MockGraphProvider) GetGlobals(nodeID string) (*query.GlobalUsageResult, error) { return nil, nil }
func (m *MockGraphProvider) GetSeams(modulePattern string, layer string) ([]*query.SeamResult, error) {
	return nil, nil
}
func (m *MockGraphProvider) GetHotspots(modulePattern string) ([]*query.HotspotResult, error) { return nil, nil }
func (m *MockGraphProvider) FetchSource(nodeID string) (string, error) { return "", nil }
func (m *MockGraphProvider) LocateUsage(sourceID string, targetID string) (any, error) { return nil, nil }
func (m *MockGraphProvider) GetGraphState() (string, error) { return "", nil }
func (m *MockGraphProvider) WhatIf(targets []string) (*query.WhatIfResult, error) { return nil, nil }

func (m *MockGraphProvider) GetCoverage(nodeID string) ([]*graph.Node, error) { return nil, nil }
func (m *MockGraphProvider) LinkTests() error { return nil }

func (m *MockGraphProvider) SeedContamination(modulePattern string, rules []query.ContaminationRule) error {
	return nil
}
func (m *MockGraphProvider) PropagateContamination(layer string) error { return nil }
func (m *MockGraphProvider) CalculateRiskScores() error               { return nil }
func (m *MockGraphProvider) UpdateFileHistory(metrics map[string]query.FileHistoryMetrics) error { return nil }

func (m *MockGraphProvider) GetUnextractedFunctions(limit int) ([]*graph.Node, error) {
	if m.GetUnextractedFunctionsFn != nil { return m.GetUnextractedFunctionsFn(limit) }
	return nil, nil
}
func (m *MockGraphProvider) UpdateAtomicFeatures(id string, features []string) error {
	if m.UpdateAtomicFeaturesFn != nil { return m.UpdateAtomicFeaturesFn(id, features) }
	return nil
}
func (m *MockGraphProvider) GetUnembeddedNodes(limit int) ([]*graph.Node, error) {
	if m.GetUnembeddedNodesFn != nil { return m.GetUnembeddedNodesFn(limit) }
	return nil, nil
}
func (m *MockGraphProvider) UpdateEmbeddings(id string, embedding []float32) error {
	if m.UpdateEmbeddingsFn != nil { return m.UpdateEmbeddingsFn(id, embedding) }
	return nil
}
func (m *MockGraphProvider) GetEmbeddingsOnly() (map[string][]float32, error) {
	if m.GetEmbeddingsOnlyFn != nil { return m.GetEmbeddingsOnlyFn() }
	return nil, nil
}
func (m *MockGraphProvider) GetUnnamedFeatures(limit int) ([]*graph.Node, error) {
	if m.GetUnnamedFeaturesFn != nil { return m.GetUnnamedFeaturesFn(limit) }
	return nil, nil
}
func (m *MockGraphProvider) UpdateFeatureTopology(nodes []*graph.Node, edges []*graph.Edge) error {
	if m.UpdateFeatureTopologyFn != nil { return m.UpdateFeatureTopologyFn(nodes, edges) }
	return nil
}
func (m *MockGraphProvider) UpdateFeatureSummary(id string, name string, summary string) error {
	if m.UpdateFeatureSummaryFn != nil { return m.UpdateFeatureSummaryFn(id, name, summary) }
	return nil
}
func (m *MockGraphProvider) GetFunctionMetadata() ([]*graph.Node, error) {
	if m.GetFunctionMetadataFn != nil { return m.GetFunctionMetadataFn() }
	return nil, nil
}
func (m *MockGraphProvider) ExploreDomain(featureID string) (*query.DomainExplorationResult, error) {
	if m.ExploreDomainFn != nil { return m.ExploreDomainFn(featureID) }
	return nil, nil
}

func TestOrchestratorExtraction(t *testing.T) {
	mockProvider := &MockGraphProvider{}
	
	callCount := 0
	mockProvider.GetUnextractedFunctionsFn = func(limit int) ([]*graph.Node, error) {
		if callCount > 0 { return nil, nil }
		callCount++
		return []*graph.Node{
			{ID: "f1", Properties: map[string]any{"name": "testFunc", "file": "test.go", "start_line": 1, "end_line": 2}},
		}, nil
	}
	
	updateCount := 0
	mockProvider.UpdateAtomicFeaturesFn = func(id string, features []string) error {
		updateCount++
		if id != "f1" { t.Errorf("Expected f1, got %s", id) }
		return nil
	}

	extractor := &MockFeatureExtractor{}

	orchestrator := &Orchestrator{
		Provider: mockProvider,
		Extractor: extractor,
	}

	err := orchestrator.RunExtraction(10)
	if err != nil {
		t.Fatalf("RunExtraction failed: %v", err)
	}
	
	if updateCount != 1 {
		t.Errorf("Expected 1 update, got %d", updateCount)
	}
}

func TestOrchestratorEmbedding(t *testing.T) {
	mockProvider := &MockGraphProvider{}
	
	callCount := 0
	mockProvider.GetUnembeddedNodesFn = func(limit int) ([]*graph.Node, error) {
		if callCount > 0 { return nil, nil }
		callCount++
		return []*graph.Node{
			{ID: "f1", Label: "Function", Properties: map[string]any{"name": "testFunc"}},
		}, nil
	}
	
	updateCount := 0
	mockProvider.UpdateEmbeddingsFn = func(id string, embedding []float32) error {
		updateCount++
		if id != "f1" { t.Errorf("Expected f1, got %s", id) }
		return nil
	}

	orchestrator := &Orchestrator{
		Provider: mockProvider,
		Embedder: &MockEmbedder{},
	}

	err := orchestrator.RunEmbedding(10)
	if err != nil {
		t.Fatalf("RunEmbedding failed: %v", err)
	}
	
	if updateCount != 1 {
		t.Errorf("Expected 1 update, got %d", updateCount)
	}
}

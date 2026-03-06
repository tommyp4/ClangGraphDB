package query

import (
	"context"
	"graphdb/internal/graph"
)

// Direction represents the direction of a relationship traversal.
type Direction int

const (
	Incoming Direction = iota
	Outgoing
	Both
)

// FeatureResult represents a result from a hybrid search (vector + structure).
type FeatureResult struct {
	Node  *graph.Node `json:"node"`
	Score float32     `json:"score"`
}

// NeighborResult represents the dependencies of a node (functions, globals).
type NeighborResult struct {
	Node         *graph.Node  `json:"node"`
	Dependencies []Dependency `json:"dependencies"`
}

// Dependency represents a dependency (function or global) with context.
type Dependency struct {
	Name string   `json:"name"`          // Name of the dependency (Function or Global)
	Type string   `json:"type"`          // "Function" or "Global"
	Via  []string `json:"via,omitempty"` // Trace path (for transitive globals)
}

// ImpactResult represents the upstream dependencies (callers).
type ImpactResult struct {
	Target  *graph.Node   `json:"target"`
	Callers []*graph.Node `json:"callers"`
	Paths   []*graph.Path `json:"paths"`
}

// GlobalUsageResult represents global variable usage.
type GlobalUsageResult struct {
	Target  *graph.Node   `json:"target"`
	Globals []*graph.Node `json:"globals"`
}

// SeamResult represents a suggested architectural seam (boundary).
type SeamResult struct {
	Seam string  `json:"seam"`
	File string  `json:"file"`
	Risk float64 `json:"risk"`
	Type string  `json:"type,omitempty"` // The type of seam (e.g., "ui", "db", "io")
}

// HotspotResult represents a high-risk area of the codebase.
type HotspotResult struct {
	Name  string  `json:"name"`
	File  string  `json:"file"`
	Risk  float64 `json:"risk"`
	Churn int     `json:"churn"`
}

// FileHistoryMetrics contains git history metadata for a file.
type FileHistoryMetrics struct {
	ChangeFrequency int      `json:"change_frequency"`
	LastChanged     string   `json:"last_changed"`
	CoChanges       []string `json:"co_changes"`
}

// ContaminationRule defines a heuristic rule for seeding initial contamination flags.
type ContaminationRule struct {
	Layer     string `json:"layer"`     // "ui", "db", "io"
	Pattern   string `json:"pattern"`   // Regex pattern for file path or function name
	Type      string `json:"type"`      // "file" or "function"
	Heuristic string `json:"heuristic"` // "path" or "content" (matches function body)
}

// DomainExplorationResult represents the result of exploring a feature's
// position in the RPG hierarchy.
type DomainExplorationResult struct {
	Feature   *graph.Node   `json:"feature"`
	Parent    *graph.Node   `json:"parent,omitempty"`
	Children  []*graph.Node `json:"children,omitempty"`
	Siblings  []*graph.Node `json:"siblings,omitempty"`
	Functions []*graph.Node `json:"functions,omitempty"`
}

// WhatIfResult represents the impact of removing/extracting nodes.
type WhatIfResult struct {
	SeveredEdges       []*graph.Edge `json:"severed_edges"`
	OrphanedNodes      []*graph.Node `json:"orphaned_nodes"`
	CrossBoundaryCalls []*graph.Edge `json:"cross_boundary_calls"`
	SharedState        []*graph.Node `json:"shared_state"`
}

// SemanticSeamResult represents a structural vs semantic divergence in a class or file.
type SemanticSeamResult struct {
	Container  string  `json:"container"`  // File or class name
	MethodA    string  `json:"method_a"`   // First function/method
	MethodB    string  `json:"method_b"`   // Second function/method
	Similarity float64 `json:"similarity"` // Cosine similarity between A and B
}

// GraphProvider defines the interface for graph database operations.
type GraphProvider interface {
	// Lifecycle
	Close() error

	// Core Operations
	Traverse(startNodeID string, relationship string, direction Direction, depth int) ([]*graph.Path, error)

	// High-Level Features
	SearchFeatures(embedding []float32, limit int) ([]*FeatureResult, error)
	SearchSimilarFunctions(embedding []float32, limit int) ([]*FeatureResult, error)
	GetNeighbors(nodeID string, depth int) (*NeighborResult, error)
	GetCallers(nodeID string) ([]string, error)
	GetImpact(nodeID string, depth int) (*ImpactResult, error)
	GetGlobals(nodeID string) (*GlobalUsageResult, error)
	GetSeams(modulePattern string, layer string) ([]*SeamResult, error)
	GetHotspots(modulePattern string) ([]*HotspotResult, error)
	FetchSource(nodeID string) (string, error)
	LocateUsage(sourceID string, targetID string) (any, error)
	ExploreDomain(featureID string) (*DomainExplorationResult, error)
	GetOverview() (*graph.Path, error)
	GetGraphState() (string, error)
	WhatIf(targets []string) (*WhatIfResult, error)
	GetSemanticSeams(ctx context.Context, similarityThreshold float64) ([]*SemanticSeamResult, error)

	// Test Coverage Analysis
	GetCoverage(nodeID string) ([]*graph.Node, error)
	LinkTests() error

	// Volatility & Risk Analysis
	SeedVolatility(modulePattern string, rules []ContaminationRule) error
	PropagateVolatility() error
	CalculateRiskScores() error
	UpdateFileHistory(metrics map[string]FileHistoryMetrics) error

	// Batch/Streaming Operations
	GetUnextractedFunctions(limit int) ([]*graph.Node, error)
	UpdateAtomicFeatures(id string, features []string) error
	GetUnembeddedNodes(limit int) ([]*graph.Node, error)
	UpdateEmbeddings(id string, embedding []float32) error
	GetEmbeddingsOnly() (map[string][]float32, error)
	GetFunctionMetadata() ([]*graph.Node, error)
	GetUnnamedFeatures(limit int) ([]*graph.Node, error)
	CountUnnamedFeatures() (int64, error)
	UpdateFeatureTopology(nodes []*graph.Node, edges []*graph.Edge) error
	UpdateFeatureSummary(id string, name string, summary string) error
}

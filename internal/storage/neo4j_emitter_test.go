package storage

import (
	"context"
	"clang-graphdb/internal/graph"
	"testing"
)

type mockLoader struct {
	nodeBatches [][]graph.Node
	edgeBatches [][]graph.Edge
}

func (m *mockLoader) BatchLoadNodes(ctx context.Context, nodes []graph.Node) error {
	batch := make([]graph.Node, len(nodes))
	copy(batch, nodes)
	m.nodeBatches = append(m.nodeBatches, batch)
	return nil
}

func (m *mockLoader) BatchLoadEdges(ctx context.Context, edges []graph.Edge) error {
	batch := make([]graph.Edge, len(edges))
	copy(batch, edges)
	m.edgeBatches = append(m.edgeBatches, batch)
	return nil
}

func TestNeo4jEmitter_Batching(t *testing.T) {
	mock := &mockLoader{}
	emitter := NewNeo4jEmitter(mock, context.Background(), 2)

	// Emit 5 nodes, should trigger 2 batches of 2, 1 node left in buffer
	nodes := []*graph.Node{
		{ID: "n1", Label: "Func"},
		{ID: "n2", Label: "Func"},
		{ID: "n3", Label: "Func"},
		{ID: "n4", Label: "Func"},
		{ID: "n5", Label: "Func"},
	}

	for _, n := range nodes {
		if err := emitter.EmitNode(n); err != nil {
			t.Fatalf("EmitNode failed: %v", err)
		}
	}

	if len(mock.nodeBatches) != 2 {
		t.Errorf("Expected 2 batches emitted, got %d", len(mock.nodeBatches))
	}

	if err := emitter.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if len(mock.nodeBatches) != 3 {
		t.Errorf("Expected 3 batches emitted after close, got %d", len(mock.nodeBatches))
	}

	if len(mock.nodeBatches[2]) != 1 || mock.nodeBatches[2][0].ID != "n5" {
		t.Errorf("Expected final batch to contain n5, got %v", mock.nodeBatches[2])
	}
}

func TestNeo4jEmitter_Edges(t *testing.T) {
	mock := &mockLoader{}
	emitter := NewNeo4jEmitter(mock, context.Background(), 3)

	edges := []*graph.Edge{
		{SourceID: "n1", TargetID: "n2", Type: "CALLS"},
		{SourceID: "n2", TargetID: "n3", Type: "CALLS"},
	}

	for _, e := range edges {
		if err := emitter.EmitEdge(e); err != nil {
			t.Fatalf("EmitEdge failed: %v", err)
		}
	}

	// Buffer not full yet
	if len(mock.edgeBatches) != 0 {
		t.Errorf("Expected 0 edge batches, got %d", len(mock.edgeBatches))
	}

	if err := emitter.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if len(mock.edgeBatches) != 1 {
		t.Errorf("Expected 1 edge batch after close, got %d", len(mock.edgeBatches))
	}
}

package storage

import (
	"context"
	"fmt"
	"clang-graphdb/internal/graph"
)

type NodeEdgeLoader interface {
	BatchLoadNodes(ctx context.Context, nodes []graph.Node) error
	BatchLoadEdges(ctx context.Context, edges []graph.Edge) error
}

type Neo4jEmitter struct {
	loader NodeEdgeLoader
	ctx    context.Context

	nodeBatch []graph.Node
	edgeBatch []graph.Edge
	batchSize int
}

func NewNeo4jEmitter(l NodeEdgeLoader, ctx context.Context, batchSize int) *Neo4jEmitter {
	if batchSize <= 0 {
		batchSize = 1000
	}
	return &Neo4jEmitter{
		loader:    l,
		ctx:       ctx,
		nodeBatch: make([]graph.Node, 0, batchSize),
		edgeBatch: make([]graph.Edge, 0, batchSize),
		batchSize: batchSize,
	}
}

func (e *Neo4jEmitter) EmitNode(node *graph.Node) error {
	e.nodeBatch = append(e.nodeBatch, *node)
	if len(e.nodeBatch) >= e.batchSize {
		if err := e.loader.BatchLoadNodes(e.ctx, e.nodeBatch); err != nil {
			return fmt.Errorf("failed to load node batch: %w", err)
		}
		e.nodeBatch = e.nodeBatch[:0]
	}
	return nil
}

func (e *Neo4jEmitter) EmitEdge(edge *graph.Edge) error {
	e.edgeBatch = append(e.edgeBatch, *edge)
	if len(e.edgeBatch) >= e.batchSize {
		if err := e.loader.BatchLoadEdges(e.ctx, e.edgeBatch); err != nil {
			return fmt.Errorf("failed to load edge batch: %w", err)
		}
		e.edgeBatch = e.edgeBatch[:0]
	}
	return nil
}

func (e *Neo4jEmitter) Close() error {
	var finalErr error
	if len(e.nodeBatch) > 0 {
		if err := e.loader.BatchLoadNodes(e.ctx, e.nodeBatch); err != nil {
			finalErr = fmt.Errorf("failed to load final node batch: %w", err)
		}
		e.nodeBatch = e.nodeBatch[:0]
	}

	if len(e.edgeBatch) > 0 {
		if err := e.loader.BatchLoadEdges(e.ctx, e.edgeBatch); err != nil {
			if finalErr != nil {
				finalErr = fmt.Errorf("%w; failed to load final edge batch: %v", finalErr, err)
			} else {
				finalErr = fmt.Errorf("failed to load final edge batch: %w", err)
			}
		}
		e.edgeBatch = e.edgeBatch[:0]
	}

	return finalErr
}

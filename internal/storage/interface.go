package storage

import "clang-graphdb/internal/graph"

type Emitter interface {
	EmitNode(node *graph.Node) error
	EmitEdge(edge *graph.Edge) error
	Close() error
}

package rpg

import (
	"fmt"
	"clang-graphdb/internal/graph"
	"strings"
)

// NodeToText converts a graph node (function) into a text representation for embedding.
// It includes multiple signals (file path, name, atomic features) so the embedding model
// can better triangulate the business domain membership.
func NodeToText(n graph.Node) string {
	var parts []string

	// File path provides structural domain context
	if file, ok := n.Properties["file"].(string); ok && file != "" {
		parts = append(parts, file)
	}

	// Function name provides behavioral context
	if name, ok := n.Properties["name"].(string); ok && name != "" {
		parts = append(parts, name)
	}

	// Atomic features provide semantic depth
	if af := getAtomicFeatures(n); len(af) > 0 {
		parts = append(parts, strings.Join(af, ", "))
	}

	if len(parts) > 0 {
		return strings.Join(parts, " | ")
	}

	return n.ID
}

func getAtomicFeatures(n graph.Node) []string {
	if af, ok := n.Properties["atomic_features"].([]string); ok && len(af) > 0 {
		return af
	}
	if afAny, ok := n.Properties["atomic_features"].([]interface{}); ok && len(afAny) > 0 {
		parts := make([]string, len(afAny))
		for j, v := range afAny {
			parts[j] = fmt.Sprintf("%v", v)
		}
		return parts
	}
	return nil
}

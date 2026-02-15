package rpg

import (
	"fmt"
	"graphdb/internal/graph"
	"strings"
)

// NodeToText converts a graph node (function) into a text representation for embedding.
// It prioritizes atomic_features, falls back to name, and finally ID.
func NodeToText(n graph.Node) string {
	if af, ok := n.Properties["atomic_features"].([]string); ok && len(af) > 0 {
		return strings.Join(af, ", ")
	} else if afAny, ok := n.Properties["atomic_features"].([]interface{}); ok && len(afAny) > 0 {
		parts := make([]string, len(afAny))
		for j, v := range afAny {
			parts[j] = fmt.Sprintf("%v", v)
		}
		return strings.Join(parts, ", ")
	} else {
		// Fallback: use function name
		if name, ok := n.Properties["name"].(string); ok && name != "" {
			return name
		}
		return n.ID
	}
}

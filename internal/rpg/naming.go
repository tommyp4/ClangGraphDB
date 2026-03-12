package rpg

import (
	"crypto/rand"
	"encoding/hex"
	"graphdb/internal/graph"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reCamel    = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	reNonAlpha = regexp.MustCompile(`[^a-zA-Z0-9]+`)
)

// GenerateDomainName derives a semantic name for a domain based on its LCA path
// or the content of its constituent nodes.
func GenerateDomainName(lca string, nodes []graph.Node) string {
	// 1. Analyze LCA
	base := filepath.Base(lca)

	// Normalize base for root/empty
	if lca == "." || lca == "" || lca == "/" {
		base = "."
	}

	// 2. Check if LCA is "Weak" (Generic)
	isWeak := base == "." || base == "internal" || base == "pkg" || base == "src" || base == "cmd"

	// 3. If Strong, use it
	if !isWeak {
		return strings.ToLower(base)
	}

	// 4. If Weak, try to extract a dominant term from nodes
	term := findDominantTerm(nodes)
	if term != "" {
		return term
	}

	// 5. Fallback
	return ""
}

func findDominantTerm(nodes []graph.Node) string {
	counts := make(map[string]int)

	for _, n := range nodes {
		nameVal, ok := n.Properties["name"]
		if !ok {
			continue
		}
		name, ok := nameVal.(string)
		if !ok {
			continue
		}

		words := splitNameIntoWords(name)
		for _, w := range words {
			// Filter out common short words or too generic ones could be added here
			if len(w) > 2 {
				counts[strings.ToLower(w)]++
			}
		}
	}

	var bestTerm string
	var maxCount int

	// Simple frequency max
	for term, count := range counts {
		if count > maxCount {
			maxCount = count
			bestTerm = term
		} else if count == maxCount {
			// Tie-breaking: lexicographical or longer?
			// Let's prefer shorter for "core" terms, or longer for specificity?
			// Let's just stick to stable (lexicographical) to be deterministic
			if term < bestTerm {
				bestTerm = term
			}
		}
	}

	return bestTerm
}

// splitNameIntoWords handles CamelCase, snake_case, etc.
func splitNameIntoWords(s string) []string {
	// 1. Insert space before Capital letters (CamelCase)
	// Match a lowercase followed by an uppercase
	s = reCamel.ReplaceAllString(s, "${1} ${2}")

	// 2. Replace non-alphanumeric with space
	s = reNonAlpha.ReplaceAllString(s, " ")

	// 3. Split by space
	return strings.Fields(s)
}

func GenerateShortUUID() string {
	b := make([]byte, 4) // 8 chars hex
	_, err := rand.Read(b)
	if err != nil {
		// Fallback to time or something if rand fails (unlikely)
		return "unknown"
	}
	return hex.EncodeToString(b)
}

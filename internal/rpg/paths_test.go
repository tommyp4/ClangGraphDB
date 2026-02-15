package rpg

import (
	"path/filepath"
	"testing"
)

func TestFindLowestCommonAncestor(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		expected string
	}{
		{
			name:     "empty list",
			paths:    []string{},
			expected: "",
		},
		{
			name:     "single path",
			paths:    []string{"/a/b/c"},
			expected: "/a/b/c",
		},
		{
			name:     "identical paths",
			paths:    []string{"/a/b/c", "/a/b/c"},
			expected: "/a/b/c",
		},
		{
			name:     "common parent",
			paths:    []string{"/a/b/c", "/a/b/d"},
			expected: "/a/b",
		},
		{
			name:     "nested paths",
			paths:    []string{"/a/b", "/a/b/c"},
			expected: "/a/b",
		},
		{
			name:     "divergent roots",
			paths:    []string{"/a/b", "/c/d"},
			expected: "/",
		},
		{
			name:     "partial name match",
			paths:    []string{"/a/b", "/a/bc"},
			expected: "/a",
		},
		{
			name:     "relative paths",
			paths:    []string{"a/b", "a/c"},
			expected: "a",
		},
		{
			name:     "relative divergent",
			paths:    []string{"a/b", "c/d"},
			expected: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Normalize inputs and expected for current OS
			var cleanPaths []string
			for _, p := range tt.paths {
				cleanPaths = append(cleanPaths, filepath.Clean(p))
			}
			expected := filepath.Clean(tt.expected)
			if tt.expected == "" {
				expected = ""
			}

			got := FindLowestCommonAncestor(cleanPaths)
			if got != expected {
				t.Errorf("FindLowestCommonAncestor(%v) = %q, want %q", cleanPaths, got, expected)
			}
		})
	}
}

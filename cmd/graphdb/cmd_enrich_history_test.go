package main

import (
	"reflect"
	"testing"
)

func TestParseGitLog(t *testing.T) {
	input := `COMMIT|2023-01-01T10:00:00Z
file1.go
file2.go
COMMIT|2023-01-02T10:00:00Z
file1.go
file3.go
COMMIT|2023-01-03T10:00:00Z
file1.go
file2.go
file3.go
COMMIT|2023-01-04T10:00:00Z
file1.go
file2.go
`
	// Co-changes:
	// file1.go: file2.go (3 times), file3.go (2 times)
	// file2.go: file1.go (3 times), file3.go (1 time)
	// file3.go: file1.go (2 times), file2.go (1 time)

	metrics, err := parseGitLog([]byte(input))
	if err != nil {
		t.Fatalf("parseGitLog failed: %v", err)
	}

	if len(metrics) != 3 {
		t.Errorf("Expected 3 files, got %d", len(metrics))
	}

	m1 := metrics["file1.go"]
	if m1.ChangeFrequency != 4 {
		t.Errorf("Expected file1.go frequency 4, got %d", m1.ChangeFrequency)
	}
	if m1.LastChanged != "2023-01-04T10:00:00Z" {
		t.Errorf("Expected file1.go last changed 2023-01-04T10:00:00Z, got %s", m1.LastChanged)
	}
	// file2.go co-changed 3 times, so it should be in co-changes
	if !reflect.DeepEqual(m1.CoChanges, []string{"file2.go"}) {
		t.Errorf("Expected file1.go co-changes [file2.go], got %v", m1.CoChanges)
	}

	m2 := metrics["file2.go"]
	if m2.ChangeFrequency != 3 {
		t.Errorf("Expected file2.go frequency 3, got %d", m2.ChangeFrequency)
	}
	if !reflect.DeepEqual(m2.CoChanges, []string{"file1.go"}) {
		t.Errorf("Expected file2.go co-changes [file1.go], got %v", m2.CoChanges)
	}

	m3 := metrics["file3.go"]
	if m3.ChangeFrequency != 2 {
		t.Errorf("Expected file3.go frequency 2, got %d", m3.ChangeFrequency)
	}
	if len(m3.CoChanges) != 0 {
		t.Errorf("Expected file3.go co-changes [], got %v", m3.CoChanges)
	}
}

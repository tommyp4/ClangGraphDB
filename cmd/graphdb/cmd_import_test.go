package main

import (
	"testing"
)

func TestDecodeJSONRow(t *testing.T) {
	raw := []byte(`{"id":"1", "arr":[10, 1.5, {"nested": 20}], "score": 0.95, "meta": {"count": 5}}`)
	flat, err := decodeJSONRow(raw)
	if err != nil {
		t.Fatalf("Failed to decode JSON row: %v", err)
	}

	// Assert score is float64
	score, ok := flat["score"].(float64)
	if !ok {
		t.Errorf("Expected score to be float64, got %T", flat["score"])
	}
	if score != 0.95 {
		t.Errorf("Expected score to be 0.95, got %v", score)
	}

	// Assert meta.count is int64
	meta, ok := flat["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected meta to be map[string]interface{}, got %T", flat["meta"])
	}
	count, ok := meta["count"].(int64)
	if !ok {
		t.Errorf("Expected count to be int64, got %T", meta["count"])
	}
	if count != 5 {
		t.Errorf("Expected count to be 5, got %v", count)
	}

	// Assert arr elements
	arr, ok := flat["arr"].([]interface{})
	if !ok {
		t.Fatalf("Expected arr to be []interface{}, got %T", flat["arr"])
	}
	if len(arr) != 3 {
		t.Fatalf("Expected arr length 3, got %d", len(arr))
	}

	if val, ok := arr[0].(int64); !ok || val != 10 {
		t.Errorf("Expected arr[0] to be int64(10), got %T(%v)", arr[0], arr[0])
	}
	if val, ok := arr[1].(float64); !ok || val != 1.5 {
		t.Errorf("Expected arr[1] to be float64(1.5), got %T(%v)", arr[1], arr[1])
	}
	nested, ok := arr[2].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected arr[2] to be map[string]interface{}, got %T", arr[2])
	}
	if val, ok := nested["nested"].(int64); !ok || val != 20 {
		t.Errorf("Expected nested['nested'] to be int64(20), got %T(%v)", nested["nested"], nested["nested"])
	}
}

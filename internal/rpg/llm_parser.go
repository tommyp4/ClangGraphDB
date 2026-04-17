package rpg

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParseLLMJSON aggressively scans for JSON object or array boundaries before unmarshaling.
func ParseLLMJSON(responseText string, target interface{}) error {
	origText := responseText

	startIdx := strings.IndexAny(responseText, "{[")
	if startIdx == -1 {
		return fmt.Errorf("failed to find beginning of JSON: %s", origText)
	}

	var endChar byte
	if responseText[startIdx] == '{' {
		endChar = '}'
	} else {
		endChar = ']'
	}

	endIdx := strings.LastIndexByte(responseText, endChar)
	if endIdx == -1 || endIdx <= startIdx {
		return fmt.Errorf("failed to find end of JSON: %s", origText)
	}

	jsonStr := responseText[startIdx : endIdx+1]

	if err := json.Unmarshal([]byte(jsonStr), target); err != nil {
		return fmt.Errorf("failed to parse LLM response: %w. Raw: %s", err, origText)
	}
	return nil
}

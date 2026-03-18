package rpg

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParseLLMJSON aggressively strips markdown and whitespace before unmarshaling.
func ParseLLMJSON(responseText string, target interface{}) error {
	origText := responseText

	// 1. Initial trim
	responseText = strings.TrimSpace(responseText)

	// 2. Trim optional prefix
	if strings.HasPrefix(responseText, "```json") {
		responseText = strings.TrimPrefix(responseText, "```json")
	} else if strings.HasPrefix(responseText, "```") {
		responseText = strings.TrimPrefix(responseText, "```")
	}

	// 3. Re-trim leading whitespace and any extra backticks
	responseText = strings.TrimLeft(responseText, "` \n\t\r")

	// 4. Trim suffix (extra backticks and whitespace from the right)
	responseText = strings.TrimRight(responseText, "` \n\t\r")

	if err := json.Unmarshal([]byte(responseText), target); err != nil {
		return fmt.Errorf("failed to parse LLM response: %w. Raw: %s", err, origText)
	}
	return nil
}

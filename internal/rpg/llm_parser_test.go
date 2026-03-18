package rpg

import (
	"testing"
)

func TestParseLLMJSON(t *testing.T) {
	type testStruct struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name         string
		responseText string
		wantErr      bool
	}{
		{
			name:         "Standard JSON without markdown",
			responseText: `{"name": "test"}`,
			wantErr:      false,
		},
		{
			name:         "Standard Markdown block",
			responseText: "```json\n{\"name\": \"test\"}\n```",
			wantErr:      false,
		},
		{
			name:         "Markdown block with leading/trailing newlines",
			responseText: "\n\n ```json\n{\"name\": \"test\"}\n``` \n\n ",
			wantErr:      false,
		},
		{
			name:         "Markdown block with extra backticks",
			responseText: "```json\n{\"name\": \"test\"}\n```\n```",
			wantErr:      false,
		},
		{
			name:         "Malformed JSON string",
			responseText: `{"name": "test"`,
			wantErr:      true,
		},
		{
			name:         "Markdown without json language tag",
			responseText: "```\n{\"name\": \"test\"}\n```",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got testStruct
			err := ParseLLMJSON(tt.responseText, &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLLMJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Name != "test" {
				t.Errorf("ParseLLMJSON() got = %v, want name 'test'", got.Name)
			}
		})
	}
}

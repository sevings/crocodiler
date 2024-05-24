package croc

import "testing"

func TestTruncateDefinition(t *testing.T) {
	tests := []struct {
		text   string
		maxLen int
		want   string
	}{
		{"short text", 10, "short text"},
		{"long text that needs to be truncated", 10, "long text…"},
		{"this is a very long text that needs to be truncated at a word boundary", 20, "this is a very long…"},
		{"this is a very. long text that needs to be truncated at a sentence boundary", 20, "this is a very…"},
		{"this_is_a_very_long_text", 20, "this_is_a_very_long…"},
		{"", 10, ""},
		{"a", 1, "a"},
	}

	for _, tt := range tests {
		got := truncateDefinition(tt.text, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateDefinition(%q, %d) = %q, want %q", tt.text, tt.maxLen, got, tt.want)
		}
	}
}

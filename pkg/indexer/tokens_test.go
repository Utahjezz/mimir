package indexer

import (
	"reflect"
	"testing"
)

func TestSplitIdentifier(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		// camelCase
		{"getUserPrimaryAddress", []string{"get", "user", "primary", "address"}},
		// PascalCase
		{"AddressBookEntity", []string{"address", "book", "entity"}},
		// snake_case
		{"get_file_meta", []string{"get", "file", "meta"}},
		// SCREAMING_SNAKE
		{"MAX_RETRY_COUNT", []string{"max", "retry", "count"}},
		// Acronym run followed by lowercase: HTTPRequest → [http, request]
		{"parseHTTPRequest", []string{"parse", "http", "request"}},
		// All uppercase acronym
		{"HTTP", []string{"http"}},
		// Single word
		{"index", []string{"index"}},
		// Already lowercase with no separator
		{"foo", []string{"foo"}},
		// Empty string
		{"", nil},
		// Leading/trailing underscores
		{"_private_field_", []string{"private", "field"}},
		// Mixed: camel + underscore
		{"myFunc_helper", []string{"my", "func", "helper"}},
		// Digits treated as regular characters (no split on digit boundaries)
		{"base64Encode", []string{"base64", "encode"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitIdentifier(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitIdentifier(%q)\n  got  %v\n  want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestTokenizeQuery(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"user address", []string{"user", "address"}},
		{"  fetch  USER  ", []string{"fetch", "user"}},
		// Deduplication
		{"foo foo bar", []string{"foo", "bar"}},
		{"", nil},
		// camelCase word is split into sub-tokens
		{"getUserPrimaryAddress", []string{"get", "user", "primary", "address"}},
		// PascalCase word
		{"AddressBookEntity", []string{"address", "book", "entity"}},
		// snake_case word
		{"get_file_meta", []string{"get", "file", "meta"}},
		// mixed: plain word + camelCase identifier — cross-word deduplication
		{"user getUserAddress", []string{"user", "get", "address"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := tokenizeQuery(tt.input)
			if len(got) == 0 && len(tt.want) == 0 {
				return // both empty — pass
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("tokenizeQuery(%q)\n  got  %v\n  want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormaliseStringToken(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		// slash-separated MIME type
		{`"application/json"`, []string{"application", "json"}},
		// hyphenated header name
		{`"Content-Type"`, []string{"content", "type"}},
		// format string: single-char fragments (%s, %w) are discarded
		{`"cannot open %s: %w"`, []string{"cannot", "open"}},
		// plain word — unchanged (still lowercased)
		{`"hello"`, []string{"hello"}},
		// empty string
		{`""`, nil},
		// only separators — no fragments
		{`"/"`, nil},
		// backtick-quoted (Go raw string)
		{"`application/json`", []string{"application", "json"}},
		// single-quoted
		{`'text/html'`, []string{"text", "html"}},
		// colon-separated
		{`"foo:bar"`, []string{"foo", "bar"}},
		// unquoted plain word
		{"hello", []string{"hello"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normaliseStringToken(tt.input)
			if len(got) == 0 && len(tt.want) == 0 {
				return // both nil/empty — pass
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("normaliseStringToken(%q)\n  got  %v\n  want %v", tt.input, got, tt.want)
			}
		})
	}
}

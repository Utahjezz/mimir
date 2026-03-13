package indexer

import (
	"strings"
	"unicode"
)

// splitIdentifier splits a symbol name into lowercase tokens by recognising
// camelCase, PascalCase, snake_case, and SCREAMING_SNAKE_CASE boundaries.
//
// Examples:
//
//	useCesarinaPrimaryAddress → [use cesarina primary address]
//	AddressBookEntity         → [address book entity]
//	get_file_meta             → [get file meta]
//	MAX_RETRY_COUNT           → [max retry count]
//	parseHTTPRequest          → [parse http request]
func splitIdentifier(name string) []string {
	runes := []rune(name)
	n := len(runes)
	if n == 0 {
		return nil
	}

	var tokens []string
	start := 0

	for i := 1; i < n; i++ {
		prev := runes[i-1]
		cur := runes[i]

		var split bool
		switch {
		// underscore boundary — skip the underscore itself
		case cur == '_':
			if i > start {
				tokens = appendToken(tokens, string(runes[start:i]))
			}
			start = i + 1
			continue

		// lowercase or digit → Uppercase: split before the uppercase letter.
		// Covers "parseHTTP" and "base64Encode".
		case (unicode.IsLower(prev) || unicode.IsDigit(prev)) && unicode.IsUpper(cur):
			split = true

		// Uppercase run followed by Uppercase+Lowercase: "HTTPRequest" → [HTTP, Request]
		case i+1 < n && unicode.IsUpper(prev) && unicode.IsUpper(cur) && unicode.IsLower(runes[i+1]):
			split = true
		}

		if split {
			tokens = appendToken(tokens, string(runes[start:i]))
			start = i
		}
	}

	// Append the final segment.
	if start < n {
		tokens = appendToken(tokens, string(runes[start:]))
	}

	return tokens
}

// appendToken lowercases s and appends it to dst, skipping empty strings and
// pure-underscore segments.
func appendToken(dst []string, s string) []string {
	s = strings.ToLower(strings.Trim(s, "_"))
	if s == "" {
		return dst
	}
	return append(dst, s)
}

// tokenizeQuery splits a free-text search query into lowercase words and
// returns them deduplicated while preserving order. Short words (< 2 chars)
// are kept so single-letter identifiers still work.
func tokenizeQuery(query string) []string {
	words := strings.Fields(query)
	seen := make(map[string]struct{}, len(words))
	out := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.ToLower(w)
		if _, ok := seen[w]; !ok {
			seen[w] = struct{}{}
			out = append(out, w)
		}
	}
	return out
}

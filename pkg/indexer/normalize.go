package indexer

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/clipperhouse/jargon"
	"github.com/clipperhouse/jargon/filters/stackoverflow"
	jsynonyms "github.com/clipperhouse/jargon/filters/synonyms"
)

//go:embed normalization.json
var normalizationJSON []byte

// QueryNormalizer expands plain fuzzy-query terms into canonical and alternate
// forms suitable for tolerant search matching.
type QueryNormalizer interface {
	NormalizeQuery(q string) NormalizedQuery
}

// NormalizedQuery contains grouped query terms. Each group counts as one fuzzy
// match requirement even if it has multiple expanded search forms.
type NormalizedQuery struct {
	Original string
	Tokens   []NormalizedToken
}

// SearchWords flattens grouped expansions into the deduplicated search terms
// used to build the broad FTS recall query.
func (q NormalizedQuery) SearchWords() []string {
	seen := make(map[string]struct{})
	words := make([]string, 0, len(q.Tokens)*2)
	for _, token := range q.Tokens {
		for _, expanded := range token.Expanded {
			if expanded == "" {
				continue
			}
			if _, ok := seen[expanded]; ok {
				continue
			}
			seen[expanded] = struct{}{}
			words = append(words, expanded)
		}
	}
	return words
}

// NormalizedToken represents one original fuzzy-query token plus any canonical
// or alternate forms that should count as equivalent for matching purposes.
type NormalizedToken struct {
	Original  string
	Canonical string
	Expanded  []string
}

type normalizationConfig struct {
	Version           int               `json:"version"`
	IgnoreRunes       []string          `json:"ignore_runes"`
	TechTerms         map[string]string `json:"tech_terms"`
	CodeAbbreviations map[string]string `json:"code_abbreviations"`
}

type jargonNormalizer struct {
	customFilter jargon.Filter
}

type noopNormalizer struct{}

var defaultQueryNormalizer = mustBuildQueryNormalizer()

func mustBuildQueryNormalizer() QueryNormalizer {
	normalizer, err := newJargonNormalizer(normalizationJSON)
	if err != nil {
		return noopNormalizer{}
	}
	return normalizer
}

func newJargonNormalizer(raw []byte) (QueryNormalizer, error) {
	var cfg normalizationConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}

	mappings := make(map[string]string, len(cfg.TechTerms)+len(cfg.CodeAbbreviations))
	mergeCanonicalMappings(mappings, cfg.TechTerms)
	mergeCanonicalMappings(mappings, cfg.CodeAbbreviations)

	return jargonNormalizer{
		customFilter: jsynonyms.NewFilter(mappings, true, decodeIgnoreRunes(cfg.IgnoreRunes)),
	}, nil
}

func mergeCanonicalMappings(dst, src map[string]string) {
	for aliases, canonical := range src {
		dst[strings.ToLower(strings.TrimSpace(aliases))] = strings.ToLower(strings.TrimSpace(canonical))
	}
}

func decodeIgnoreRunes(raw []string) []rune {
	ignore := make([]rune, 0, len(raw))
	for _, value := range raw {
		runes := []rune(value)
		if len(runes) != 1 {
			continue
		}
		ignore = append(ignore, runes[0])
	}
	return ignore
}

func (noopNormalizer) NormalizeQuery(q string) NormalizedQuery {
	return buildNormalizedQuery(q, baseQueryWords(q), nil)
}

func (n jargonNormalizer) NormalizeQuery(q string) NormalizedQuery {
	baseWords := baseQueryWords(q)
	if len(baseWords) == 0 {
		return NormalizedQuery{Original: q}
	}

	canonicalByWord := make(map[string][]string, len(baseWords))
	for _, word := range baseWords {
		canonicalWords, err := n.normalizeWord(word)
		if err != nil {
			return buildNormalizedQuery(q, baseWords, nil)
		}
		if len(canonicalWords) > 0 {
			canonicalByWord[word] = canonicalWords
		}
	}

	return buildNormalizedQuery(q, baseWords, canonicalByWord)
}

func (n jargonNormalizer) normalizeWord(word string) ([]string, error) {
	stream := jargon.TokenizeString(word).Filter(stackoverflow.Tags, n.customFilter)
	terms, err := streamWords(stream)
	if err != nil {
		return nil, err
	}
	return tokeniseTerms(terms), nil
}

func buildNormalizedQuery(q string, baseWords []string, canonicalByWord map[string][]string) NormalizedQuery {
	tokens := make([]NormalizedToken, 0, len(baseWords))
	for _, word := range baseWords {
		expanded := []string{word}
		canonical := word
		for _, candidate := range canonicalByWord[word] {
			expanded = appendUniqueStrings(expanded, candidate)
			canonical = candidate
		}
		tokens = append(tokens, NormalizedToken{
			Original:  word,
			Canonical: canonical,
			Expanded:  expanded,
		})
	}

	return NormalizedQuery{Original: q, Tokens: tokens}
}

func baseQueryWords(q string) []string {
	words := tokenizeQuery(q)
	terms, err := streamWords(jargon.TokenizeString(q))
	if err != nil {
		return words
	}
	return appendUniqueStrings(words, tokeniseTerms(terms)...)
}

func streamWords(stream *jargon.TokenStream) ([]string, error) {
	stream = stream.Words().Distinct()
	words := make([]string, 0)
	for stream.Scan() {
		word := strings.ToLower(stream.Token().String())
		if word == "" {
			continue
		}
		words = append(words, word)
	}
	if err := stream.Err(); err != nil {
		return nil, err
	}
	return words, nil
}

func tokeniseTerms(terms []string) []string {
	words := make([]string, 0, len(terms))
	for _, term := range terms {
		words = appendUniqueStrings(words, tokenizeQuery(term)...)
	}
	return words
}

func appendUniqueStrings(dst []string, values ...string) []string {
	seen := make(map[string]struct{}, len(dst))
	for _, value := range dst {
		seen[value] = struct{}{}
	}
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		dst = append(dst, value)
	}
	return dst
}

package indexer

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/Utahjezz/mimir/pkg/indexer/languages/csharp"
	"github.com/Utahjezz/mimir/pkg/indexer/languages/golang"
	"github.com/Utahjezz/mimir/pkg/indexer/languages/javascript"
	"github.com/Utahjezz/mimir/pkg/indexer/languages/python"
	"github.com/Utahjezz/mimir/pkg/indexer/languages/typescript"
)

// langEntry holds the resolved language and query string for a given extension,
// built once at NewMuncher time from registeredLanguages.
type langEntry struct {
	language      *tree_sitter.Language
	query         string
	compiledQuery *tree_sitter.Query // compiled once at startup; never closed
}

// languageDefinition groups a single grammar instance with its query string
// and all file extensions it handles.
type languageDefinition struct {
	language   *tree_sitter.Language
	query      string
	extensions []string
}

// registeredLanguages is the single source of truth for all supported grammars.
// To add a new language: import its package and append a languageDefinition here.
var registeredLanguages = []languageDefinition{
	{
		language:   javascript.Language,
		query:      javascript.Queries,
		extensions: javascript.Extensions,
	},
	{
		language:   typescript.Language,
		query:      typescript.Queries,
		extensions: typescript.Extensions,
	},
	{
		language:   typescript.TSXLanguage,
		query:      typescript.Queries,
		extensions: typescript.TSXExtensions,
	},
	{
		language:   golang.Language,
		query:      golang.Queries,
		extensions: golang.Extensions,
	},
	{
		language:   python.Language,
		query:      python.Queries,
		extensions: python.Extensions,
	},
	{
		language:   csharp.Language,
		query:      csharp.Queries,
		extensions: csharp.Extensions,
	},
}

// callDefinition maps a human-readable language name to the grammar and
// query strings used by ExtractCalls.
//
// callQuery matches direct function/method calls: foo(), obj.Method().
// refQuery matches identifier references used as values (not called directly):
// struct literal fields (RunE: runIndex), var/assignment RHS (f = myFunc).
// refQuery may be empty for languages that do not need it.
type callDefinition struct {
	language          *tree_sitter.Language
	callQuery         string
	refQuery          string
	compiledCallQuery *tree_sitter.Query // compiled once at startup; never closed
	compiledRefQuery  *tree_sitter.Query // nil when refQuery is empty
}

// registeredCallLanguages lists languages that support call-site extraction.
var registeredCallLanguages = []struct {
	name string
	callDefinition
}{
	{
		name: "go",
		callDefinition: callDefinition{
			language:  golang.Language,
			callQuery: golang.CallQueries,
			refQuery:  golang.RefQueries,
		},
	},
	{
		name: "javascript",
		callDefinition: callDefinition{
			language:  javascript.Language,
			callQuery: javascript.JSXCallQueries,
			refQuery:  javascript.RefQueries,
		},
	},
	{
		name: "typescript",
		callDefinition: callDefinition{
			language:  typescript.Language,
			callQuery: typescript.CallQueries,
			refQuery:  typescript.RefQueries,
		},
	},
	{
		name: "tsx",
		callDefinition: callDefinition{
			language:  typescript.TSXLanguage,
			callQuery: typescript.TSXCallQueries,
			refQuery:  typescript.RefQueries,
		},
	},
	{
		name: "python",
		callDefinition: callDefinition{
			language:  python.Language,
			callQuery: python.CallQueries,
			refQuery:  python.RefQueries,
		},
	},
	{
		name: "csharp",
		callDefinition: callDefinition{
			language:  csharp.Language,
			callQuery: csharp.CallQueries,
			refQuery:  csharp.RefQueries,
		},
	},
}

// buildCallLangMap builds a language-name → callDefinition lookup used by
// ExtractCalls. Queries are compiled once here and reused for every file.
func buildCallLangMap() map[string]callDefinition {
	m := make(map[string]callDefinition, len(registeredCallLanguages))
	for _, def := range registeredCallLanguages {
		cd := def.callDefinition

		cq, err := tree_sitter.NewQuery(cd.language, cd.callQuery)
		if err != nil {
			panic("mimir: failed to compile callQuery for " + def.name + ": " + err.Error())
		}
		cd.compiledCallQuery = cq

		if cd.refQuery != "" {
			rq, err := tree_sitter.NewQuery(cd.language, cd.refQuery)
			if err != nil {
				panic("mimir: failed to compile refQuery for " + def.name + ": " + err.Error())
			}
			cd.compiledRefQuery = rq
		}

		m[def.name] = cd
	}
	return m
}

// callLangMap is the global language-name → callDefinition lookup.
var callLangMap = buildCallLangMap()

func buildLangMap() map[string]langEntry {
	langs := make(map[string]langEntry)
	for _, def := range registeredLanguages {
		cq, err := tree_sitter.NewQuery(def.language, def.query)
		if err != nil {
			panic("mimir: failed to compile symbol query for language: " + err.Error())
		}
		entry := langEntry{
			language:      def.language,
			query:         def.query,
			compiledQuery: cq,
		}
		for _, ext := range def.extensions {
			langs[ext] = entry
		}
	}
	return langs
}

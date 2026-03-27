package indexer

import "time"

// indexVersion is bumped when the on-disk format changes in a breaking way.
const indexVersion = 6

// SchemaVersion returns the current index schema version.
// Exposed so CLI commands can print it alongside the binary version.
func SchemaVersion() int { return indexVersion }

// CallSite records a single outgoing call extracted from a source file.
// CalleeName is the bare identifier of the called function or method.
// Line is 1-based, matching tree-sitter row + 1.
type CallSite struct {
	CalleeName string `json:"callee_name"`
	Line       int    `json:"line"`
}

// FileMeta holds the stored stat+hash metadata for a single indexed file.
// Used by processJob to determine whether a file needs re-parsing.
type FileMeta struct {
	Hash  string // sha256 hex
	Mtime string // RFC3339 UTC modification time at index time
	Size  int64  // file size in bytes at index time
}

// FileEntry holds the per-file metadata and extracted symbols.
// Path is stored as the key in the DB files table, not repeated here.
type FileEntry struct {
	Language  string       `json:"language"`
	SHA256    string       `json:"sha256"`
	Mtime     string       `json:"mtime"` // RFC3339 UTC mtime recorded at index time
	Size      int64        `json:"size"`  // file size in bytes recorded at index time
	IndexedAt time.Time    `json:"indexed_at"`
	Symbols   []SymbolInfo `json:"symbols"`
	Calls     []CallSite   `json:"calls,omitempty"`
}

// FileError records a per-file failure during indexing.
// Fatal walk errors are returned as the Run error return value;
// per-file read/parse failures are collected in IndexStats so the rest
// of the index is still saved.
type FileError struct {
	Path string
	Err  error
}

func (e FileError) Error() string {
	return e.Path + ": " + e.Err.Error()
}

// IndexStats summarises the outcome of a Run call.
type IndexStats struct {
	Unchanged  int
	Updated    int
	Added      int
	Removed    int
	Errors     int
	FileErrors []FileError
}

// LanguageStat holds per-language file and symbol counts for RepoReport.
type LanguageStat struct {
	Language    string `json:"language"`
	FileCount   int    `json:"file_count"`
	SymbolCount int    `json:"symbol_count"`
}

// SymbolTypeStat holds per-type symbol counts for RepoReport.
type SymbolTypeStat struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// RepoReport is the result of ReportIndex: a high-level summary of what
// is stored in the index for a given repository root.
type RepoReport struct {
	RepoID      string           `json:"repo_id"`
	Root        string           `json:"root"`
	GitHead     string           `json:"git_head,omitempty"`
	IndexedAt   string           `json:"indexed_at"`
	FileCount   int              `json:"file_count"`
	SymbolCount int              `json:"symbol_count"`
	Languages   []LanguageStat   `json:"languages"`
	SymbolTypes []SymbolTypeStat `json:"symbol_types"`
}

package workspace

import (
	"errors"
	"fmt"
	"time"
)

// workspaceVersion is bumped when the on-disk format changes in a breaking way.
const workspaceVersion = 3

func SchemaVersion() int { return workspaceVersion }

// SchemaMismatchError is returned by OpenWorkspace when the on-disk workspace
// was created with a different schema version than the current binary expects.
// The caller should instruct the user to recreate the workspace.
type SchemaMismatchError struct {
	Stored  int // version recorded in the existing workspace
	Current int // version expected by this binary
}

func (e *SchemaMismatchError) Error() string {
	return fmt.Sprintf("workspace schema mismatch: stored v%d, current v%d", e.Stored, e.Current)
}

// IsSchemaMismatch reports whether err (or any error in its chain) is a
// *SchemaMismatchError.
func IsSchemaMismatch(err error) bool {
	var target *SchemaMismatchError
	return errors.As(err, &target)
}

type Repository struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	AddedAt     time.Time `json:"added_at"`
	LastIndexed time.Time `json:"last_indexed"`
}

// Link represents a manually declared cross-repo symbol relationship stored
// in the workspace. It maps a symbol in one repository to a symbol in another.
type Link struct {
	ID        int64             `json:"id"`
	SrcRepoID string            `json:"src_repo_id"`
	SrcSymbol string            `json:"src_symbol"`
	SrcFile   string            `json:"src_file"`
	DstRepoID string            `json:"dst_repo_id"`
	DstSymbol string            `json:"dst_symbol"`
	DstFile   string            `json:"dst_file"`
	Note      string            `json:"note"`
	CreatedAt time.Time         `json:"created_at"`
	Meta      map[string]string `json:"meta,omitempty"`

	// Validation fields populated when --check is used.
	// These fields are only present in the JSON output when validation
	// has been performed. They are not stored in the database.
	SrcValid      bool   `json:"src_valid,omitempty"`
	SrcFileValid  bool   `json:"src_file_valid,omitempty"`
	SrcActualFile string `json:"src_actual_file,omitempty"`
	SrcError      string `json:"src_error,omitempty"`
	DstValid      bool   `json:"dst_valid,omitempty"`
	DstFileValid  bool   `json:"dst_file_valid,omitempty"`
	DstActualFile string `json:"dst_actual_file,omitempty"`
	DstError      string `json:"dst_error,omitempty"`
}

// ValidationResult holds the outcome of validating a single link.
// It preserves the original Link fields and adds validation-specific
// diagnostics. It is not stored in the database.
type ValidationResult struct {
	Link

	// SrcValid reports whether the source symbol was found in the src repo.
	SrcValid bool
	// SrcFileValid reports whether the symbol was found at the recorded path.
	SrcFileValid bool
	// SrcActualFile is the path where the symbol was actually found.
	// If the symbol was not found, this is empty.
	SrcActualFile string
	// SrcError is a non-empty error message if validation could not complete
	// (e.g., repo not found, index unavailable). Empty if validation succeeded.
	SrcError string

	// DstValid reports whether the destination symbol was found.
	DstValid bool
	// DstFileValid reports whether the symbol was found at the recorded path.
	DstFileValid bool
	// DstActualFile is the path where the symbol was actually found.
	DstActualFile string
	// DstError is a non-empty error message if validation could not complete.
	DstError string
}

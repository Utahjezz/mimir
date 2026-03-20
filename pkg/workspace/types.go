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
}

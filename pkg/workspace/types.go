package workspace

import "time"

// workspaceVersion is bumped when the on-disk format changes in a breaking way.
const workspaceVersion = 1

func SchemaVersion() int { return workspaceVersion }

type Repository struct {
	ID          string    `json:"id"`
	AddedAt     time.Time `json:"added_at"`
	LastIndexed time.Time `json:"last_indexed"`
}

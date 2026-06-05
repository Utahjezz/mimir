package graph

import (
	"database/sql"

	"github.com/Utahjezz/mimir/pkg/indexer"
	"github.com/Utahjezz/mimir/pkg/workspace"
)

// Scope identifies the aggregation level used when building a graph.
// It is a string type so future scopes can be added without changing the API.
type Scope string

const (
	// ScopePackage aggregates relationships at the package level.
	ScopePackage Scope = "package"
)

const (
	// FormatJSON renders graph output as structured JSON.
	FormatJSON = "json"
	// FormatMermaid renders graph output as Mermaid text.
	FormatMermaid = "mermaid"
)

// Graph is the normalized graph model shared by builders, filters, and renderers.
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Node represents a graph node at the active aggregation scope.
type Node struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Package   string `json:"package"`
	CallCount int    `json:"call_count"`
	RepoID    string `json:"repo_id,omitempty"`
}

// Edge represents a directed relationship between two nodes.
type Edge struct {
	Source      string            `json:"source"`
	Target      string            `json:"target"`
	CallCount   int               `json:"call_count"`
	Functions   []string          `json:"functions"`
	IsCrossRepo bool              `json:"is_cross_repo,omitempty"`
	RepoID      string            `json:"repo_id,omitempty"`
	LinkMeta    map[string]string `json:"link_meta,omitempty"`
}

// GraphOptions controls graph construction, filtering, and rendering behavior.
type GraphOptions struct {
	Scope     Scope  `json:"scope"`
	Format    string `json:"format"`
	Focus     string `json:"focus,omitempty"`
	Exclude   string `json:"exclude,omitempty"`
	MinCalls  int    `json:"min_calls"`
	Depth     int    `json:"depth"`
	Workspace string `json:"workspace,omitempty"`
	NoRefresh bool   `json:"no_refresh"`
}

// BuildOptions provides explicit inputs to graph builders.
// Refs and WorkspaceLinks are optional preloaded data sources for later subtasks.
type BuildOptions struct {
	Root           string           `json:"root"`
	IndexDB        *sql.DB          `json:"-"`
	Options        GraphOptions     `json:"options"`
	Refs           []indexer.RefRow `json:"refs,omitempty"`
	WorkspaceLinks []workspace.Link `json:"workspace_links,omitempty"`
}

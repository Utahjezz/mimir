package commands

import (
	"encoding/json"
	"fmt"

	"github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

const defaultConcurrency = 2

var (
	workspaceIndexJSON        bool
	workspaceIndexRebuild     bool
	workspaceIndexConcurrency int
)

var workspaceIndexCmd = &cobra.Command{
	Use:   "index [workspace_name]",
	Short: "Index all repositories in a workspace",
	Long:  `Index all repositories in a workspace. If workspace_name is omitted the current active workspace is used. Repos are indexed incrementally and concurrently.`,
	Args:  cobra.RangeArgs(0, 1),
	RunE:  runWorkspaceIndex,
}

func runWorkspaceIndex(cmd *cobra.Command, args []string) error {
	workspaceName, err := resolveWorkspaceName(args, 0)
	if err != nil {
		return err
	}

	db, err := workspace.OpenWorkspace(workspaceName)
	if err != nil {
		return fmt.Errorf("cannot open workspace: %w", err)
	}
	defer db.Close()

	results, err := workspace.IndexWorkspace(db, workspaceIndexConcurrency, workspaceIndexRebuild)
	if err != nil {
		return err
	}

	type repoSummary struct {
		Path      string `json:"path"`
		Added     int    `json:"added"`
		Updated   int    `json:"updated"`
		Unchanged int    `json:"unchanged"`
		Removed   int    `json:"removed"`
		Errors    int    `json:"errors"`
		Error     string `json:"error,omitempty"`
	}

	var summaries []repoSummary
	var failCount int

	for res := range results {
		s := repoSummary{Path: res.Repo.Path}
		if res.Err != nil {
			s.Error = res.Err.Error()
			failCount++
		} else {
			s.Added = res.Stats.Added
			s.Updated = res.Stats.Updated
			s.Unchanged = res.Stats.Unchanged
			s.Removed = res.Stats.Removed
			s.Errors = res.Stats.Errors
		}
		summaries = append(summaries, s)

		if !workspaceIndexJSON {
			if res.Err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "  error  %s: %v\n", res.Repo.Path, res.Err)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(),
					"  done   %s  added %d  updated %d  unchanged %d  removed %d  errors %d\n",
					res.Repo.Path,
					res.Stats.Added, res.Stats.Updated, res.Stats.Unchanged,
					res.Stats.Removed, res.Stats.Errors,
				)
			}
		}
	}

	if workspaceIndexJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
			"workspace":    workspaceName,
			"repositories": summaries,
		})
	}

	if failCount > 0 {
		return fmt.Errorf("%d repo(s) failed to index", failCount)
	}

	return nil
}

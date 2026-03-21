package commands

import (
	"encoding/json"
	"fmt"

	"github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

var workspaceShowJSON bool

var workspaceShowCmd = &cobra.Command{
	Use:   "show [workspace]",
	Short: "Show the repositories in a workspace",
	Long:  `Show the repositories in a workspace. If workspace is omitted the current active workspace is used.`,
	Args:  cobra.RangeArgs(0, 1),
	RunE:  runWorkspaceShow,
}

func runWorkspaceShow(cmd *cobra.Command, args []string) error {
	name, err := resolveWorkspaceName(args, 0)
	if err != nil {
		return err
	}

	db, err := workspace.OpenWorkspace(name)
	if err != nil {
		return fmt.Errorf("cannot open workspace: %w", err)
	}
	defer db.Close()

	repos, err := workspace.ListRepositories(db)
	if err != nil {
		return err
	}

	if workspaceShowJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
			"name":         name,
			"repositories": repos,
		})
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Workspace: %s\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "Repositories:\n")
	for _, repo := range repos {
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s (added %s, last indexed %s)\n", repo.ID, repo.AddedAt, repo.LastIndexed)
	}

	return nil
}

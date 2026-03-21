package commands

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

var workspaceListJSON bool

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all local workspaces",
	Long:  `List all workspaces available on the local environment. The active workspace is marked with *.`,
	Args:  cobra.NoArgs,
	RunE:  runWorkspaceList,
}

func runWorkspaceList(cmd *cobra.Command, _ []string) error {
	names, err := workspace.ListWorkspaces()
	if err != nil {
		return fmt.Errorf("cannot list workspaces: %w", err)
	}

	if workspaceListJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(names)
	}

	current, err := workspace.GetCurrentWorkspace()
	if err != nil && !errors.Is(err, workspace.ErrNoCurrentWorkspace) {
		return fmt.Errorf("cannot get current workspace: %w", err)
	}

	for _, name := range names {
		marker := "  "
		if name == current {
			marker = "* "
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s%s\n", marker, name)
	}

	return nil
}

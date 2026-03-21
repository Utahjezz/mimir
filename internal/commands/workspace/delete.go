package commands

import (
	"errors"
	"fmt"

	"github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

var workspaceDeleteConfirm bool

var workspaceDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Permanently delete a workspace",
	Long: `Permanently delete a workspace and all its repository memberships and symbol links.

This operation is irreversible. You must pass --confirm to proceed:

  mimir workspace delete myws --confirm`,
	Args: cobra.ExactArgs(1),
	RunE: runWorkspaceDelete,
}

func runWorkspaceDelete(cmd *cobra.Command, args []string) error {
	name := args[0]

	if !workspaceDeleteConfirm {
		return fmt.Errorf(
			"this will permanently delete workspace %q and all its links.\n"+
				"Re-run with --confirm to proceed:\n\n"+
				"  mimir workspace delete %s --confirm", name, name,
		)
	}

	if err := workspace.DeleteWorkspace(name); err != nil {
		if errors.Is(err, workspace.ErrWorkspaceNotFound) {
			return fmt.Errorf("workspace %q not found", name)
		}
		return fmt.Errorf("cannot delete workspace: %w", err)
	}

	// If the deleted workspace was the active one, clear the current workspace
	// so subsequent commands don't fail with a confusing "cannot open workspace" error.
	current, err := workspace.GetCurrentWorkspace()
	if err == nil && current == name {
		if err := workspace.SetCurrentWorkspace(""); err != nil {
			// Non-fatal: workspace is already gone, just warn.
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not clear active workspace: %v\n", err)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Workspace %q deleted\n", name)
	return nil
}

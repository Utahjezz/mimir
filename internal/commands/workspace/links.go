package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Utahjezz/mimir/pkg/indexer"
	"github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

var (
	workspaceLinksFrom      string
	workspaceLinksJSON      bool
	workspaceLinksSrcSymbol string
	workspaceLinksDstSymbol string
	workspaceLinksCheck     bool
)

var workspaceLinksCmd = &cobra.Command{
	Use:   "links [workspace]",
	Short: "List cross-repo symbol links in a workspace",
	Long: `List all cross-repo symbol links in a workspace.

Use --from to filter by source repository (defaults to the current directory).
If the current directory is not registered in the workspace, all links are listed.

Use --src-symbol to filter by the source symbol name (exact match).
Use --dst-symbol to filter by the destination symbol name (exact match).`,
	Args: cobra.RangeArgs(0, 1),
	RunE: runWorkspaceLinks,
}

func runWorkspaceLinks(cmd *cobra.Command, args []string) error {
	workspaceName, err := resolveWorkspaceName(args, 0)
	if err != nil {
		return err
	}

	db, err := workspace.OpenWorkspace(workspaceName)
	if err != nil {
		return fmt.Errorf("cannot open workspace: %w", err)
	}
	defer db.Close()

	// Resolve --from: explicit flag > cwd > all links.
	srcFilter := ""
	fromPath := workspaceLinksFrom
	if fromPath == "" {
		if cwd, err := os.Getwd(); err == nil {
			fromPath = cwd
		}
	}
	if fromPath != "" {
		repoID := indexer.RepoID(fromPath)
		repos, err := workspace.ListRepositories(db)
		if err != nil {
			return fmt.Errorf("cannot list repositories: %w", err)
		}
		for _, r := range repos {
			if r.ID == repoID {
				srcFilter = repoID
				break
			}
		}
		// If fromPath is not in the workspace, srcFilter stays "" → list all.
	}

	links, err := workspace.ListLinks(db, workspace.LinkQuery{
		SrcRepoID: srcFilter,
		SrcSymbol: workspaceLinksSrcSymbol,
		DstSymbol: workspaceLinksDstSymbol,
	})
	if err != nil {
		return err
	}

	// Validate links if --check flag is set.
	if workspaceLinksCheck {
		for i := range links {
			result, err := workspace.ValidateLink(db, &links[i])
			if err != nil {
				return fmt.Errorf("validation failed for link #%d: %w", links[i].ID, err)
			}
			links[i] = result.Link
		}
	}

	if workspaceLinksJSON {
		if links == nil {
			links = []workspace.Link{}
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(links)
	}

	if len(links) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No links found.")
		return nil
	}

	brokenCount := 0
	for _, l := range links {
		fmt.Fprintf(cmd.OutOrStdout(), "#%-4d  %s (%s)\n       → %s (%s)\n",
			l.ID,
			l.SrcSymbol, l.SrcRepoID,
			l.DstSymbol, l.DstRepoID,
		)
		if l.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "       note: %s\n", l.Note)
		}
		for k, v := range l.Meta {
			fmt.Fprintf(cmd.OutOrStdout(), "       %s=%s\n", k, v)
		}
		if workspaceLinksCheck {
			srcBroken := (l.SrcError != nil && *l.SrcError != "") ||
				(l.SrcFileValid != nil && !*l.SrcFileValid)
			dstBroken := (l.DstError != nil && *l.DstError != "") ||
				(l.DstFileValid != nil && !*l.DstFileValid)

			if l.SrcError != nil && *l.SrcError != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "       [CHECK] src: %s\n", *l.SrcError)
			} else if l.SrcFileValid != nil && !*l.SrcFileValid {
				fmt.Fprintf(cmd.OutOrStdout(), "       [CHECK] src: moved from %s → %s\n", l.SrcFile, strDeref(l.SrcActualFile))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "       [CHECK] src: OK (%s)\n", l.SrcFile)
			}
			if l.DstError != nil && *l.DstError != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "       [CHECK] dst: %s\n", *l.DstError)
			} else if l.DstFileValid != nil && !*l.DstFileValid {
				fmt.Fprintf(cmd.OutOrStdout(), "       [CHECK] dst: moved from %s → %s\n", l.DstFile, strDeref(l.DstActualFile))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "       [CHECK] dst: OK (%s)\n", l.DstFile)
			}

			if srcBroken || dstBroken {
				brokenCount++
			}
		}
	}

	if workspaceLinksCheck && brokenCount > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\n⚠ %d broken link(s) found. Run `mimir workspace unlink <id>` to remove.\n", brokenCount)
	}

	return nil
}

// strDeref safely dereferences a *string, returning "" if nil.
func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

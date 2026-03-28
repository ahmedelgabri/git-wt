package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/ui"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:     "update",
	Aliases: []string{"u"},
	Short:   "Fetch and update the default branch worktree",
	Long: `Fetch all remotes (with prune) and pull the default branch (main/master)
in its worktree.`,
	Example: `  git wt update
  git wt u`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var defaultBranch string
		var entryPath string

		if err := ui.RunSteps([]ui.Step{{
			Message:    "Fetching from all remotes",
			ShowOutput: true,
			Run: func(ctx context.Context, w io.Writer) error {
				return git.RunToContext(ctx, w, "fetch", "--all", "--prune", "--prune-tags")
			},
		}, {
			Message: "Resolving default branch worktree",
			Run: func(context.Context, io.Writer) error {
				remote := worktree.DefaultRemote()
				defaultBranch = worktree.DefaultBranch(remote)
				if defaultBranch == "" {
					ui.Error("Could not determine default branch from remote")
					return fmt.Errorf("could not determine default branch")
				}

				entries, err := worktree.List()
				if err != nil {
					return err
				}

				entry := worktree.FindByBranch(entries, defaultBranch)
				if entry == nil {
					ui.Errorf("No worktree found for default branch '%s'", defaultBranch)
					fmt.Fprintln(os.Stderr, "Available worktrees:")
					git.Run("worktree", "list")
					return fmt.Errorf("no worktree for default branch '%s'", defaultBranch)
				}
				entryPath = entry.Path
				return nil
			},
		}}); err != nil {
			return err
		}

		return ui.RunSteps([]ui.Step{{
			Message:    fmt.Sprintf("Updating %s in %s", ui.Accent(defaultBranch), ui.Muted(entryPath)),
			ShowOutput: true,
			Run: func(ctx context.Context, w io.Writer) error {
				return git.RunInToContext(ctx, entryPath, w, "pull")
			},
		}})
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

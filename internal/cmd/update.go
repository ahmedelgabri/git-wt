package cmd

import (
	"fmt"
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
	Long: `git wt update - Fetch and update the default branch worktree

Usage:
  git wt update                            Fetch all and pull default branch
  git wt u                                 Alias for update

Options:
  --help, -h      Show this help message

Example:
  git wt update                            # Fetch all, then pull main/master`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Info("Fetching from all remotes...")
		if err := git.Run("fetch", "--all", "--prune", "--prune-tags"); err != nil {
			return err
		}

		defaultBranch := worktree.DefaultBranch()
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

		ui.Infof("Updating default branch '%s' in %s...", defaultBranch, entry.Path)
		return git.RunIn(entry.Path, "pull")
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

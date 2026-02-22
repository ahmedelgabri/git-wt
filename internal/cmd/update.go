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
	Long: `Fetch all remotes (with prune) and pull the default branch (main/master)
in its worktree.`,
	Example: `  git wt update
  git wt u`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ui.Spin("Fetching from all remotes", func() error {
			_, err := git.RunWithOutput("fetch", "--all", "--prune", "--prune-tags")
			return err
		}); err != nil {
			return err
		}

		remote := worktree.DefaultRemote()
		defaultBranch := worktree.DefaultBranch(remote)
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

		fmt.Printf("Updating %s in %s\n", ui.Accent(defaultBranch), ui.Muted(entry.Path))
		return git.RunIn(entry.Path, "pull")
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

package cmd

import (
	"github.com/spf13/cobra"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy [<worktree>...]",
	Short: "Remove worktree(s) and delete LOCAL and REMOTE branch(es)",
	Long: `git wt destroy - Remove worktree(s) and delete LOCAL and REMOTE branch(es)

Usage:
  git wt destroy                           Interactive mode (fzf)
  git wt destroy <path> [<path>...]        Destroy specific worktree(s)
  git wt destroy --dry-run                 Preview without changes

Options:
  --dry-run, -n   Show what would be destroyed without making changes
  --help, -h      Show this help message

Examples:
  git wt destroy                           # Interactive selection
  git wt destroy feature-1 feature-2       # Destroy multiple
  git wt destroy --dry-run                 # Preview in interactive mode
  git wt destroy -n feature-1 feature-2    # Preview specific worktrees

Warning: This deletes both local AND remote branches permanently.`,
	DisableFlagParsing: true,
	SilenceUsage:       true,
	SilenceErrors:      true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRemoveOrDestroy(cmd, args, "destroy")
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)
}

package cmd

import (
	"github.com/spf13/cobra"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy [<worktree>...]",
	Short: "Remove worktree(s) and delete LOCAL and REMOTE branch(es)",
	Long: `Remove worktree(s) and delete both LOCAL and REMOTE branch(es). With no
arguments, opens an interactive picker with multi-select (TAB to toggle). Supports
--dry-run (-n) to preview without changes. Requires confirmation before destroying.`,
	Example: `  git wt destroy                           # Interactive selection
  git wt destroy feature-1 feature-2       # Destroy multiple
  git wt destroy -n feature-1 feature-2    # Preview specific worktrees`,
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

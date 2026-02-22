package cmd

import (
	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all worktrees",
	Long: `List all worktrees. This is a pass-through to 'git worktree list', so all
git worktree list flags are supported (e.g. --porcelain).`,
	FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	SilenceUsage:       true,
	SilenceErrors:      true,
	RunE: func(cmd *cobra.Command, args []string) error {
		fullArgs := append([]string{"worktree", "list"}, args...)
		return git.Run(fullArgs...)
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

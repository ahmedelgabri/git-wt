package cmd

import (
	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all worktrees",
	Long: `git wt list - List all worktrees

Usage:
  git wt list                              List all worktrees

Options:
  --help, -h      Show this help message

Note: This is a wrapper around 'git worktree list'`,
	DisableFlagParsing: true,
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

package cmd

import (
	"fmt"
	"os"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "git-wt",
	Short: "Git worktree management tool",
	Long: `Git worktree management using the bare repository pattern.

Uses a .bare/ directory for git data with each branch in its own worktree
directory. Run 'git-wt <command> --help' for details on any command.

Native git worktree commands (list, lock, unlock, move, prune, repair) are
also supported as pass-throughs.`,
	// Don't show usage on errors from subcommands
	SilenceUsage: true,
	// We handle error formatting ourselves
	SilenceErrors: true,
	// When called with no subcommand, print help
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	rootCmd.Version = Version

	// Disable default completion command - we generate completions separately
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Pass-through commands to git worktree
	for _, name := range []string{"lock", "unlock", "move", "prune", "repair"} {
		name := name
		rootCmd.AddCommand(&cobra.Command{
			Use:                name,
			Short:              fmt.Sprintf("Pass-through to git worktree %s", name),
			FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
			SilenceUsage:       true,
			SilenceErrors:      true,
			RunE: func(cmd *cobra.Command, args []string) error {
				fullArgs := append([]string{"worktree", name}, args...)
				return git.Run(fullArgs...)
			},
		})
	}
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// Only pass through to git worktree for unknown subcommands.
		// Check if the error is an "unknown command" error by seeing if the
		// first arg matches any registered command.
		if args := os.Args[1:]; len(args) > 0 && !isKnownCommand(args[0]) {
			passErr := git.Run(append([]string{"worktree"}, args...)...)
			if passErr != nil {
				os.Exit(1)
			}
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func isKnownCommand(name string) bool {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == name {
			return true
		}
		for _, alias := range cmd.Aliases {
			if alias == name {
				return true
			}
		}
	}
	// Also check built-in names
	return name == "help" || name == "--help" || name == "-h"
}

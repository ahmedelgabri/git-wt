package cmd

import (
	"fmt"
	"os"
	"slices"
	"sync"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

const (
	supportedCommandGroup   = "supported"
	passthroughCommandGroup = "passthrough"
)

var (
	configureRootCommandOnce sync.Once
	supportedCommandNames    = []string{"add", "agent-skill", "clone", "doctor", "migrate", "remove", "status", "switch", "update"}
	passthroughCommandNames  = []string{"list", "lock", "unlock", "move", "prune", "repair"}
)

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
	for _, name := range passthroughCommandNames {
		name := name
		rootCmd.AddCommand(&cobra.Command{
			Use:                name,
			Short:              fmt.Sprintf("Pass-through to git worktree %s", name),
			FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
			SilenceUsage:       true,
			SilenceErrors:      true,
			RunE: func(cmd *cobra.Command, args []string) error {
				return runWorktreePassthrough(name, args)
			},
		})
	}
}

func runWorktreePassthrough(name string, args []string) error {
	rawArgs := args
	if len(os.Args) > 2 {
		rawArgs = os.Args[2:]
	}
	fullArgs := append([]string{"worktree", name}, rawArgs...)
	if name == "list" {
		return git.QueryRun(fullArgs...)
	}
	return git.Run(fullArgs...)
}

func configureRootCommand() {
	configureRootCommandOnce.Do(func() {
		rootCmd.AddGroup(
			&cobra.Group{ID: supportedCommandGroup, Title: "Supported commands:"},
			&cobra.Group{ID: passthroughCommandGroup, Title: "Passthrough:"},
		)
		rootCmd.SetHelpCommandGroupID(supportedCommandGroup)

		for _, cmd := range rootCmd.Commands() {
			switch {
			case slices.Contains(supportedCommandNames, cmd.Name()):
				cmd.GroupID = supportedCommandGroup
			case slices.Contains(passthroughCommandNames, cmd.Name()):
				cmd.GroupID = passthroughCommandGroup
			}
		}
	})
}

// Execute runs the root command.
func Execute() {
	configureRootCommand()
	os.Args = normalizeLegacyAliases(os.Args)

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

func normalizeLegacyAliases(args []string) []string {
	if len(args) < 2 {
		return args
	}

	switch args[1] {
	case "destroy":
		normalized := []string{args[0], "remove", "--delete-remote"}
		return append(normalized, args[2:]...)
	default:
		return args
	}
}

func isKnownCommand(name string) bool {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == name {
			return true
		}

		if slices.Contains(cmd.Aliases, name) {
			return true
		}
	}
	// Also check built-in names
	return name == "help" || name == "--help" || name == "-h"
}

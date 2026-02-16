package cmd

import (
	"fmt"
	"os"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "git-wt",
	Short: "Git worktree management tool",
	Long: `  Run: git worktree --help for worktree help
  -------------------------------------------------------------------------------
    Additional commands (use --help on each for details):

    git-wt <command> --help                      Show help for a specific command

    git-wt clone <repo> [folder-name]
        Clone a repository and set it up with worktree structure
        Creates .bare directory for git data and initial worktree for default branch

    git-wt migrate [EXPERIMENTAL]
        Migrate an existing repository to use worktrees
        Converts current repo to .bare + worktrees structure
        Preserves: uncommitted changes, staged changes, untracked files, and stashes
        Must be run from within the repository to migrate

    git-wt add [options] [<path>] [<commit-ish>]
        Create a new worktree (enhanced wrapper around git worktree add)
        Always fetches latest refs from origin before creating worktree
        Supports ALL git worktree add flags (run 'git wt add --help' for full list)

        Interactive mode (no arguments):
          git wt add
              Opens fzf to select from remote branches or create new branch
              - Fetches latest remote refs first
              - Shows git log preview for each remote branch
              - Option to create new branch with custom name and path
              - Automatically sets up upstream tracking

        Common flags:
          -b <branch>     Create new branch
          -B <branch>     Create/reset branch
          -d, --detach    Detach HEAD at named commit
          --lock          Keep the new working tree locked
          -q, --quiet     Suppress progress reporting

        Upstream tracking:
          - Automatically sets upstream when using -b if branch exists on origin
          - Always sets upstream in interactive mode
          - Provides helpful push instructions for new local branches

        Examples:
          git wt add
              Interactive mode - select branch with fzf

          git wt add feature origin/feature
              Create worktree from remote branch

          git wt add -b new-feature new-feature
              Create new branch and worktree

          git wt add existing-branch
              Create worktree from existing local branch

    git-wt remove|rm [options] [<worktree>...]
        Remove worktree(s) and delete local branch(es)
        Does NOT delete branches from remote

        Options:
          --dry-run, -n    Show what would be removed without making changes

        Interactive mode (no arguments):
          git wt remove
              Opens fzf to select worktree(s) to remove
              - Use TAB to select multiple worktrees
              - Shows git status and log preview
              - Lists all selected worktrees before confirmation
              - Shows progress for each worktree
              - Removes worktrees and deletes local branches only

        Direct mode (with arguments):
          git wt remove <worktree-path> [<worktree-path>...]
              Removes specified worktree(s) and deletes local branch(es)
              Supports multiple worktrees as arguments

        Examples:
          git wt remove --dry-run
              Preview what would be removed (interactive)

          git wt remove -n feature-1 feature-2
              Preview removing multiple worktrees (direct)

    git-wt destroy [options] [<worktree>...]
        Remove worktree(s), delete local branch(es), AND delete from remote
        Use this when you're completely done with feature branch(es)

        Options:
          --dry-run, -n    Show what would be destroyed without making changes

        Interactive mode (no arguments):
          git wt destroy
              Opens fzf to select worktree(s) to destroy
              - Use TAB to select multiple worktrees
              - Shows warning about permanent deletion
              - Shows git status and log preview
              - Single selection: requires typing exact branch name
              - Multiple selection: requires typing 'destroy'
              - Shows progress for each worktree
              - Removes worktrees, deletes local and remote branches

        Direct mode (with arguments):
          git wt destroy <worktree-path> [<worktree-path>...]
              Destroys specified worktree(s)
              Prompts for confirmation before destroying
              Supports multiple worktrees as arguments

        Examples:
          git wt destroy --dry-run
              Preview what would be destroyed (interactive)

          git wt destroy -n feature-1 feature-2
              Preview destroying multiple worktrees with full details

    git-wt update|u
        Fetch all remotes and update the default branch worktree
        Runs: git fetch --all --prune --prune-tags
        Then pulls the default branch (main/master) in its worktree

        Example:
          git wt update                            # Fetch and pull default branch
          git wt u                                 # Alias for update

    git-wt switch
        Interactively select a worktree and print its path
        Use with cd: cd $(git wt switch)
        Shows git status and log preview for each worktree

        Example:
          cd $(git wt switch)                      # Select and change to worktree

    git-wt list
        List all worktrees (wrapper around git worktree list)`,
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
	// Disable default completion command - we generate completions separately
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Pass-through commands to git worktree
	for _, name := range []string{"lock", "unlock", "move", "prune", "repair"} {
		name := name
		rootCmd.AddCommand(&cobra.Command{
			Use:                name,
			Short:              fmt.Sprintf("Pass-through to git worktree %s", name),
			DisableFlagParsing: true,
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

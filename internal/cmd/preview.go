package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/ui"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
	"github.com/spf13/cobra"
)

// previewCmd is a hidden command used by fzf --preview to generate preview
// content. It is not intended for direct user invocation.
var previewCmd = &cobra.Command{
	Use:    "_preview",
	Hidden: true,
}

var previewWorktreeCmd = &cobra.Command{
	Use:           "worktree <path> [mode]",
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		wtPath := args[0]
		mode := "remove"
		if len(args) > 1 {
			mode = args[1]
		}
		fmt.Print(generateWorktreePreview(wtPath, mode))
		return nil
	},
}

var previewBranchCmd = &cobra.Command{
	Use:           "branch <name>",
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]
		if branch == "__create_new__" {
			fmt.Print("Create a new branch and worktree\n\nYou will be prompted to enter:\n  - Branch name\n  - Worktree path (optional, defaults to branch name)")
			return nil
		}
		out, _ := git.Query("log", "--oneline", "--graph", "--date=short",
			"--color=always", "--pretty=format:%C(auto)%cd %h%d %s",
			"origin/"+branch, "-10", "--")
		fmt.Printf("Branch: %s\n\nRecent commits:\n%s", branch, out)
		return nil
	},
}

func init() {
	previewCmd.AddCommand(previewWorktreeCmd)
	previewCmd.AddCommand(previewBranchCmd)
	rootCmd.AddCommand(previewCmd)
}

func generateWorktreePreview(wtPath string, mode string) string {
	var b strings.Builder

	if mode == "destroy" {
		b.WriteString(ui.Bold(ui.Red("DESTROY MODE")) + "\n\n")
	}

	b.WriteString(ui.Bold(ui.Accent("Worktree")) + "\n")
	b.WriteString(fmt.Sprintf("  %s %s\n", ui.Subtle("Path:"), wtPath))

	entries, _ := worktree.List()
	branch := worktree.BranchFor(entries, wtPath)
	if branch != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", ui.Subtle("Branch:"), branch))
	}

	if mode == "destroy" {
		b.WriteString("\n")
		b.WriteString(ui.Yellow("  - Remove worktree directory") + "\n")
		b.WriteString(ui.Yellow("  - Delete local branch") + "\n")
		b.WriteString(ui.Yellow(fmt.Sprintf("  - Delete remote branch (origin/%s)", branch)) + "\n")
	}

	b.WriteString("\n" + ui.Bold(ui.Accent("Status")) + "\n")
	status, err := git.QueryIn(wtPath, "status", "--short", "--branch")
	if err != nil {
		b.WriteString("  (unable to get status)\n")
	} else {
		for _, line := range strings.Split(status, "\n") {
			b.WriteString("  " + line + "\n")
		}
	}

	b.WriteString("\n" + ui.Bold(ui.Accent("Recent Commits")) + "\n")
	if branch != "" {
		log, err := git.Query("log", "--oneline", "--graph", "--date=short",
			"--color=always", "--pretty=format:%C(auto)%cd %h%d %s", branch, "-10", "--")
		if err != nil {
			b.WriteString("  (unable to get log)\n")
		} else {
			for _, line := range strings.Split(log, "\n") {
				b.WriteString("  " + line + "\n")
			}
		}
	}

	return b.String()
}

// previewWorktreeCmdStr returns the fzf --preview command string for worktree
// previews. {1} is replaced by fzf with the first tab-delimited field (the
// worktree path).
func previewWorktreeCmdStr(mode string) string {
	exe, _ := os.Executable()
	return fmt.Sprintf("%s _preview worktree {1} %s", exe, mode)
}

// previewBranchCmdStr returns the fzf --preview command string for branch
// previews. {1} is replaced by fzf with the first tab-delimited field (the
// branch name).
func previewBranchCmdStr() string {
	exe, _ := os.Executable()
	return fmt.Sprintf("%s _preview branch {1}", exe)
}

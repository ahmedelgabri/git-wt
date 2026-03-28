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

const (
	createNewBranchValue    = "__create_new__"
	previewModeRemove       = "remove"
	previewModeDeleteRemote = "remove-remote"
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
		mode := previewModeRemove
		if len(args) > 1 {
			mode = args[1]
		}
		fmt.Print(generateWorktreePreview(wtPath, mode))
		return nil
	},
}

var previewBranchCmd = &cobra.Command{
	Use:           "branch <remote-ref>",
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		remoteRef := args[0]
		if remoteRef == createNewBranchValue {
			fmt.Print("Create a new branch and worktree\n\nYou will be prompted to enter:\n  - Branch name\n  - Worktree path (optional, defaults to branch name)")
			return nil
		}

		remote, branch := splitRemoteBranchRef(remoteRef)
		out, _ := git.Query("log", "--oneline", "--graph", "--date=short",
			previewGitColorArg(), "--pretty=format:%C(auto)%cd %h%d %s",
			remoteRef, "-10", "--")

		if branch == "" {
			fmt.Printf("Reference: %s\n\nRecent commits:\n%s", remoteRef, out)
			return nil
		}

		fmt.Printf("Branch: %s\nRemote: %s\n\nRecent commits:\n%s", branch, remote, out)
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

	b.WriteString(ui.Bold(ui.Accent("Worktree")) + "\n")
	b.WriteString(fmt.Sprintf("  %s %s\n", ui.Subtle("Path:"), wtPath))

	entries, _ := worktree.List()
	entry := worktree.FindByPath(entries, wtPath)
	if entry != nil {
		switch {
		case entry.Detached:
			b.WriteString(fmt.Sprintf("  %s %s\n", ui.Subtle("Branch:"), "detached HEAD"))
		case entry.Branch != "":
			b.WriteString(fmt.Sprintf("  %s %s\n", ui.Subtle("Branch:"), entry.Branch))
		}
	}

	remote := worktree.DefaultRemote()

	if mode == previewModeDeleteRemote {
		b.WriteString("\n" + ui.Bold(ui.Accent("Actions")) + "\n")
		b.WriteString(ui.Yellow("  - Remove worktree directory") + "\n")
		if entry == nil || entry.Detached || entry.Branch == "" {
			b.WriteString(ui.Yellow("  - Detached HEAD: no local or remote branch deletion") + "\n")
		} else {
			b.WriteString(ui.Yellow("  - Delete local branch") + "\n")
			if remote != "" {
				b.WriteString(ui.Yellow(fmt.Sprintf("  - Delete remote branch (%s/%s)", remote, entry.Branch)) + "\n")
			} else {
				b.WriteString(ui.Yellow("  - No remote configured; remote branch deletion skipped") + "\n")
			}
		}
	}

	b.WriteString("\n" + ui.Bold(ui.Accent("Status")) + "\n")
	status, err := git.QueryIn(wtPath, "status", "--short", "--branch")
	if err != nil {
		b.WriteString("  (unable to get status)\n")
	} else {
		for line := range strings.SplitSeq(status, "\n") {
			b.WriteString("  " + line + "\n")
		}
	}

	b.WriteString("\n" + ui.Bold(ui.Accent("Recent Commits")) + "\n")
	log, err := git.QueryIn(wtPath, "log", "--oneline", "--graph", "--date=short",
		previewGitColorArg(), "--pretty=format:%C(auto)%cd %h%d %s", "HEAD", "-10", "--")
	if err != nil {
		b.WriteString("  (unable to get log)\n")
	} else {
		for line := range strings.SplitSeq(log, "\n") {
			b.WriteString("  " + line + "\n")
		}
	}

	return b.String()
}

// previewWorktreeCmdStr returns the fzf --preview command string for worktree
// previews. {1} is replaced by fzf with the first tab-delimited field (the
// worktree path).
func previewWorktreeCmdStr(mode string) string {
	exe, _ := os.Executable()
	return fmt.Sprintf("sh -c 'exec \"$1\" _preview worktree \"$2\" \"$3\"' sh %s {1} %s", shellQuote(exe), shellQuote(mode))
}

// previewBranchCmdStr returns the fzf --preview command string for branch
// previews. {1} is replaced by fzf with the first tab-delimited field (the
// remote ref).
func previewBranchCmdStr() string {
	exe, _ := os.Executable()
	return fmt.Sprintf("sh -c 'exec \"$1\" _preview branch \"$2\"' sh %s {1}", shellQuote(exe))
}

func previewGitColorArg() string {
	if ui.NoColor() {
		return "--color=never"
	}
	return "--color=always"
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

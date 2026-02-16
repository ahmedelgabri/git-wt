package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/picker"
	"github.com/ahmedelgabri/git-wt/internal/ui"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove [<worktree>...]",
	Aliases: []string{"rm"},
	Short:   "Remove worktree(s) and delete local branch(es)",
	Long: `git wt remove - Remove worktree(s) and delete local branch(es)

Usage:
  git wt remove                            Interactive mode (fzf)
  git wt remove <path> [<path>...]         Remove specific worktree(s)
  git wt remove --dry-run                  Preview without changes

Options:
  --dry-run, -n   Show what would be removed without making changes
  --help, -h      Show this help message

Examples:
  git wt remove                            # Interactive selection
  git wt remove feature-1 feature-2        # Remove multiple
  git wt remove --dry-run                  # Preview in interactive mode
  git wt remove -n feature-1 feature-2     # Preview specific worktrees

Note: Remote branches are NOT deleted. Use 'destroy' for that.`,
	DisableFlagParsing: true,
	SilenceUsage:       true,
	SilenceErrors:      true,
	RunE:               runRemove,
}

func init() {
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	return runRemoveOrDestroy(cmd, args, "remove")
}

// runRemoveOrDestroy handles both remove and destroy commands since they share
// most of their logic.
func runRemoveOrDestroy(cmd *cobra.Command, args []string, mode string) error {
	// Parse flags
	dryRun := false
	var worktreeArgs []string

	for _, a := range args {
		switch a {
		case "--help", "-h":
			return cmd.Help()
		case "--dry-run", "-n":
			dryRun = true
		default:
			worktreeArgs = append(worktreeArgs, a)
		}
	}

	entries, err := worktree.List()
	if err != nil {
		return err
	}

	// Interactive mode
	if len(worktreeArgs) == 0 {
		return removeInteractive(entries, mode, dryRun)
	}

	// Non-interactive mode
	return removeNonInteractive(entries, worktreeArgs, mode, dryRun)
}

func removeInteractive(entries []worktree.Entry, mode string, dryRun bool) error {
	if len(entries) == 0 {
		fmt.Printf("No worktrees to %s\n", mode)
		return nil
	}

	items := entriesToPickerItems(entries)
	prompt := fmt.Sprintf("Select worktree(s) to %s (TAB to select multiple): ", mode)
	header := "TAB: select/deselect | ENTER: confirm | ESC: cancel\nLocal branches will also be deleted (remote branches preserved)"
	if mode == "destroy" {
		prompt = "Select worktree(s) to DESTROY (TAB to select multiple): "
		header = "WARNING: This will delete LOCAL and REMOTE branches\nTAB: select/deselect | ENTER: confirm | ESC: cancel"
	}

	result, err := picker.Run(picker.Config{
		Items:  items,
		Multi:  true,
		Prompt: prompt,
		Header: header,
		PreviewFunc: func(item picker.Item) string {
			return generateWorktreePreview(item, mode)
		},
	})
	if err != nil {
		return err
	}
	if result.Canceled || len(result.Items) == 0 {
		return nil
	}

	// Resolve selected items to paths and branches
	var targets []removalTarget
	for _, item := range result.Items {
		t := removalTarget{path: item.Value}
		t.branch = worktree.BranchFor(entries, item.Value)
		targets = append(targets, t)
	}

	// Show what will be removed
	fmt.Println()
	if dryRun {
		if mode == "destroy" {
			fmt.Printf("[DRY RUN] Would DESTROY %d worktree(s):\n", len(targets))
		} else {
			fmt.Printf("[DRY RUN] Would remove %d worktree(s):\n", len(targets))
		}
	} else if mode == "destroy" {
		fmt.Printf("%s %s\n\n", ui.Red("WARNING: DESTRUCTIVE OPERATION"), "")
		fmt.Printf("About to DESTROY %d worktree(s):\n", len(targets))
	} else {
		ui.Infof("About to remove %d worktree(s):", len(targets))
	}

	for i, t := range targets {
		fmt.Printf("  [%d] %s (branch: %s)\n", i+1, t.path, t.branch)
	}

	if mode == "destroy" {
		fmt.Printf("\nThis will:\n  - Remove worktree directories\n  - Delete local branches\n  - Delete remote branches (origin/<branch>)\n\n")
	} else {
		fmt.Println()
		fmt.Println("Note: Remote branches will NOT be deleted")
		fmt.Println()
	}

	if dryRun {
		fmt.Println("[DRY RUN] No changes made")
		return nil
	}

	// Confirmation
	if mode == "destroy" {
		fmt.Println("This action CANNOT be undone.")
		fmt.Println()
		if len(targets) == 1 {
			fmt.Printf("%s ", ui.Yellow("Type the branch name to confirm:"))
			reader := bufio.NewReader(os.Stdin)
			confirm, _ := reader.ReadString('\n')
			confirm = strings.TrimSpace(confirm)
			if confirm != targets[0].branch {
				fmt.Println("Cancelled (confirmation did not match branch name)")
				return nil
			}
		} else {
			fmt.Printf("%s ", ui.Yellow("Type 'destroy' to confirm:"))
			reader := bufio.NewReader(os.Stdin)
			confirm, _ := reader.ReadString('\n')
			confirm = strings.TrimSpace(confirm)
			if confirm != "destroy" {
				fmt.Println("Cancelled (must type 'destroy' to confirm)")
				return nil
			}
		}
	} else {
		fmt.Printf("%s ", ui.Yellow("Continue? [y/N]:"))
		reader := bufio.NewReader(os.Stdin)
		confirm, _ := reader.ReadString('\n')
		confirm = strings.TrimSpace(confirm)
		if confirm != "y" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Execute removal
	fmt.Println()
	return executeRemoval(targets, mode)
}

func removeNonInteractive(entries []worktree.Entry, args []string, mode string, dryRun bool) error {
	// Validate all worktree paths first
	var targets []removalTarget

	for _, arg := range args {
		if err := worktree.Validate(entries, arg); err != nil {
			ui.Error(err.Error())
			return fmt.Errorf("invalid worktree")
		}
		resolved, _ := worktree.Resolve(entries, arg)
		branch := worktree.BranchFor(entries, resolved)
		targets = append(targets, removalTarget{path: resolved, branch: branch})
	}

	// Dry run
	if dryRun {
		if mode == "destroy" {
			fmt.Printf("[DRY RUN] Would DESTROY %d worktree(s):\n", len(targets))
			for _, t := range targets {
				if t.branch != "" {
					fmt.Printf("  - %s (branch: %s)\n", t.path, t.branch)
					fmt.Printf("    - Remove worktree directory\n")
					fmt.Printf("    - Delete local branch: %s\n", t.branch)
					fmt.Printf("    - Delete remote branch: origin/%s\n", t.branch)
				} else {
					fmt.Printf("  - %s\n", t.path)
				}
			}
			fmt.Println()
			fmt.Println("[DRY RUN] No changes made")
		} else {
			fmt.Printf("[DRY RUN] Would remove %d worktree(s):\n", len(targets))
			for _, t := range targets {
				if t.branch != "" {
					fmt.Printf("  - %s (branch: %s)\n", t.path, t.branch)
				} else {
					fmt.Printf("  - %s\n", t.path)
				}
			}
			fmt.Println()
			fmt.Println("[DRY RUN] No changes made")
		}
		return nil
	}

	// Confirmation for destroy mode
	if mode == "destroy" {
		firstBranch := targets[0].branch
		extraMsg := ""
		if firstBranch != "" {
			extraMsg = fmt.Sprintf(" and delete its remote branch [%s]", firstBranch)
		}
		fmt.Printf("%s ", ui.Yellow(fmt.Sprintf("Are you sure you want to destroy '%s' workspace%s?",
			filepath.Base(targets[0].path), extraMsg)))
		if len(targets) > 1 {
			fmt.Printf("(and %d more) ", len(targets)-1)
		}
		fmt.Printf("[y/N]: ")

		reader := bufio.NewReader(os.Stdin)
		confirm, _ := reader.ReadString('\n')
		confirm = strings.TrimSpace(confirm)
		if confirm != "y" {
			fmt.Println("Cancelled")
			return fmt.Errorf("cancelled")
		}
	}

	return executeRemoval(targets, mode)
}

type removalTarget struct {
	path   string
	branch string
}

func executeRemoval(targets []removalTarget, mode string) error {
	successCount := 0
	failedCount := 0

	for i, t := range targets {
		if len(targets) > 1 {
			if mode == "destroy" {
				ui.Infof("[%d/%d] Destroying %s...", i+1, len(targets), t.path)
			} else {
				ui.Infof("[%d/%d] Removing %s...", i+1, len(targets), t.path)
			}
		}

		if err := removeSingleWorktree(t.path, t.branch, mode, len(targets) > 1); err != nil {
			failedCount++
		} else {
			successCount++
		}
	}

	if len(targets) > 1 {
		fmt.Println()
		fmt.Printf("Summary: %s, %s\n", ui.Green(fmt.Sprintf("%d succeeded", successCount)),
			ui.Red(fmt.Sprintf("%d failed", failedCount)))
	}

	if failedCount > 0 {
		return fmt.Errorf("%d removal(s) failed", failedCount)
	}
	return nil
}

func removeSingleWorktree(wtPath, branch, mode string, showPrefix bool) error {
	actionVerb := "Removed"
	if mode == "destroy" {
		actionVerb = "Destroyed"
	}

	prefix := ""
	if showPrefix {
		prefix = "  "
	}

	if err := git.Run("worktree", "remove", "-f", wtPath); err != nil {
		fmt.Printf("%s%s\n", prefix, ui.Red(fmt.Sprintf("Failed to %s worktree '%s'", strings.ToLower(actionVerb[:len(actionVerb)-1])+"e", wtPath)))
		return err
	}

	if branch == "" {
		fmt.Printf("%s%s\n", prefix, ui.Green(fmt.Sprintf("%s worktree '%s'", actionVerb, wtPath)))
		return nil
	}

	// Delete local branch
	git.Run("branch", "-D", branch)

	if mode == "destroy" {
		remoteStatus := deleteRemoteBranch(branch)
		fmt.Printf("%s%s\n", prefix, ui.Green(fmt.Sprintf("%s worktree '%s' and deleted branch '%s' (%s)", actionVerb, wtPath, branch, remoteStatus)))
	} else {
		fmt.Printf("%s%s\n", prefix, ui.Green(fmt.Sprintf("%s worktree '%s' and deleted local branch '%s'", actionVerb, wtPath, branch)))
	}

	return nil
}

func deleteRemoteBranch(branch string) string {
	// Check if remote branch exists
	if _, err := git.Query("ls-remote", "--exit-code", "--heads", "origin", branch); err != nil {
		return "no remote branch"
	}

	if err := git.Run("push", "origin", "--delete", branch); err != nil {
		return "remote deletion failed"
	}
	return "local and remote"
}

func entriesToPickerItems(entries []worktree.Entry) []picker.Item {
	items := make([]picker.Item, len(entries))
	for i, e := range entries {
		label := filepath.Base(e.Path)
		if e.Branch != "" && e.Branch != "(detached)" {
			label = fmt.Sprintf("%s [%s]", filepath.Base(e.Path), e.Branch)
		} else if e.Branch == "(detached)" {
			label = fmt.Sprintf("%s (detached HEAD)", filepath.Base(e.Path))
		}

		homeDir, _ := os.UserHomeDir()
		displayPath := e.Path
		if homeDir != "" {
			displayPath = strings.Replace(displayPath, homeDir, "~", 1)
		}

		items[i] = picker.Item{
			Label: label,
			Value: e.Path,
			Desc:  displayPath,
		}
	}
	return items
}

func generateWorktreePreview(item picker.Item, mode string) string {
	var b strings.Builder

	if mode == "destroy" {
		b.WriteString("DESTROY MODE - PERMANENT DELETION\n\n")
	}

	b.WriteString(fmt.Sprintf("Worktree: %s\n", item.Value))

	// Try to get branch from value
	entries, _ := worktree.List()
	branch := worktree.BranchFor(entries, item.Value)
	if branch != "" {
		b.WriteString(fmt.Sprintf("Branch: %s\n", branch))
	}

	if mode == "destroy" {
		b.WriteString(fmt.Sprintf("\nThis will:\n  1. Remove worktree directory\n  2. Delete local branch\n  3. Delete remote branch (origin/%s)\n", branch))
	}

	b.WriteString("\nStatus:\n")
	status, err := git.QueryIn(item.Value, "status", "--short", "--branch")
	if err != nil {
		b.WriteString("(unable to get status)\n")
	} else {
		b.WriteString(status + "\n")
	}

	b.WriteString("\nRecent commits:\n")
	if branch != "" {
		log, err := git.Query("log", "--oneline", "--graph", "--date=short",
			"--color=always", "--pretty=format:%C(auto)%cd %h%d %s", branch, "-10", "--")
		if err != nil {
			b.WriteString("(unable to get log)\n")
		} else {
			b.WriteString(log + "\n")
		}
	}

	return b.String()
}

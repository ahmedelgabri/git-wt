package cmd

import (
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
	Long: `Remove worktree(s) and delete local branch(es). With no arguments, opens
an interactive picker with multi-select (TAB to toggle). Remote branches are NOT
deleted; use 'destroy' for that.`,
	Example: `  git wt remove                            # Interactive selection
  git wt remove feature-1 feature-2        # Remove multiple
  git wt remove -n feature-1 feature-2     # Preview specific worktrees`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runRemove,
}

func init() {
	removeCmd.Flags().BoolP("dry-run", "n", false, "Preview what would be removed without making changes")
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	return runRemoveOrDestroy(cmd, args, "remove")
}

// runRemoveOrDestroy handles both remove and destroy commands since they share
// most of their logic.
func runRemoveOrDestroy(cmd *cobra.Command, args []string, mode string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	entries, err := worktree.List()
	if err != nil {
		return err
	}

	// Interactive mode
	if len(args) == 0 {
		return removeInteractive(entries, mode, dryRun)
	}

	// Non-interactive mode
	return removeNonInteractive(entries, args, mode, dryRun)
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
			if !ui.PromptDangerous("Type the branch name to confirm:", targets[0].branch) {
				fmt.Println("Cancelled (confirmation did not match branch name)")
				return nil
			}
		} else {
			if !ui.PromptDangerous("Type 'destroy' to confirm:", "destroy") {
				fmt.Println("Cancelled (must type 'destroy' to confirm)")
				return nil
			}
		}
	} else {
		if !ui.Confirm("Continue? [y/N]:") {
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
		msg := fmt.Sprintf("Are you sure you want to destroy '%s' workspace%s?",
			filepath.Base(targets[0].path), extraMsg)
		if len(targets) > 1 {
			msg += fmt.Sprintf(" (and %d more)", len(targets)-1)
		}
		msg += " [y/N]:"
		if !ui.Confirm(msg) {
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
			counter := ui.Dim(fmt.Sprintf("[%d/%d]", i+1, len(targets)))
			if mode == "destroy" {
				fmt.Printf("%s Destroying %s...\n", counter, ui.Bold(t.path))
			} else {
				fmt.Printf("%s Removing %s...\n", counter, ui.Bold(t.path))
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
		summary := fmt.Sprintf("Summary: %s", ui.Green(fmt.Sprintf("%d succeeded", successCount)))
		if failedCount > 0 {
			summary += fmt.Sprintf(", %s", ui.Red(fmt.Sprintf("%d failed", failedCount)))
		}
		fmt.Println(summary)
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
	bareRoot, _ := worktree.BareRoot()

	items := make([]picker.Item, len(entries))
	for i, e := range entries {
		workspace := filepath.Base(e.Path)
		if bareRoot != "" {
			workspace = strings.TrimPrefix(e.Path, bareRoot+string(os.PathSeparator))
		}

		label := workspace
		if e.Branch != "" && e.Branch != "(detached)" {
			label = fmt.Sprintf("%s [%s]", workspace, e.Branch)
		} else if e.Branch == "(detached)" {
			label = fmt.Sprintf("%s (detached HEAD)", workspace)
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
		b.WriteString(ui.Bold(ui.Red("DESTROY MODE")) + "\n\n")
	}

	b.WriteString(ui.Bold(ui.Accent("Worktree")) + "\n")
	b.WriteString(fmt.Sprintf("  %s %s\n", ui.Subtle("Path:"), item.Value))

	// Try to get branch from value
	entries, _ := worktree.List()
	branch := worktree.BranchFor(entries, item.Value)
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
	status, err := git.QueryIn(item.Value, "status", "--short", "--branch")
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

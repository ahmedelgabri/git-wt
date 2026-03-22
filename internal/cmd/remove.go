package cmd

import (
	"fmt"
	"io"
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

	remote := worktree.DefaultRemote()

	// Interactive mode.
	if len(args) == 0 {
		return removeInteractive(entries, mode, dryRun, remote)
	}

	// Non-interactive mode.
	return removeNonInteractive(entries, args, mode, dryRun, remote)
}

func removeInteractive(entries []worktree.Entry, mode string, dryRun bool, remote string) error {
	if len(entries) == 0 {
		fmt.Printf("No worktrees to %s\n", mode)
		return nil
	}

	items := entriesToPickerItems(entries)
	prompt := fmt.Sprintf("Select worktree(s) to %s (TAB to select multiple): ", mode)
	header := "TAB: select/deselect | ENTER: confirm | ESC: cancel\nLocal branches will also be deleted (remote branches preserved)"
	if mode == "destroy" {
		prompt = "Select worktree(s) to DESTROY (TAB to select multiple): "
		header = "WARNING: This will delete LOCAL and REMOTE branches when applicable\nTAB: select/deselect | ENTER: confirm | ESC: cancel"
	}

	result, err := picker.Run(picker.Config{
		Items:      items,
		Multi:      true,
		Prompt:     prompt,
		Header:     header,
		PreviewCmd: previewWorktreeCmdStr(mode),
	})
	if err != nil {
		return err
	}
	if result.Canceled || len(result.Items) == 0 {
		return nil
	}

	// Resolve selected items to targets.
	var targets []removalTarget
	for _, item := range result.Items {
		targets = append(targets, newRemovalTarget(entries, item.Value))
	}

	// Show what will be removed.
	fmt.Println()
	fmt.Println(renderRemovalPlan(targets, mode, remote, dryRun))
	fmt.Println()

	if dryRun {
		fmt.Printf("%s No changes made\n", ui.Yellow("[DRY RUN]"))
		return nil
	}

	// Confirmation.
	if mode == "destroy" {
		fmt.Println(ui.Red("This action CANNOT be undone."))
		fmt.Println()
		if len(targets) == 1 {
			expect := targets[0].confirmToken()
			if !ui.PromptDangerous(fmt.Sprintf("Type %s to confirm:", ui.Bold(expect)), expect) {
				fmt.Println("Cancelled")
				return nil
			}
		} else {
			if !ui.PromptDangerous(fmt.Sprintf("Type %s to confirm:", ui.Bold("destroy")), "destroy") {
				fmt.Println("Cancelled")
				return nil
			}
		}
	} else {
		if !ui.Confirm("Continue? [y/N]:") {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Execute removal.
	fmt.Println()
	return executeRemoval(targets, mode, remote)
}

func removeNonInteractive(entries []worktree.Entry, args []string, mode string, dryRun bool, remote string) error {
	// Validate all worktree paths first.
	var targets []removalTarget

	for _, arg := range args {
		if err := worktree.Validate(entries, arg); err != nil {
			return err
		}
		resolved, _ := worktree.Resolve(entries, arg)
		targets = append(targets, newRemovalTarget(entries, resolved))
	}

	// Dry run.
	if dryRun {
		fmt.Println(renderRemovalPlan(targets, mode, remote, true))
		fmt.Println()
		fmt.Printf("%s No changes made\n", ui.Yellow("[DRY RUN]"))
		return nil
	}

	// Confirmation for destroy mode.
	if mode == "destroy" {
		fmt.Println(renderRemovalPlan(targets, mode, remote, false))
		fmt.Println()
		extraMsg := ""
		if targets[0].hasBranch() {
			extraMsg = fmt.Sprintf(" and delete its remote branch [%s]", targets[0].branch)
		}
		msg := fmt.Sprintf("Are you sure you want to destroy '%s' workspace%s?",
			filepath.Base(targets[0].path), extraMsg)
		if len(targets) > 1 {
			msg += fmt.Sprintf(" (and %d more)", len(targets)-1)
		}
		msg += " [y/N]:"
		if !ui.Confirm(msg) {
			fmt.Println("Cancelled")
			return nil
		}
	}

	return executeRemoval(targets, mode, remote)
}

type removalTarget struct {
	path     string
	branch   string
	detached bool
}

func newRemovalTarget(entries []worktree.Entry, path string) removalTarget {
	t := removalTarget{path: path}
	if entry := worktree.FindByPath(entries, path); entry != nil {
		t.branch = entry.Branch
		t.detached = entry.Detached
	}
	return t
}

func (t removalTarget) hasBranch() bool {
	return t.branch != "" && !t.detached
}

func (t removalTarget) branchLabel() string {
	switch {
	case t.detached:
		return "detached HEAD"
	case t.branch != "":
		return t.branch
	default:
		return "no branch"
	}
}

func (t removalTarget) confirmToken() string {
	if t.hasBranch() {
		return t.branch
	}
	return "destroy"
}

func renderRemovalPlan(targets []removalTarget, mode string, remote string, dryRun bool) string {
	rows := make([][]string, 0, len(targets))
	localDeletes := 0
	remoteDeletes := 0
	for _, target := range targets {
		if target.hasBranch() {
			localDeletes++
			if mode == "destroy" && remote != "" {
				remoteDeletes++
			}
		}
		rows = append(rows, []string{
			displayWorktreePath(target.path),
			target.branchLabel(),
			removalEffect(target, mode, remote),
		})
	}

	notes := []string{}
	if dryRun {
		notes = append(notes, ui.Yellow("[DRY RUN] Preview only"))
	}
	if mode == "destroy" {
		notes = append(notes, ui.Red("Destructive: matching local and remote branches will be deleted when possible."))
	} else {
		notes = append(notes, ui.Subtle("Remote branches are preserved."))
	}

	summaryParts := []string{ui.Subtle(fmt.Sprintf("%d target(s)", len(targets)))}
	if localDeletes > 0 {
		summaryParts = append(summaryParts, ui.Red(fmt.Sprintf("%d local branch delete(s)", localDeletes)))
	}
	if mode == "destroy" {
		if remoteDeletes > 0 {
			summaryParts = append(summaryParts, ui.Red(fmt.Sprintf("%d remote branch delete(s)", remoteDeletes)))
		} else if remote == "" {
			summaryParts = append(summaryParts, ui.Yellow("no remote configured"))
		}
	}

	return renderTableSection([]ui.TableColumn{
		{Title: "WORKTREE", MinWidth: 18},
		{Title: "BRANCH", MinWidth: 14},
		{Title: "EFFECT", MinWidth: 24},
	}, rows, notes, strings.Join(summaryParts, " • "))
}

func removalEffect(target removalTarget, mode string, remote string) string {
	if !target.hasBranch() {
		return ui.Yellow("remove worktree only")
	}
	if mode == "destroy" {
		if remote != "" {
			return ui.Red("remove + delete local + remote")
		}
		return ui.Red("remove + delete local")
	}
	return ui.Red("remove + delete local")
}

func executeRemoval(targets []removalTarget, mode string, remote string) error {
	successCount := 0
	failedCount := 0

	for i, t := range targets {
		if len(targets) > 1 {
			counter := ui.Dim(fmt.Sprintf("[%d/%d]", i+1, len(targets)))
			fmt.Printf("%s %s\n", counter, ui.Bold(filepath.Base(t.path)))
		}

		if err := removeSingleWorktree(t, mode, remote); err != nil {
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

func removeSingleWorktree(target removalTarget, mode string, remote string) error {
	name := filepath.Base(target.path)

	// Remove the worktree.
	if err := ui.SpinWithOutput(fmt.Sprintf("Removing worktree %s", ui.Accent(name)), func(w io.Writer) error {
		return git.RunTo(w, "worktree", "remove", "-f", target.path)
	}); err != nil {
		return err
	}

	if !target.hasBranch() {
		return nil
	}

	// Delete local branch (fast, no spinner needed).
	out, err := git.RunWithOutput("branch", "-D", target.branch)
	if err != nil {
		if out != "" {
			return fmt.Errorf("%s", strings.TrimSpace(out))
		}
		return err
	}
	ui.Successf("Deleted local branch %s", ui.Accent(target.branch))

	// Delete remote branch in destroy mode.
	if mode == "destroy" {
		deleteRemoteBranch(target.branch, remote)
	}

	return nil
}

func deleteRemoteBranch(branch, remote string) {
	if remote == "" {
		fmt.Printf("%s %s\n", ui.Muted("·"), ui.Muted("No remote configured"))
		return
	}

	remoteBranch := remote + "/" + branch

	// Check if remote branch exists.
	if _, err := git.Query("ls-remote", "--exit-code", "--heads", remote, branch); err != nil {
		fmt.Printf("%s %s\n", ui.Muted("·"), ui.Muted("No remote branch "+remoteBranch))
		return
	}

	// Delete remote branch (network operation, needs spinner).
	if err := ui.SpinWithOutput(fmt.Sprintf("Deleting remote branch %s", ui.Accent(remoteBranch)), func(w io.Writer) error {
		return git.RunTo(w, "push", remote, "--delete", branch)
	}); err != nil {
		ui.Warnf("Failed to delete remote branch %s: %s", remoteBranch, err)
	}
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
		switch {
		case e.Detached:
			label = fmt.Sprintf("%s (detached HEAD)", workspace)
		case e.Branch != "":
			label = fmt.Sprintf("%s [%s]", workspace, e.Branch)
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

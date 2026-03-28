package cmd

import (
	"context"
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

type removalAction string

const (
	removalActionRemove removalAction = "remove"
	removalActionPrune  removalAction = "prune"
)

type removalItem struct {
	Action removalAction
	Target removalTarget
	Reason string
}

type removeOptions struct {
	dryRun       bool
	deleteRemote bool
}

type removeFilters struct {
	merged bool
	gone   bool
	stale  bool
}

func (f removeFilters) any() bool {
	return f.merged || f.gone || f.stale
}

var removeCmd = &cobra.Command{
	Use:     "remove [<worktree>...]",
	Aliases: []string{"rm"},
	Short:   "Remove worktrees directly or by safe cleanup filters",
	Long: `Remove worktrees directly or by safe cleanup filters.

By default, removing a worktree also deletes its local branch. Use
'--delete-remote' to also delete the remote branch when possible.

Cleanup filters let you select safe bulk candidates:
  --merged  branches fully merged into the default branch
  --gone    branches whose upstream is gone
  --stale   missing or prunable worktree metadata
  --sweep   shorthand for --merged --gone --stale

With no arguments and no cleanup filters, an interactive picker is shown.`,
	Example: `  git wt remove feature-1
  git wt remove feature-1 --delete-remote
  git wt remove feature-1 feature-2
  git wt remove --sweep
  git wt remove --merged --dry-run`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runRemove,
}

func init() {
	removeCmd.Flags().BoolP("dry-run", "n", false, "Preview what would be removed without making changes")
	removeCmd.Flags().Bool("delete-remote", false, "Also delete matching remote branches when possible")
	removeCmd.Flags().Bool("merged", false, "Select worktrees whose branches are fully merged into the default branch")
	removeCmd.Flags().Bool("gone", false, "Select worktrees whose upstream is gone")
	removeCmd.Flags().Bool("stale", false, "Select stale or prunable worktree metadata")
	removeCmd.Flags().Bool("sweep", false, "Select merged, gone, and stale cleanup candidates")
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	return runRemoveWithDefaults(cmd, args, false, true)
}

func runRemoveWithDefaults(cmd *cobra.Command, args []string, defaultDeleteRemote bool, allowFilters bool) error {
	opts := removeOptions{
		dryRun:       boolFlag(cmd, "dry-run"),
		deleteRemote: defaultDeleteRemote || boolFlag(cmd, "delete-remote"),
	}

	filters := removeFilters{}
	if allowFilters {
		filters = removeFilters{
			merged: boolFlag(cmd, "merged"),
			gone:   boolFlag(cmd, "gone"),
			stale:  boolFlag(cmd, "stale"),
		}
		if boolFlag(cmd, "sweep") {
			filters.merged = true
			filters.gone = true
			filters.stale = true
		}
	}

	if len(args) > 0 && filters.any() {
		return fmt.Errorf("cleanup filters cannot be combined with explicit worktree arguments")
	}

	entries, err := worktree.List()
	if err != nil {
		return err
	}

	remote := worktree.DefaultRemote()

	switch {
	case filters.any():
		return removeByFilter(entries, filters, opts, remote)
	case len(args) == 0:
		return removeInteractive(entries, opts, remote)
	default:
		return removeNonInteractive(entries, args, opts, remote)
	}
}

func removeInteractive(entries []worktree.Entry, opts removeOptions, remote string) error {
	if len(entries) == 0 {
		fmt.Println(ui.Subtle("No worktrees to remove"))
		return nil
	}

	prompt := "Select worktree(s) to remove (TAB to select multiple): "
	header := "TAB: select/deselect | ENTER: confirm | ESC: cancel\nLocal branches will also be deleted when applicable"
	if opts.deleteRemote {
		prompt = "Select worktree(s) to remove and delete remotely (TAB to select multiple): "
		header = "WARNING: This will delete LOCAL and REMOTE branches when possible\nTAB: select/deselect | ENTER: confirm | ESC: cancel"
	}

	result, err := picker.Run(picker.Config{
		Items:      entriesToPickerItems(entries),
		Multi:      true,
		Prompt:     prompt,
		Header:     header,
		PreviewCmd: previewWorktreeCmdStr(removalPreviewMode(opts.deleteRemote)),
	})
	if err != nil {
		return err
	}
	if result.Canceled || len(result.Items) == 0 {
		return nil
	}

	items := make([]removalItem, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, removalItem{
			Action: removalActionRemove,
			Target: newRemovalTarget(entries, item.Value),
		})
	}

	return runRemovalPlan(items, opts, remote, false)
}

func removeNonInteractive(entries []worktree.Entry, args []string, opts removeOptions, remote string) error {
	items, err := explicitRemovalItems(entries, args)
	if err != nil {
		return err
	}
	return runRemovalPlan(items, opts, remote, false)
}

func explicitRemovalItems(entries []worktree.Entry, args []string) ([]removalItem, error) {
	items := make([]removalItem, 0, len(args))
	for _, arg := range args {
		if err := worktree.Validate(entries, arg); err != nil {
			return nil, err
		}
		resolved, _ := worktree.Resolve(entries, arg)
		items = append(items, removalItem{
			Action: removalActionRemove,
			Target: newRemovalTarget(entries, resolved),
		})
	}
	return items, nil
}

func removeByFilter(entries []worktree.Entry, filters removeFilters, opts removeOptions, remote string) error {
	items, err := findRemovalCandidates(entries, filters)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Println(ui.Subtle("No matching cleanup candidates found"))
		return nil
	}

	selected := items
	if shouldUseInteractiveCleanupSelection() {
		selected, err = selectRemovalCandidates(items, opts.deleteRemote)
		if err != nil {
			return err
		}
		if len(selected) == 0 {
			return nil
		}
	}

	return runRemovalPlan(selected, opts, remote, true)
}

func shouldUseInteractiveCleanupSelection() bool {
	if os.Getenv("GIT_WT_SELECT") != "" {
		return true
	}
	return ui.CanRenderSelection()
}

func selectRemovalCandidates(items []removalItem, deleteRemote bool) ([]removalItem, error) {
	prompt := "Select cleanup candidate(s) to remove (TAB to select multiple): "
	header := "TAB: select/deselect | ENTER: confirm | ESC: cancel\nSafe candidates only: merged branches, gone upstreams, and stale metadata"
	if deleteRemote {
		header = "WARNING: Matching remote branches will also be deleted when possible\nTAB: select/deselect | ENTER: confirm | ESC: cancel"
	}

	result, err := picker.Run(picker.Config{
		Items:      removalCandidatesToPickerItems(items),
		Multi:      true,
		Prompt:     prompt,
		Header:     header,
		PreviewCmd: previewWorktreeCmdStr(removalPreviewMode(deleteRemote)),
	})
	if err != nil {
		return nil, err
	}
	if result.Canceled || len(result.Items) == 0 {
		return nil, nil
	}

	byPath := make(map[string]removalItem, len(items))
	for _, item := range items {
		byPath[item.Target.path] = item
	}

	selected := make([]removalItem, 0, len(result.Items))
	for _, item := range result.Items {
		if candidate, ok := byPath[item.Value]; ok {
			selected = append(selected, candidate)
		}
	}
	return selected, nil
}

func runRemovalPlan(items []removalItem, opts removeOptions, remote string, cleanup bool) error {
	fmt.Println(renderRemovalPlan(items, opts, remote, cleanup))
	fmt.Println()

	if opts.dryRun {
		fmt.Printf("%s No changes made\n", ui.Yellow("[DRY RUN]"))
		return nil
	}

	if !confirmRemoval(items, opts, remote, cleanup) {
		fmt.Println("Cancelled")
		return nil
	}

	fmt.Println()
	return executeRemovalItems(items, opts.deleteRemote, remote)
}

func confirmRemoval(items []removalItem, opts removeOptions, remote string, cleanup bool) bool {
	if cleanup {
		fmt.Println(ui.Red("Bulk cleanup is destructive."))
		fmt.Println(ui.Subtle("Selected worktrees will be removed, local branches deleted when applicable, and stale metadata pruned."))
		if opts.deleteRemote {
			if remote != "" {
				fmt.Println(ui.Red(fmt.Sprintf("Matching remote branches on %s will also be deleted when possible.", remote)))
			} else {
				fmt.Println(ui.Yellow("No remote configured; remote branch deletion will be skipped."))
			}
		}
		fmt.Println()
		return ui.PromptDangerous(fmt.Sprintf("Type %s to confirm:", ui.Bold("cleanup")), "cleanup")
	}

	if opts.deleteRemote {
		fmt.Println(ui.Red("This action will delete local and remote branches when possible."))
		fmt.Println()
		expect := "remove"
		if len(items) == 1 && items[0].Target.hasBranch() {
			expect = items[0].Target.branch
		}
		return ui.PromptDangerous(fmt.Sprintf("Type %s to confirm:", ui.Bold(expect)), expect)
	}

	if len(items) == 1 {
		target := items[0].Target
		if target.hasBranch() {
			return ui.Confirm(fmt.Sprintf("Remove '%s' and delete its local branch? [y/N]:", filepath.Base(target.path)))
		}
		return ui.Confirm(fmt.Sprintf("Remove '%s'? [y/N]:", filepath.Base(target.path)))
	}

	return ui.Confirm(fmt.Sprintf("Remove %d worktree(s) and delete local branches where applicable? [y/N]:", len(items)))
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

func renderRemovalPlan(items []removalItem, opts removeOptions, remote string, cleanup bool) string {
	rows := make([][]string, 0, len(items))
	removeCount, pruneCount := 0, 0
	localDeletes := 0
	remoteDeletes := 0
	for _, item := range items {
		switch item.Action {
		case removalActionPrune:
			pruneCount++
		default:
			removeCount++
		}
		if item.Action == removalActionRemove && item.Target.hasBranch() {
			localDeletes++
			if opts.deleteRemote && remote != "" {
				remoteDeletes++
			}
		}

		rows = append(rows, []string{
			renderRemovalAction(item.Action),
			displayWorktreePath(item.Target.path),
			item.Target.branchLabel(),
			removalEffect(item, opts.deleteRemote, remote),
			removalReason(item),
		})
	}

	notes := []string{}
	if opts.dryRun {
		notes = append(notes, ui.Yellow("[DRY RUN] Preview only"))
	}
	if cleanup {
		notes = append(notes, ui.Subtle("Safe candidates only: merged branches, gone upstreams, and stale metadata."))
	}
	if opts.deleteRemote {
		if remote != "" {
			notes = append(notes, ui.Red(fmt.Sprintf("Matching remote branches on %s will be deleted when possible.", remote)))
		} else {
			notes = append(notes, ui.Yellow("No remote configured; remote branch deletion will be skipped."))
		}
	} else {
		notes = append(notes, ui.Subtle("Remote branches are preserved."))
	}
	if pruneCount > 0 {
		notes = append(notes, ui.Subtle("Prune candidates remove stale metadata only; their branches are preserved."))
	}

	label := "target(s)"
	if cleanup {
		label = "candidate(s)"
	}
	summaryParts := []string{ui.Subtle(fmt.Sprintf("%d %s", len(items), label))}
	if removeCount > 0 {
		summaryParts = append(summaryParts, ui.Red(fmt.Sprintf("%d remove", removeCount)))
	}
	if pruneCount > 0 {
		summaryParts = append(summaryParts, ui.Yellow(fmt.Sprintf("%d prune", pruneCount)))
	}
	if localDeletes > 0 {
		summaryParts = append(summaryParts, ui.Red(fmt.Sprintf("%d local branch delete(s)", localDeletes)))
	}
	if opts.deleteRemote {
		if remoteDeletes > 0 {
			summaryParts = append(summaryParts, ui.Red(fmt.Sprintf("%d remote branch delete(s)", remoteDeletes)))
		} else if remote == "" {
			summaryParts = append(summaryParts, ui.Yellow("no remote configured"))
		}
	}

	return renderTableSection([]ui.TableColumn{
		{Title: "ACTION", MinWidth: 8, MaxWidth: 10},
		{Title: "WORKTREE", MinWidth: 18},
		{Title: "BRANCH", MinWidth: 14},
		{Title: "EFFECT", MinWidth: 28},
		{Title: "REASON", MinWidth: 18, MaxWidth: 64},
	}, rows, notes, strings.Join(summaryParts, " • "))
}

func renderRemovalAction(action removalAction) string {
	switch action {
	case removalActionPrune:
		return ui.Yellow("prune")
	default:
		return ui.Red("remove")
	}
}

func removalEffect(item removalItem, deleteRemote bool, remote string) string {
	if item.Action == removalActionPrune {
		return ui.Yellow("prune stale metadata")
	}
	if !item.Target.hasBranch() {
		return ui.Yellow("remove worktree only")
	}
	if deleteRemote {
		if remote != "" {
			return ui.Red("remove + delete local + remote")
		}
		return ui.Red("remove + delete local")
	}
	return ui.Red("remove + delete local")
}

func removalReason(item removalItem) string {
	if strings.TrimSpace(item.Reason) == "" {
		return ui.Subtle("selected target")
	}
	return item.Reason
}

func executeRemovalItems(items []removalItem, deleteRemote bool, remote string) error {
	successCount := 0
	failedCount := 0

	for i, item := range items {
		if len(items) > 1 {
			counter := ui.Dim(fmt.Sprintf("[%d/%d]", i+1, len(items)))
			fmt.Printf("%s %s\n", counter, ui.Bold(filepath.Base(item.Target.path)))
		}

		var err error
		switch item.Action {
		case removalActionPrune:
			err = pruneStaleWorktree(item.Target)
		default:
			err = removeSingleWorktree(item.Target, deleteRemote, remote)
		}
		if err != nil {
			failedCount++
		} else {
			successCount++
		}
	}

	if len(items) > 1 {
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

func pruneStaleWorktree(target removalTarget) error {
	name := filepath.Base(target.path)
	return ui.SpinWithOutputContext(fmt.Sprintf("Pruning stale metadata for %s", ui.Accent(name)), func(ctx context.Context, w io.Writer) error {
		return git.RunToContext(ctx, w, "worktree", "remove", "--force", target.path)
	})
}

func removeSingleWorktree(target removalTarget, deleteRemote bool, remote string) error {
	name := filepath.Base(target.path)

	if err := ui.SpinWithOutputContext(fmt.Sprintf("Removing worktree %s", ui.Accent(name)), func(ctx context.Context, w io.Writer) error {
		return git.RunToContext(ctx, w, "worktree", "remove", "-f", target.path)
	}); err != nil {
		return err
	}

	if !target.hasBranch() {
		return nil
	}

	out, err := git.RunWithOutput("branch", "-D", target.branch)
	if err != nil {
		if out != "" {
			return fmt.Errorf("%s", strings.TrimSpace(out))
		}
		return err
	}
	ui.Successf("Deleted local branch %s", ui.Accent(target.branch))

	if deleteRemote {
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

	if _, err := git.Query("ls-remote", "--exit-code", "--heads", remote, branch); err != nil {
		fmt.Printf("%s %s\n", ui.Muted("·"), ui.Muted("No remote branch "+remoteBranch))
		return
	}

	if err := ui.SpinWithOutputContext(fmt.Sprintf("Deleting remote branch %s", ui.Accent(remoteBranch)), func(ctx context.Context, w io.Writer) error {
		return git.RunToContext(ctx, w, "push", remote, "--delete", branch)
	}); err != nil {
		ui.Warnf("Failed to delete remote branch %s: %s", remoteBranch, err)
	}
}

func entriesToPickerItems(entries []worktree.Entry) []picker.Item {
	items := make([]picker.Item, len(entries))
	for i, e := range entries {
		workspace := pickerWorkspaceName(e.Path)

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

func removalCandidatesToPickerItems(items []removalItem) []picker.Item {
	pickerItems := make([]picker.Item, 0, len(items))
	for _, item := range items {
		workspace := pickerWorkspaceName(item.Target.path)
		label := fmt.Sprintf("%s [%s]", workspace, item.Target.branchLabel())
		desc := string(item.Action)
		if item.Reason != "" {
			desc += " · " + item.Reason
		}
		pickerItems = append(pickerItems, picker.Item{
			Label: label,
			Value: item.Target.path,
			Desc:  desc,
		})
	}
	return pickerItems
}

func pickerWorkspaceName(path string) string {
	bareRoot, _ := worktree.BareRoot()
	if bareRoot != "" {
		return strings.TrimPrefix(path, bareRoot+string(os.PathSeparator))
	}
	return filepath.Base(path)
}

func removalPreviewMode(deleteRemote bool) string {
	if deleteRemote {
		return previewModeDeleteRemote
	}
	return previewModeRemove
}

func boolFlag(cmd *cobra.Command, name string) bool {
	if cmd.Flags().Lookup(name) == nil {
		return false
	}
	v, _ := cmd.Flags().GetBool(name)
	return v
}

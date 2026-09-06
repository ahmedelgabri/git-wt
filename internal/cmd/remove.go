package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/hook"
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
	force        bool
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

By default, removing a worktree also deletes its local branch, provided its
commits are preserved by another branch or tag. Dirty worktrees and unique
commits require --force with explicit targets. Current and locked worktrees
remain protected. Ignored files do not block removal and are deleted with the
worktree, as with native Git. Use --delete-remote to delete the target's
configured upstream.

Cleanup filters let you select safe bulk candidates:
  --merged  branches fully merged into the cleanup base
  --gone    branches whose upstream is gone and which are fully merged
  --stale   missing, unlocked worktree paths with attached branches
  --sweep   shorthand for --merged --gone --stale

Set an explicit local cleanup base with:
  git config wt.cleanupBase refs/heads/main
Otherwise cleanup discovers the remote default branch. A local remote (.) needs
an explicit base. Raw URL discovery respects wt.remoteTimeout.

Remote deletion with multiple push URLs or differing fetch/push URLs requires
Git 2.46 or newer. One matching fetch/push URL needs no destination overrides.

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
	removeCmd.Flags().Bool("delete-remote", false, "Also delete each worktree branch's configured upstream branch")
	removeCmd.Flags().Bool("force", false, "Allow explicit removal of dirty worktrees and commits without another retained ref")
	removeCmd.Flags().Bool("merged", false, "Select worktrees whose branches are fully merged into the cleanup base")
	removeCmd.Flags().Bool("gone", false, "Select fully merged worktrees whose upstream is gone")
	removeCmd.Flags().Bool("stale", false, "Select missing, unlocked worktree paths with attached branches")
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
		force:        boolFlag(cmd, "force"),
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

	if opts.force && filters.any() {
		return fmt.Errorf("--force cannot be combined with safe cleanup filters")
	}
	switch {
	case filters.any():
		return removeByFilterPreloaded(filters, opts)
	case len(args) == 0:
		return removeInteractivePreloaded(opts)
	default:
		entries, err := worktree.List()
		if err != nil {
			return err
		}
		return removeNonInteractive(entries, args, opts)
	}
}

func removeInteractivePreloaded(opts removeOptions) error {
	entries, err := runPreload(context.Background(), "Loading worktrees…", func(ctx context.Context, update func(phase ui.AsyncPhase, message string)) ([]worktree.Entry, error) {
		update(ui.AsyncLoading, "Loading worktrees…")
		return worktree.ListContext(ctx)
	})
	if errors.Is(err, context.Canceled) {
		return nil
	}
	if err != nil {
		return err
	}
	return removeInteractive(entries, opts)
}

func removeInteractive(entries []worktree.Entry, opts removeOptions) error {
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

	return runRemovalPlan(items, opts, false)
}

func removeNonInteractive(entries []worktree.Entry, args []string, opts removeOptions) error {
	items, err := explicitRemovalItems(entries, args)
	if err != nil {
		return err
	}
	return runRemovalPlan(items, opts, false)
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

func removeByFilterPreloaded(filters removeFilters, opts removeOptions) error {
	items, err := runPreload(context.Background(), "Scanning cleanup candidates…", func(ctx context.Context, update func(phase ui.AsyncPhase, message string)) ([]removalItem, error) {
		update(ui.AsyncLoading, "Loading worktrees…")
		entries, err := worktree.ListContext(ctx)
		if err != nil {
			return nil, err
		}
		update(ui.AsyncPartial, "Scanning cleanup candidates…")
		return findRemovalCandidates(ctx, entries, filters)
	})
	if errors.Is(err, context.Canceled) {
		return nil
	}
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

	return runRemovalPlan(selected, opts, true)
}

func shouldUseInteractiveCleanupSelection() bool {
	if os.Getenv("GIT_WT_SELECT") != "" {
		return true
	}
	return ui.CanRenderSelection()
}

func selectRemovalCandidates(items []removalItem, deleteRemote bool) ([]removalItem, error) {
	prompt := "Select cleanup candidate(s) to remove (TAB to select multiple): "
	header := "TAB: select/deselect | ENTER: confirm | ESC: cancel\nSafe candidates only: fully merged branches and missing, unlocked metadata"
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

func runRemovalPlan(items []removalItem, opts removeOptions, cleanup bool) error {
	fmt.Println(renderRemovalPlan(items, opts, cleanup))
	fmt.Println()

	if opts.dryRun {
		fmt.Printf("%s No changes made\n", ui.Yellow("[DRY RUN]"))
		return nil
	}

	// Check the entire selection before hooks or local mutations. A later
	// incompatible target must not strand earlier targets in a bulk removal.
	if opts.deleteRemote {
		seen := make(map[string]bool)
		for _, item := range items {
			remote := item.Target.remote
			if item.Action == removalActionRemove && remote != "" && !seen[remote] {
				if _, err := remoteDeletionDestinations(remote); err != nil {
					return err
				}
				seen[remote] = true
			}
		}
	}

	if !confirmRemoval(items, opts, cleanup) {
		fmt.Println("Cancelled")
		return nil
	}

	fmt.Println()
	return executeRemovalItems(items, opts, cleanup)
}

func confirmRemoval(items []removalItem, opts removeOptions, cleanup bool) bool {
	if cleanup {
		fmt.Println(ui.Red("Bulk cleanup is destructive."))
		fmt.Println(ui.Subtle("Selected worktrees will be removed, local branches deleted when applicable, and stale metadata pruned."))
		if opts.deleteRemote {
			fmt.Println(ui.Red("The configured upstream branches shown in the plan will also be deleted."))
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
	path         string
	branch       string
	detached     bool
	locked       bool
	lockedReason string
	prunable     bool
	remote       string
	remoteBranch string
	upstreamRef  string
}

func newRemovalTargetFromEntry(entry worktree.Entry) removalTarget {
	remote, remoteBranch, upstreamRef := removalUpstream(entry.Branch)
	return removalTarget{
		path:         entry.Path,
		branch:       entry.Branch,
		detached:     entry.Detached,
		locked:       entry.Locked,
		lockedReason: entry.LockedReason,
		prunable:     entry.Prunable,
		remote:       remote,
		remoteBranch: remoteBranch,
		upstreamRef:  upstreamRef,
	}
}

func newRemovalTarget(entries []worktree.Entry, path string) removalTarget {
	if entry := worktree.FindByPath(entries, path); entry != nil {
		return newRemovalTargetFromEntry(*entry)
	}
	return removalTarget{path: path}
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

func renderRemovalPlan(items []removalItem, opts removeOptions, cleanup bool) string {
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
			if opts.deleteRemote && item.Target.remote != "" {
				remoteDeletes++
			}
		}

		rows = append(rows, []string{
			renderRemovalAction(item.Action),
			displayWorktreePath(item.Target.path),
			item.Target.branchLabel(),
			removalEffect(item, opts.deleteRemote),
			removalReason(item),
		})
	}

	notes := []string{}
	if opts.dryRun {
		notes = append(notes, ui.Yellow("[DRY RUN] Preview only"))
	}
	if cleanup {
		notes = append(notes, ui.Subtle("Safe candidates only: fully merged branches and missing, unlocked metadata."))
	}
	if removeCount > 0 {
		notes = append(notes, ui.Yellow("Ignored files, including .env files and build output, are deleted with the worktree."))
	}
	if opts.force {
		notes = append(notes, ui.Red("FORCE: dirty files and commits without another retained ref may be lost."))
	}
	if opts.deleteRemote {
		notes = append(notes, ui.Red("Only configured upstream branches shown in the plan will be deleted."))
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
		} else {
			summaryParts = append(summaryParts, ui.Yellow("no remote upstreams to delete"))
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

func removalEffect(item removalItem, deleteRemote bool) string {
	if item.Action == removalActionPrune {
		return ui.Yellow("prune stale metadata")
	}
	if !item.Target.hasBranch() {
		return ui.Yellow("remove worktree only")
	}
	if deleteRemote && item.Target.remote != "" {
		return ui.Red("remove + delete local + " + item.Target.remote + "/" + item.Target.remoteBranch)
	}
	return ui.Red("remove + delete local")
}

func removalReason(item removalItem) string {
	if strings.TrimSpace(item.Reason) == "" {
		return ui.Subtle("selected target")
	}
	return item.Reason
}

func executeRemovalItems(items []removalItem, opts removeOptions, cleanup bool) error {
	successCount := 0
	failedCount := 0
	var singleErr error

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
			err = removeSingleWorktree(item.Target, opts, cleanup)
		}
		if err != nil {
			failedCount++
			if len(items) == 1 {
				singleErr = err
			} else {
				fmt.Fprintf(os.Stderr, "%s: %v\n", item.Target.path, err)
			}
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

	if singleErr != nil {
		return singleErr
	}
	if failedCount > 0 {
		return fmt.Errorf("%d removal(s) failed", failedCount)
	}
	return nil
}

func pruneStaleWorktree(target removalTarget) error {
	entries, err := worktree.List()
	if err != nil {
		return err
	}
	entry := worktree.FindByPath(entries, target.path)
	if entry == nil {
		return fmt.Errorf("stale worktree no longer exists: %s", target.path)
	}
	if _, stale := pruneReason(*entry); !stale {
		return fmt.Errorf("worktree is no longer a missing, unlocked prune candidate: %s", target.path)
	}
	name := filepath.Base(target.path)
	return ui.SpinWithOutputContext(fmt.Sprintf("Pruning stale metadata for %s", ui.Accent(name)), func(ctx context.Context, w io.Writer) error {
		return git.RunToContext(ctx, w, "worktree", "remove", "--", target.path)
	})
}

func preflightRemoveHook(target removalTarget) (bool, error) {
	if currentRoot, err := currentWorktreeRoot(); err == nil && samePath(target.path, currentRoot) {
		return false, fmt.Errorf("cannot remove current worktree %q", target.path)
	}

	if target.locked {
		if target.lockedReason != "" {
			return false, fmt.Errorf("cannot remove locked worktree %q: %s", target.path, target.lockedReason)
		}
		return false, fmt.Errorf("cannot remove locked worktree %q", target.path)
	}

	if target.prunable {
		return false, nil
	}

	if _, err := os.Stat(target.path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func removeSingleWorktree(target removalTarget, opts removeOptions, cleanup bool) error {
	name := filepath.Base(target.path)

	runHooks, err := preflightRemoveHook(target)
	if err != nil {
		return err
	}

	var beforeHooks, afterHooks []string
	if runHooks {
		beforeHooks, err = hook.Load(hook.BeforeRemove)
		if err != nil {
			return err
		}
		afterHooks, err = hook.Load(hook.AfterRemove)
		if err != nil {
			return err
		}
		runHooks = len(beforeHooks) > 0 || len(afterHooks) > 0
	}

	var bareRoot string
	if runHooks {
		bareRoot, err = worktree.BareRoot()
		if err != nil {
			return err
		}
	}
	invocation := hook.Invocation{
		Event:        hook.BeforeRemove,
		Dir:          target.path,
		WorktreePath: target.path,
		Branch:       target.branch,
		BareRoot:     bareRoot,
	}
	if runHooks {
		if err := hook.Run(context.Background(), beforeHooks, invocation, os.Stderr); err != nil {
			return fmt.Errorf("before removing worktree %q: %w", name, err)
		}
	}

	// Hooks and interactive selection may have taken time. Re-read identity and
	// safety immediately before removing anything.
	entries, err := worktree.List()
	if err != nil {
		return err
	}
	entry := worktree.FindByPath(entries, target.path)
	if entry == nil || entry.Branch != target.branch || entry.Detached != target.detached {
		return fmt.Errorf("worktree changed since selection: %s", target.path)
	}
	fresh := newRemovalTargetFromEntry(*entry)
	for _, other := range entries {
		if target.hasBranch() && other.Branch == target.branch && other.Path != target.path {
			return fmt.Errorf("branch %s is also checked out at %s", target.branch, other.Path)
		}
	}
	if _, err := preflightRemoveHook(fresh); err != nil {
		return err
	}
	if fresh.remote != target.remote || fresh.remoteBranch != target.remoteBranch {
		return fmt.Errorf("upstream changed since selection: %s", target.path)
	}
	branchHead := ""
	if target.hasBranch() {
		branchHead, err = git.Query("rev-parse", "--verify", "refs/heads/"+target.branch)
		if err != nil {
			return err
		}
	}
	if !opts.force {
		if err := validateRemovalSafety(fresh, opts.deleteRemote, cleanup); err != nil {
			return err
		}
	}
	var deletions []remoteDeletion
	if opts.deleteRemote && target.remote != "" {
		deletions, err = planRemoteDeletions(target, branchHead, opts.force)
		if err != nil {
			return err
		}
	}
	if err := ui.SpinWithOutputContext(fmt.Sprintf("Removing worktree %s", ui.Accent(name)), func(ctx context.Context, w io.Writer) error {
		args := []string{"worktree", "remove"}
		if opts.force {
			args = append(args, "--force")
		}
		return git.RunToContext(ctx, w, append(args, "--", target.path)...)
	}); err != nil {
		return err
	}

	if target.hasBranch() {
		if err := deleteLocalBranch(target.branch, branchHead); err != nil {
			return err
		}
		ui.Successf("Deleted local branch %s", ui.Accent(target.branch))

		if opts.deleteRemote {
			if err := deleteRemoteBranches(target.remoteBranch, target.remote, deletions); err != nil {
				return err
			}
		}
	}

	if runHooks {
		invocation.Event = hook.AfterRemove
		invocation.Dir = bareRoot
		if err := hook.Run(context.Background(), afterHooks, invocation, os.Stderr); err != nil {
			return fmt.Errorf("worktree %q was removed, but %w", name, err)
		}
	}
	return nil
}

func deleteRemoteBranches(branch, remote string, deletions []remoteDeletion) error {
	if remote == "" {
		fmt.Printf("%s %s\n", ui.Muted("·"), ui.Muted("No remote upstream configured; remote deletion skipped"))
		return nil
	}

	remoteBranch := remote + "/" + branch

	for i, deletion := range deletions {
		if deletion.head == "" {
			fmt.Printf("%s No remote branch %s at push destination %d; deletion skipped\n", ui.Muted("·"), remoteBranch, i+1)
			continue
		}
		if err := ui.SpinWithOutputContext(fmt.Sprintf("Deleting remote branch %s", ui.Accent(remoteBranch)), func(ctx context.Context, w io.Writer) error {
			// A single matching fetch/push URL needs no overrides, even on old
			// Git. Both paths retain the named remote's transport settings.
			var args []string
			if deletion.overrideURL {
				args = []string{"-c", "remote." + remote + ".pushurl=", "-c", "remote." + remote + ".pushurl=" + deletion.url}
			}
			return git.RunToContext(ctx, w, append(args, "push", "--force-with-lease=refs/heads/"+branch+":"+deletion.head, remote, ":refs/heads/"+branch)...)
		}); err != nil {
			return fmt.Errorf("local worktree removed, but remote deletion failed for %s at %s: %w", remoteBranch, deletion.url, err)
		}
	}
	return nil
}

func entriesToPickerItems(entries []worktree.Entry) []picker.Item {
	items := make([]picker.Item, len(entries))
	bareRoot, _ := worktree.BareRoot()
	homeDir, _ := os.UserHomeDir()
	for i, e := range entries {
		workspace := pickerWorkspaceNameWithBareRoot(e.Path, bareRoot)

		label := workspace
		switch {
		case e.Detached:
			label = fmt.Sprintf("%s (detached HEAD)", workspace)
		case e.Branch != "":
			label = fmt.Sprintf("%s [%s]", workspace, e.Branch)
		}

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
	bareRoot, _ := worktree.BareRoot()
	for _, item := range items {
		workspace := pickerWorkspaceNameWithBareRoot(item.Target.path, bareRoot)
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

func pickerWorkspaceNameWithBareRoot(path, bareRoot string) string {
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

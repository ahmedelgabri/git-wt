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
	// force is set when the user confirmed discarding this target's work.
	force bool
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
commits are preserved by another branch or tag. On a terminal, each dirty
worktree or branch with unique commits asks you to type its name to discard
that work, or press Enter to skip it; the rest of the selection continues.
Without a terminal such targets are skipped and the command exits 1. --force
discards without asking, for explicit targets. With --delete-remote, --force
also deletes remote branches that have commits you have not fetched. Current
and locked worktrees remain protected. Ignored files do not block removal and are deleted with the
worktree, as with native Git. Use --delete-remote to delete the target's
configured upstream. An upstream with a different name than the local branch,
such as origin/release for a branch created from it, must be confirmed by
typing its remote name, and cleanup filters never delete it.

Cleanup filters let you select safe bulk candidates:
  --merged  branches fully merged into the cleanup base
  --gone    branches whose upstream is gone and which are fully merged
  --stale   missing, unlocked worktree paths with attached branches
  --sweep   shorthand for --merged --gone --stale

Set an explicit cleanup base with either:
  git config wt.cleanupBase refs/heads/main
  git config wt.cleanupBase refs/remotes/origin/main
Remote-tracking bases use the last fetched tip without an implicit fetch, and
protect the same-name local branch if it exists. Without wt.cleanupBase,
cleanup discovers the remote default branch. A local remote (.) needs an explicit base.
Raw URL discovery respects wt.remoteTimeout.

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
	removeCmd.Flags().Bool("force", false, "Allow explicit removal of dirty worktrees and commits without another retained ref; with --delete-remote, also unfetched remote commits")
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
		ui.Cancelled()
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
	if result.Canceled {
		ui.Cancelled()
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
		ui.Cancelled()
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
	if result.Canceled {
		ui.Cancelled()
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

	ready, skipped := checkRemovalItems(items, opts, cleanup)
	var skippedErr error
	if skipped > 0 {
		skippedErr = fmt.Errorf("%d target(s) skipped", skipped)
	}
	if len(ready) == 0 {
		fmt.Println(ui.Subtle("Nothing to remove"))
		return skippedErr
	}

	if !confirmRemoval(ready, opts, cleanup) {
		ui.Cancelled()
		return skippedErr
	}

	fmt.Println()
	return errors.Join(executeRemovalItems(ready, opts, cleanup), skippedErr)
}

// checkRemovalItems checks every target before confirmation and hooks, so a
// doomed removal never runs teardown hooks. On a terminal, each target with
// work to discard gets its own confirmation; skipping it keeps the rest of the
// selection. Without a terminal, and in cleanup, which never forces, such
// targets are skipped and counted, as are targets that fail other checks.
func checkRemovalItems(items []removalItem, opts removeOptions, cleanup bool) (ready []removalItem, skipped int) {
	for _, item := range items {
		if item.Action != removalActionRemove {
			ready = append(ready, item)
			continue
		}
		_, _, err := checkRemoval(item.Target, opts, cleanup)
		var unsafe *unsafeRemovalError
		switch {
		case err == nil:
			ready = append(ready, item)
		case errors.As(err, &unsafe) && !cleanup && ui.CanPrompt():
			if confirmDiscard(item.Target, unsafe) {
				item.force = true
				ready = append(ready, item)
			} else {
				fmt.Printf("%s Skipped %s\n", ui.Muted("·"), displayWorktreePath(item.Target.path))
			}
		default:
			ui.Warnf("Skipped %s: %v", displayWorktreePath(item.Target.path), err)
			skipped++
		}
	}
	return ready, skipped
}

// confirmDiscard asks for the worktree's name, not y/N: approving it loses work.
func confirmDiscard(target removalTarget, unsafe *unsafeRemovalError) bool {
	name := filepath.Base(target.path)
	fmt.Println(ui.Red(name + " has work that removal would discard:"))
	for _, problem := range unsafe.problems {
		fmt.Println("  " + ui.Subtle("-") + " " + problem)
	}
	return ui.PromptDangerous(fmt.Sprintf("Type %s to discard it and remove the worktree, or press Enter to skip:", ui.Bold(name)), name)
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
			if items[0].Target.renamedUpstream() {
				return confirmRenamedUpstream(items[0].Target)
			}
		}
		if !ui.PromptDangerous(fmt.Sprintf("Type %s to confirm:", ui.Bold(expect)), expect) {
			return false
		}
		for _, item := range items[1:] {
			if item.Target.hasBranch() && item.Target.renamedUpstream() && !confirmRenamedUpstream(item.Target) {
				return false
			}
		}
		return true
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

// Typing the local branch name must not delete a differently named, possibly
// shared, remote branch. Require its full remote name instead.
func confirmRenamedUpstream(target removalTarget) bool {
	upstream := target.upstreamLabel()
	fmt.Println(ui.Red(fmt.Sprintf("%s tracks %s, a differently named branch that may be shared.", target.branch, upstream)))
	return ui.PromptDangerous(fmt.Sprintf("Type %s to delete it:", ui.Bold(upstream)), upstream)
}

type removalTarget struct {
	path         string
	branch       string
	head         string
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
		head:         entry.Head,
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

func (t removalTarget) upstreamLabel() string {
	return t.remote + "/" + t.remoteBranch
}

// A differently named upstream is often a shared branch the target was created
// from, such as origin/main or origin/release.
func (t removalTarget) renamedUpstream() bool {
	return t.remote != "" && t.remoteBranch != t.branch
}

// deletesRemote reports whether removal also deletes the target's upstream.
// Cleanup never deletes a differently named upstream: no one confirms it by name.
func (t removalTarget) deletesRemote(opts removeOptions, cleanup bool) bool {
	return opts.deleteRemote && t.hasBranch() && t.remote != "" && !(cleanup && t.renamedUpstream())
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
			if item.Target.deletesRemote(opts, cleanup) {
				remoteDeletes++
			}
		}

		rows = append(rows, []string{
			renderRemovalAction(item.Action),
			displayWorktreePath(item.Target.path),
			item.Target.branchLabel(),
			removalEffect(item, opts, cleanup),
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
	if opts.force && opts.deleteRemote {
		notes = append(notes, ui.Red("FORCE: dirty files, commits without another retained ref, and unfetched commits on deleted remote branches may be lost."))
	} else if opts.force {
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

func removalEffect(item removalItem, opts removeOptions, cleanup bool) string {
	if item.Action == removalActionPrune {
		return ui.Yellow("prune stale metadata")
	}
	if !item.Target.hasBranch() {
		return ui.Yellow("remove worktree only")
	}
	if item.Target.deletesRemote(opts, cleanup) {
		return ui.Red("remove + delete local + " + item.Target.upstreamLabel())
	}
	if opts.deleteRemote && item.Target.remote != "" {
		return ui.Red("remove + delete local") + ui.Yellow(" (keeps "+item.Target.upstreamLabel()+": different name)")
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
			itemOpts := opts
			itemOpts.force = opts.force || item.force
			err = removeSingleWorktree(item.Target, itemOpts, cleanup)
		}
		if err != nil {
			failedCount++
			if len(items) == 1 {
				singleErr = err
			} else {
				ui.Errorf("%s: %v", item.Target.path, err)
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

// checkRemoval verifies that a target is safe to remove and returns what the
// removal needs. It runs before confirmation and again after before-remove
// hooks, which may change the worktree.
func checkRemoval(target removalTarget, opts removeOptions, cleanup bool) (branchHead string, deletions []remoteDeletion, err error) {
	if _, err := preflightRemoveHook(target); err != nil {
		return "", nil, err
	}
	if target.hasBranch() {
		branchHead, err = git.Query("rev-parse", "--verify", "refs/heads/"+target.branch)
		if err != nil {
			return "", nil, err
		}
	}
	// Collect everything a removal would discard, so one confirmation covers it.
	var problems []string
	deleteRemote := target.deletesRemote(opts, cleanup)
	if !opts.force {
		if err := addUnsafeRemoval(&problems, validateRemovalSafety(target, deleteRemote, cleanup)); err != nil {
			return "", nil, err
		}
	}
	if deleteRemote {
		deletions, err = planRemoteDeletions(target, branchHead, opts.force)
		if err := addUnsafeRemoval(&problems, err); err != nil {
			return "", nil, err
		}
	}
	if len(problems) > 0 {
		return "", nil, &unsafeRemovalError{problems: problems}
	}
	return branchHead, deletions, nil
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
	if fresh.remote != target.remote || fresh.remoteBranch != target.remoteBranch {
		return fmt.Errorf("upstream changed since selection: %s", target.path)
	}
	branchHead, deletions, err := checkRemoval(fresh, opts, cleanup)
	if err != nil {
		return err
	}
	deleteRemote := target.deletesRemote(opts, cleanup)
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

		switch {
		case deleteRemote:
			if err := deleteRemoteBranches(target.remoteBranch, target.remote, deletions); err != nil {
				return err
			}
		case opts.deleteRemote && target.remote != "":
			fmt.Printf("%s Kept %s: cleanup does not delete a differently named upstream\n", ui.Muted("·"), target.upstreamLabel())
		case opts.deleteRemote:
			fmt.Printf("%s %s\n", ui.Muted("·"), ui.Muted("No remote upstream configured; remote deletion skipped"))
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

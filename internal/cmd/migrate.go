package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/ahmedelgabri/git-wt/internal/fsutil"
	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/ui"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:           "migrate",
	Short:         "Migrate an existing repository to use worktrees [EXPERIMENTAL]",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runMigrate,
}

func init() {
	migrateCmd.Flags().Bool("dry-run", false, "Show migration plan without making changes")
	rootCmd.AddCommand(migrateCmd)
}

type migratePlan struct {
	repoRoot       string
	repoName       string
	parentDir      string
	currentBranch  string
	defaultBranch  string
	defaultRemote  string
	defaultURL     string
	hasChanges     bool
	untrackedFiles []string
	stashCount     int
	remotes        []preservedRemote
	configs        []preservedConfig
	warnings       []string
}

type preservedRemote struct {
	Name       string
	URL        string
	FetchSpecs []string
}

type preservedConfig struct {
	Key    string
	Values []string
}

var preservedConfigPrefixes = []string{
	"alias.",
	"branch.",
	"diff.",
	"merge.",
	"pull.",
	"rebase.",
	"rerere.",
}

var preservedConfigKeys = map[string]bool{
	"core.hookspath": true,
	"user.email":     true,
	"user.name":      true,
}

func runMigrate(cmd *cobra.Command, args []string) error {
	if _, err := git.Query("rev-parse", "--git-dir"); err != nil {
		ui.Error("Not in a git repository")
		return fmt.Errorf("not in a git repository")
	}

	repoRoot, err := git.Query("rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	// Resolve symlinks (macOS /tmp -> /private/tmp).
	repoRoot, err = filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return err
	}

	plan, err := buildMigratePlan(repoRoot)
	if err != nil {
		return err
	}

	fmt.Println(renderMigratePlan(plan))

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		fmt.Printf("%s No changes made\n", ui.Yellow("[DRY RUN]"))
		return nil
	}

	// Confirm.
	if !ui.Confirm("This will restructure the repository. Continue? [y/N]:") {
		fmt.Println("Cancelled")
		return nil
	}

	newStructure := filepath.Join(plan.parentDir, fmt.Sprintf("%s-new-%d", plan.repoName, os.Getpid()))
	tempBackup := filepath.Join(plan.parentDir, fmt.Sprintf("%s-backup-%d", plan.repoName, os.Getpid()))

	// Setup cleanup on signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			if _, err := os.Stat(newStructure); err == nil {
				_ = os.RemoveAll(newStructure)
			}
			if _, err := os.Stat(tempBackup); err == nil {
				restoreBackup(tempBackup, plan.repoRoot)
			}
		})
	}

	go func() {
		<-sigCh
		cleanup()
		os.Exit(1)
	}()

	// Ensure cleanup on any error.
	success := false
	defer func() {
		signal.Stop(sigCh)
		if !success {
			cleanup()
		}
	}()

	if err := buildMigratedStructure(plan, newStructure); err != nil {
		return err
	}

	requiredEntries := []string{".git", ".bare", plan.currentBranch}
	if plan.defaultBranch != "" && plan.defaultBranch != plan.currentBranch {
		requiredEntries = append(requiredEntries, plan.defaultBranch)
	}

	fmt.Println()
	if err := ui.RunSteps([]ui.Step{{
		Message: "Finalizing migration",
		Run: func(context.Context, io.Writer) error {
			return finalizeMigration(plan.repoRoot, newStructure, tempBackup, requiredEntries)
		},
	}}); err != nil {
		return err
	}

	fmt.Println()
	ui.Success("Migration complete")

	var branches []treeBranch
	if plan.defaultBranch != "" && plan.defaultBranch == plan.currentBranch {
		branches = append(branches, treeBranch{plan.currentBranch, "active worktree"})
	} else {
		if plan.defaultBranch != "" {
			branches = append(branches, treeBranch{plan.defaultBranch, "default branch"})
		}
		branches = append(branches, treeBranch{plan.currentBranch, "current branch"})
	}

	fmt.Println()
	fmt.Println(renderRepoLayoutSection(".", branches))

	if outcome := renderMigrateOutcome(plan); outcome != "" {
		fmt.Println()
		fmt.Println(outcome)
	}

	hints := []commandHint{{
		Action:  "Create another worktree",
		Command: fmt.Sprintf("cd %s && git wt add <branch-name> <branch-name>", plan.repoRoot),
	}, {
		Action:  "Open your worktree",
		Command: fmt.Sprintf("cd %s/%s", plan.repoRoot, plan.currentBranch),
	}}
	if plan.stashCount > 0 {
		hints = append(hints, commandHint{
			Action:  "Review migrated stashes",
			Command: fmt.Sprintf("cd %s/%s && git stash list", plan.repoRoot, plan.currentBranch),
		})
	}

	fmt.Println()
	fmt.Println(renderCommandHintsSection(hints))

	success = true
	return nil
}

func buildMigratePlan(repoRoot string) (migratePlan, error) {
	plan := migratePlan{
		repoRoot:  repoRoot,
		repoName:  filepath.Base(repoRoot),
		parentDir: filepath.Dir(repoRoot),
	}

	currentBranch, err := git.QueryIn(repoRoot, "branch", "--show-current")
	if err != nil || currentBranch == "" {
		ui.Error("Not on a branch (detached HEAD state). Please check out a branch first.")
		return migratePlan{}, fmt.Errorf("detached HEAD state")
	}
	plan.currentBranch = currentBranch

	warnings, err := preflightMigrateRepo(repoRoot)
	if err != nil {
		return migratePlan{}, err
	}
	plan.warnings = warnings

	plan.remotes, err = captureRemotes(repoRoot)
	if err != nil {
		return migratePlan{}, err
	}
	plan.configs, err = capturePreservedConfig(repoRoot)
	if err != nil {
		return migratePlan{}, err
	}

	plan.defaultRemote = worktree.DefaultRemote()
	if plan.defaultRemote != "" {
		plan.defaultURL, _ = git.QueryIn(repoRoot, "remote", "get-url", plan.defaultRemote)
	}
	plan.defaultBranch = worktree.DefaultBranch(plan.defaultRemote)

	if err := checkGitDiff(repoRoot); err != nil {
		plan.hasChanges = true
	}
	plan.untrackedFiles, _ = git.QueryLines("-C", repoRoot, "ls-files", "--others", "--exclude-standard")
	stashList, _ := git.QueryLines("-C", repoRoot, "stash", "list")
	plan.stashCount = len(stashList)

	return plan, nil
}

func renderMigratePlan(plan migratePlan) string {
	rows := [][]string{
		{"Repository", ui.Bold(plan.repoName)},
		{"Path", ui.Path(plan.repoRoot)},
		{"Current branch", ui.Accent(plan.currentBranch)},
	}
	if plan.defaultBranch != "" {
		rows = append(rows, []string{"Default branch", ui.Accent(plan.defaultBranch)})
	}
	if plan.defaultRemote != "" {
		rows = append(rows, []string{"Default remote", plan.defaultRemote})
	}
	if plan.defaultURL != "" {
		rows = append(rows, []string{"Remote URL", plan.defaultURL})
	}

	notes := make([]string, 0, len(plan.warnings)+4)
	if plan.defaultRemote == "" {
		notes = append(notes, ui.Yellow("! no remote found"))
	}
	if plan.hasChanges {
		notes = append(notes, ui.Yellow("! uncommitted changes will be preserved"))
	}
	if len(plan.untrackedFiles) > 0 {
		notes = append(notes, ui.Yellow(fmt.Sprintf("! %d untracked file(s) will be preserved", len(plan.untrackedFiles))))
	}
	if plan.stashCount > 0 {
		notes = append(notes, ui.Yellow(fmt.Sprintf("! %d stash(es) will be migrated", plan.stashCount)))
	}
	for _, warning := range plan.warnings {
		notes = append(notes, ui.Yellow("! "+warning))
	}

	summaryParts := []string{}
	if len(plan.remotes) > 0 {
		summaryParts = append(summaryParts, ui.Subtle(fmt.Sprintf("%d remote(s)", len(plan.remotes))))
	}
	if len(plan.configs) > 0 {
		summaryParts = append(summaryParts, ui.Subtle(fmt.Sprintf("%d config entries preserved", len(plan.configs))))
	}
	summary := strings.Join(summaryParts, " • ")

	return renderTableSection([]ui.TableColumn{
		{Title: "ITEM", MinWidth: 16, MaxWidth: 20},
		{Title: "DETAIL", MinWidth: 28, MaxWidth: 64},
	}, rows, notes, summary)
}

func renderMigrateOutcome(plan migratePlan) string {
	rows := make([][]string, 0, 5)
	if plan.defaultURL != "" {
		rows = append(rows, []string{"Remote URL", plan.defaultURL})
	}
	if len(plan.remotes) > 1 {
		rows = append(rows, []string{"Remotes", fmt.Sprintf("Preserved %d remotes", len(plan.remotes))})
	}
	if len(plan.configs) > 0 {
		rows = append(rows, []string{"Config", fmt.Sprintf("Preserved %d repo-local entries", len(plan.configs))})
	}
	if plan.stashCount > 0 {
		rows = append(rows, []string{"Stashes", fmt.Sprintf("Migrated %d stash(es)", plan.stashCount)})
	}
	if plan.hasChanges {
		rows = append(rows, []string{"Working tree", fmt.Sprintf("Preserved uncommitted changes in %s", plan.currentBranch)})
	}
	if len(plan.untrackedFiles) > 0 {
		rows = append(rows, []string{"Untracked files", fmt.Sprintf("Preserved %d file(s) in %s", len(plan.untrackedFiles), plan.currentBranch)})
	}
	if len(rows) == 0 {
		return ""
	}

	summary := ui.Green("Migration artifacts preserved")
	return renderTableSection([]ui.TableColumn{
		{Title: "OUTCOME", MinWidth: 16, MaxWidth: 20},
		{Title: "DETAIL", MinWidth: 28, MaxWidth: 64},
	}, rows, nil, summary)
}

func preflightMigrateRepo(repoRoot string) ([]string, error) {
	gitPath := filepath.Join(repoRoot, ".git")
	gitInfo, err := os.Stat(gitPath)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", gitPath, err)
	}
	if !gitInfo.IsDir() {
		return nil, fmt.Errorf("unsupported repository layout: %s must be a directory", gitPath)
	}

	if _, err := os.Stat(filepath.Join(repoRoot, ".gitmodules")); err == nil {
		return nil, fmt.Errorf("repositories with submodules are not supported by migrate")
	}

	worktreeOut, err := git.QueryIn(repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	if strings.Count(worktreeOut, "worktree ") > 1 {
		return nil, fmt.Errorf("repositories with linked worktrees are not supported by migrate")
	}

	if sparseEnabled, _ := git.QueryIn(repoRoot, "config", "--bool", "core.sparseCheckout"); sparseEnabled == "true" {
		return nil, fmt.Errorf("repositories using sparse checkout are not supported by migrate")
	}
	if _, err := os.Stat(filepath.Join(gitPath, "info", "sparse-checkout")); err == nil {
		return nil, fmt.Errorf("repositories using sparse checkout are not supported by migrate")
	}

	if _, err := os.Stat(filepath.Join(gitPath, "objects", "info", "alternates")); err == nil {
		return nil, fmt.Errorf("repositories using alternate object directories are not supported by migrate")
	}

	var warnings []string
	if remotes, _ := captureRemotes(repoRoot); len(remotes) > 1 {
		warnings = append(warnings, fmt.Sprintf("Detected %d remotes; all remotes and fetch specs will be preserved", len(remotes)))
	}
	if len(warnings) == 0 {
		return nil, nil
	}
	return warnings, nil
}

func buildMigratedStructure(plan migratePlan, newStructure string) error {
	if err := os.MkdirAll(newStructure, 0o755); err != nil {
		return err
	}

	if err := ui.RunSteps([]ui.Step{{
		Message:    "Converting to bare repository",
		ShowOutput: true,
		RawOutput:  true,
		Run: func(ctx context.Context, w io.Writer) error {
			return git.RunToContext(ctx, w, "clone", "--bare", plan.repoRoot, filepath.Join(newStructure, ".bare"))
		},
	}, {
		Message: "Configuring migrated worktree layout",
		Run: func(context.Context, io.Writer) error {
			if err := os.WriteFile(filepath.Join(newStructure, ".git"), []byte("gitdir: ./.bare\n"), 0o644); err != nil {
				return err
			}
			if err := configureBareRepo(newStructure); err != nil {
				return err
			}
			if err := applyRemotes(newStructure, plan.remotes); err != nil {
				return err
			}
			return applyPreservedConfig(newStructure, plan.configs)
		},
	}, {
		Message:    "Fetching all branches from preserved remotes",
		ShowOutput: true,
		RawOutput:  true,
		Run: func(ctx context.Context, w io.Writer) error {
			if len(plan.remotes) == 0 {
				return nil
			}
			if err := git.RunInToContext(ctx, newStructure, w, "fetch", "--all"); err != nil {
				ui.Warn("Could not fetch from remote (remote may be unreachable) - continuing with local data")
			}
			return nil
		},
	}}); err != nil {
		return err
	}

	cleanupLocalBranchRefs(newStructure)

	// Migrate stashes.
	if plan.stashCount > 0 {
		if err := ui.Spin(fmt.Sprintf("Migrating %d stash(es)", plan.stashCount), func() error {
			oldGitDir := filepath.Join(plan.repoRoot, ".git")
			newBareDir := filepath.Join(newStructure, ".bare")

			stashRef := filepath.Join(oldGitDir, "refs", "stash")
			if _, err := os.Stat(stashRef); err == nil {
				if err := copyFileSimple(stashRef, filepath.Join(newBareDir, "refs", "stash")); err != nil {
					return fmt.Errorf("failed to copy stash ref: %w", err)
				}
			}

			stashLog := filepath.Join(oldGitDir, "logs", "refs", "stash")
			if _, err := os.Stat(stashLog); err == nil {
				if err := os.MkdirAll(filepath.Join(newBareDir, "logs", "refs"), 0o755); err != nil {
					return err
				}
				if err := copyFileSimple(stashLog, filepath.Join(newBareDir, "logs", "refs", "stash")); err != nil {
					return fmt.Errorf("failed to copy stash log: %w", err)
				}
			}
			return nil
		}); err != nil {
			ui.Warnf("Stash migration failed: %s", err)
		}
	}

	// Create worktrees.
	if plan.defaultBranch != "" && plan.defaultBranch == plan.currentBranch {
		if err := createMigrationWorktree(newStructure, plan.currentBranch, plan.defaultRemote); err != nil {
			return err
		}
	} else {
		if plan.defaultBranch != "" {
			if err := createMigrationWorktree(newStructure, plan.defaultBranch, plan.defaultRemote); err != nil {
				return err
			}
		}
		if err := createMigrationWorktree(newStructure, plan.currentBranch, plan.defaultRemote); err != nil {
			return err
		}
	}

	// Restore working directory state.
	destDir := filepath.Join(newStructure, plan.currentBranch)
	if err := ui.RunSteps([]ui.Step{{
		Message: "Restoring working directory state",
		Run: func(context.Context, io.Writer) error {
			if err := fsutil.CopyDir(plan.repoRoot, destDir, []string{".git"}); err != nil {
				return fmt.Errorf("failed to copy working directory: %w", err)
			}
			// Restore git index (staged changes).
			oldIndex := filepath.Join(plan.repoRoot, ".git", "index")
			if _, err := os.Stat(oldIndex); err == nil {
				newIndex, err := git.QueryIn(destDir, "rev-parse", "--git-path", "index")
				if err != nil {
					return fmt.Errorf("failed to resolve migrated git index: %w", err)
				}
				if newIndex == "" {
					return fmt.Errorf("failed to resolve migrated git index")
				}
				if !filepath.IsAbs(newIndex) {
					newIndex = filepath.Join(destDir, newIndex)
				}
				if err := copyFileSimple(oldIndex, newIndex); err != nil {
					return fmt.Errorf("failed to restore git index: %w", err)
				}
			}
			return nil
		},
	}}); err != nil {
		return err
	}

	return nil
}

func createMigrationWorktree(repoDir, branch, preferredRemote string) error {
	sourceRef := migrationBranchSourceRef(repoDir, branch, preferredRemote)
	return ui.SpinWithOutputContext(fmt.Sprintf("Creating worktree for %s", ui.Accent(branch)), func(ctx context.Context, w io.Writer) error {
		if sourceRef == branch {
			return git.RunInToContext(ctx, repoDir, w, "worktree", "add", branch, branch)
		}
		return git.RunInToContext(ctx, repoDir, w, "worktree", "add", "-b", branch, branch, sourceRef)
	})
}

func migrationBranchSourceRef(repoDir, branch, preferredRemote string) string {
	if _, err := git.QueryIn(repoDir, "show-ref", "--verify", "refs/heads/"+branch); err == nil {
		return branch
	}
	if preferredRemote != "" {
		ref := "refs/remotes/" + preferredRemote + "/" + branch
		if _, err := git.QueryIn(repoDir, "show-ref", "--verify", ref); err == nil {
			return preferredRemote + "/" + branch
		}
	}

	remoteRefs, _ := git.QueryIn(repoDir, "for-each-ref", "--format=%(refname:short)", "refs/remotes")
	for remoteRef := range strings.SplitSeq(remoteRefs, "\n") {
		remoteRef = strings.TrimSpace(remoteRef)
		if remoteRef == "" || strings.HasSuffix(remoteRef, "/HEAD") {
			continue
		}
		if _, remoteBranch, ok := strings.Cut(remoteRef, "/"); ok && remoteBranch == branch {
			return remoteRef
		}
	}
	return branch
}

func captureRemotes(repoRoot string) ([]preservedRemote, error) {
	out, err := git.QueryIn(repoRoot, "remote")
	if err != nil || out == "" {
		return nil, nil
	}

	var remotes []preservedRemote
	for name := range strings.SplitSeq(out, "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		url, err := git.QueryIn(repoRoot, "remote", "get-url", name)
		if err != nil {
			return nil, err
		}
		fetchOut, err := git.QueryIn(repoRoot, "config", "--get-all", "remote."+name+".fetch")
		fetchSpecs := []string(nil)
		if err == nil && fetchOut != "" {
			fetchSpecs = strings.Split(fetchOut, "\n")
		}
		remotes = append(remotes, preservedRemote{Name: name, URL: url, FetchSpecs: fetchSpecs})
	}
	return remotes, nil
}

func applyRemotes(dir string, remotes []preservedRemote) error {
	out, err := git.QueryIn(dir, "remote")
	if err == nil && out != "" {
		for name := range strings.SplitSeq(out, "\n") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if _, err := git.RunInWithOutput(dir, "remote", "remove", name); err != nil {
				return err
			}
		}
	}

	for _, remote := range remotes {
		if _, err := git.RunInWithOutput(dir, "remote", "add", remote.Name, remote.URL); err != nil {
			return err
		}
		if len(remote.FetchSpecs) == 0 {
			continue
		}
		key := "remote." + remote.Name + ".fetch"
		_, _ = git.RunInWithOutput(dir, "config", "--unset-all", key)
		for _, spec := range remote.FetchSpecs {
			if _, err := git.RunInWithOutput(dir, "config", "--add", key, spec); err != nil {
				return err
			}
		}
	}
	return nil
}

func capturePreservedConfig(repoRoot string) ([]preservedConfig, error) {
	out, err := git.QueryIn(repoRoot, "config", "--local", "--list")
	if err != nil || out == "" {
		return nil, nil
	}

	order := []string{}
	values := map[string][]string{}
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || !shouldPreserveConfig(key) {
			continue
		}
		if _, exists := values[key]; !exists {
			order = append(order, key)
		}
		values[key] = append(values[key], value)
	}

	preserved := make([]preservedConfig, 0, len(order))
	for _, key := range order {
		preserved = append(preserved, preservedConfig{Key: key, Values: values[key]})
	}
	return preserved, nil
}

func shouldPreserveConfig(key string) bool {
	key = strings.ToLower(key)
	if preservedConfigKeys[key] {
		return true
	}
	for _, prefix := range preservedConfigPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func applyPreservedConfig(dir string, configs []preservedConfig) error {
	for _, cfg := range configs {
		_, _ = git.RunInWithOutput(dir, "config", "--unset-all", cfg.Key)
		for i, value := range cfg.Values {
			var err error
			if i == 0 {
				_, err = git.RunInWithOutput(dir, "config", cfg.Key, value)
			} else {
				_, err = git.RunInWithOutput(dir, "config", "--add", cfg.Key, value)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func finalizeMigration(repoRoot, newStructure, tempBackup string, requiredEntries []string) error {
	if err := os.MkdirAll(tempBackup, 0o755); err != nil {
		return err
	}

	backupNames, err := moveContentsTracked(repoRoot, tempBackup)
	if err != nil {
		return fmt.Errorf("failed to backup original repo: %w", err)
	}

	promotedNames, err := moveContentsTracked(newStructure, repoRoot)
	if err != nil {
		_ = restoreNamedEntries(tempBackup, repoRoot, backupNames)
		return fmt.Errorf("failed to move new structure: %w", err)
	}

	if err := validateMigratedLayout(repoRoot, requiredEntries); err != nil {
		_ = rollbackFinalization(repoRoot, newStructure, tempBackup, promotedNames, backupNames)
		return err
	}

	if err := os.Remove(newStructure); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.RemoveAll(tempBackup)
}

func validateMigratedLayout(repoRoot string, requiredEntries []string) error {
	gitInfo, err := os.Stat(filepath.Join(repoRoot, ".git"))
	if err != nil {
		return err
	}
	if gitInfo.IsDir() {
		return fmt.Errorf("migration validation failed: .git should be a file after migration")
	}
	for _, rel := range requiredEntries {
		if _, err := os.Stat(filepath.Join(repoRoot, rel)); err != nil {
			return fmt.Errorf("migration validation failed: missing %s", rel)
		}
	}
	return nil
}

func rollbackFinalization(repoRoot, newStructure, tempBackup string, promotedNames, backupNames []string) error {
	if err := restoreNamedEntries(repoRoot, newStructure, promotedNames); err != nil {
		_ = removeNamedEntries(repoRoot, promotedNames)
	}
	return restoreNamedEntries(tempBackup, repoRoot, backupNames)
}

func checkGitDiff(repoRoot string) error {
	_, err := git.QueryIn(repoRoot, "diff-index", "--quiet", "HEAD", "--")
	return err
}

func copyFileSimple(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode().Perm())
}

func moveContents(src, dst string) error {
	_, err := moveContentsTracked(src, dst)
	return err
}

func moveContentsTracked(src, dst string) ([]string, error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil, err
	}
	moved := make([]string, 0, len(entries))
	for _, entry := range entries {
		oldPath := filepath.Join(src, entry.Name())
		newPath := filepath.Join(dst, entry.Name())
		if err := os.Rename(oldPath, newPath); err != nil {
			_ = restoreNamedEntries(dst, src, moved)
			return nil, err
		}
		moved = append(moved, entry.Name())
	}
	return moved, nil
}

func restoreNamedEntries(src, dst string, names []string) error {
	for i := len(names) - 1; i >= 0; i-- {
		if err := os.Rename(filepath.Join(src, names[i]), filepath.Join(dst, names[i])); err != nil {
			return err
		}
	}
	return nil
}

func removeNamedEntries(root string, names []string) error {
	for _, name := range names {
		if err := os.RemoveAll(filepath.Join(root, name)); err != nil {
			return err
		}
	}
	return nil
}

func restoreBackup(backup, repoRoot string) {
	entries, err := os.ReadDir(backup)
	if err != nil {
		return
	}
	for _, entry := range entries {
		_ = os.Rename(filepath.Join(backup, entry.Name()), filepath.Join(repoRoot, entry.Name()))
	}
	_ = os.RemoveAll(backup)
}

package cmd

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ahmedelgabri/git-wt/internal/fsutil"
	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/ui"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use: "migrate", Short: "Migrate an existing repository to use worktrees [EXPERIMENTAL]",
	SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs, RunE: runMigrate,
}

func init() {
	migrateCmd.Flags().Bool("dry-run", false, "Show migration plan without making changes")
	rootCmd.AddCommand(migrateCmd)
}

type migrationGitState struct {
	refs, index, stashes string
}

type migratePlan struct {
	migrationGitState
	repoRoot      string
	currentBranch string
	defaultBranch string
	defaultRemote string
	files         map[string]fsutil.FileState
	gitFiles      map[string]fsutil.FileState
	objectFiles   map[string]fsutil.FileState
	config        migrationConfigState
}

func runMigrate(cmd *cobra.Command, args []string) error {
	repoRoot, err := git.QueryPath("rev-parse", "--show-toplevel")
	if err != nil {
		ui.Error("Not in a git repository")
		return fmt.Errorf("not in a git repository: %w", err)
	}
	repoRoot, err = filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	plan, err := buildMigratePlan(ctx, repoRoot)
	if err != nil {
		return err
	}
	fmt.Printf("Repository: %s\nCurrent branch: %s\n", repoRoot, plan.currentBranch)
	fmt.Println("The complete Git database and working directory will be copied and verified.")
	fmt.Println("Stop other Git operations and file writers before continuing. The original repository will be retained as a backup.")
	if boolFlag(cmd, "dry-run") || git.Debug() {
		fmt.Println("[DRY RUN] No changes made")
		return nil
	}
	if !ui.Confirm("This will restructure the repository. Continue? [y/N]:") {
		fmt.Println("Cancelled")
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	parent := filepath.Dir(repoRoot)
	newStructure, err := os.MkdirTemp(parent, filepath.Base(repoRoot)+"-new-")
	if err != nil {
		return err
	}
	// Staging is disposable only before promotion. Once finalization starts,
	// both staging and backup may contain recovery data and must be retained.
	finalizing := false
	defer func() {
		if !finalizing {
			_ = os.RemoveAll(newStructure)
		}
	}()
	if err := buildMigratedStructure(ctx, plan, newStructure); err != nil {
		return err
	}
	if err := verifyMigration(ctx, plan, newStructure); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	backup, err := os.MkdirTemp(parent, filepath.Base(repoRoot)+"-backup-")
	if err != nil {
		return err
	}
	required := []string{".git", ".bare", plan.currentBranch}
	if plan.defaultBranch != "" && plan.defaultBranch != plan.currentBranch {
		required = append(required, plan.defaultBranch)
	}
	finalizing = true
	// Finalization runs synchronously. Signals cancel preparation, never race a
	// second cleanup goroutine against the directory renames below.
	if err := finalizeMigration(repoRoot, newStructure, backup, required, migrationMoves{rename: renameEntry}, func() error { return verifyMigrationState(context.Background(), plan, repoRoot) }); err != nil {
		return migrationRecoveryError(err, repoRoot, newStructure, backup)
	}
	ui.Success("Migration complete")
	fmt.Printf("Original repository retained at %s\n", backup)
	fmt.Println("Keep this backup until you have checked your worktrees, hooks, and configuration.")
	branches := []treeBranch{{plan.currentBranch, "current branch"}}
	if plan.defaultBranch != "" && plan.defaultBranch != plan.currentBranch {
		branches = append(branches, treeBranch{plan.defaultBranch, "default branch"})
	}
	fmt.Println(renderRepoLayoutSection(".", branches))
	fmt.Println(renderCommandHintsSection([]commandHint{
		{Action: "Create another worktree", Command: fmt.Sprintf("cd %s && git wt add <branch-name> <branch-name>", shellQuote(repoRoot))},
		{Action: "Open your worktree", Command: "cd " + shellQuote(filepath.Join(repoRoot, plan.currentBranch))},
	}))
	return nil
}

func buildMigratePlan(ctx context.Context, root string) (migratePlan, error) {
	plan := migratePlan{repoRoot: root}
	if err := preflightMigrateRepo(root); err != nil {
		return plan, err
	}
	config, err := readMigrationConfig(ctx, root)
	if err != nil {
		return plan, err
	}
	plan.config = config
	branch, err := git.QueryInContext(ctx, root, "branch", "--show-current")
	if err != nil || branch == "" {
		return plan, fmt.Errorf("detached HEAD state: check out a branch before migrating")
	}
	plan.currentBranch = branch
	plan.defaultRemote = worktree.DefaultRemoteInContext(ctx, root)
	plan.defaultBranch = worktree.DefaultBranchInContext(ctx, root, plan.defaultRemote)
	// An offline migration must not depend on a remote default branch whose
	// objects are unavailable locally.
	if plan.defaultBranch != "" {
		if _, err := git.QueryInContext(ctx, root, "rev-parse", "--verify", "refs/heads/"+plan.defaultBranch); err != nil {
			if _, err := git.QueryInContext(ctx, root, "rev-parse", "--verify", "refs/remotes/"+plan.defaultRemote+"/"+plan.defaultBranch); err != nil {
				plan.defaultBranch = ""
			}
		}
	}
	plan.migrationGitState, err = readMigrationGitState(ctx, root)
	if err != nil {
		return plan, err
	}
	plan.gitFiles, err = fsutil.Snapshot(ctx, filepath.Join(root, ".git"), []string{"objects"})
	if err != nil {
		return plan, err
	}
	plan.objectFiles, err = fsutil.SnapshotMetadata(ctx, filepath.Join(root, ".git", "objects"), nil)
	if err != nil {
		return plan, err
	}
	plan.files, err = fsutil.Snapshot(ctx, root, []string{".git"})
	return plan, err
}

func preflightMigrateRepo(root string) error {
	gitPath := filepath.Join(root, ".git")
	info, err := os.Lstat(gitPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("unsupported repository layout: %s must be a directory", gitPath)
	}
	for _, control := range []string{"commondir", "gitdir"} {
		if _, err := os.Lstat(filepath.Join(gitPath, control)); err == nil {
			return fmt.Errorf("unsupported Git layout: %s contains linked-worktree control file %s", gitPath, control)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".gitmodules")); err == nil {
		return fmt.Errorf("repositories with submodules are not supported by migrate")
	}
	out, err := git.QueryIn(root, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return err
	}
	if len(worktree.ParsePorcelain(out)) > 1 {
		return fmt.Errorf("repositories with linked worktrees are not supported by migrate")
	}
	if enabled, _ := git.QueryIn(root, "config", "--bool", "core.sparseCheckout"); enabled == "true" {
		return fmt.Errorf("repositories using sparse checkout are not supported by migrate")
	}
	for _, item := range []struct{ path, reason string }{
		{"info/sparse-checkout", "sparse checkout"},
		{"objects/info/alternates", "alternate object directories"},
		{"rebase-merge", "an in-progress rebase"},
		{"rebase-apply", "an in-progress rebase or am"},
		{"MERGE_HEAD", "an in-progress merge"},
		{"CHERRY_PICK_HEAD", "an in-progress cherry-pick"},
		{"REVERT_HEAD", "an in-progress revert"},
		{"sequencer", "an in-progress sequencer operation"},
	} {
		if _, err := os.Stat(filepath.Join(gitPath, item.path)); err == nil {
			return fmt.Errorf("repositories using %s are not supported by migrate", item.reason)
		}
	}
	if format, _ := git.QueryIn(root, "config", "--get", "extensions.refStorage"); format != "" && format != "files" {
		return fmt.Errorf("migration does not support the %s ref storage format", format)
	}
	if enabled, _ := git.QueryIn(root, "config", "--bool", "extensions.worktreeConfig"); enabled == "true" {
		return fmt.Errorf("repositories using per-worktree config are not supported by migrate")
	}
	if _, err := git.QueryIn(root, "rev-parse", "--verify", "HEAD"); err != nil {
		return fmt.Errorf("repository needs an initial commit before migration: %w", err)
	}
	if err := checkMigrationIncludes(context.Background(), root); err != nil {
		return err
	}
	// Refuse active Git writes rather than copying their intermediate state.
	return filepath.WalkDir(gitPath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinked Git metadata is not supported by migrate: %s", path)
		}
		if strings.HasSuffix(entry.Name(), ".lock") {
			return fmt.Errorf("git lock present at %s; stop other Git operations before migrating", path)
		}
		return fsutil.CheckMetadata(path)
	})
}

func buildMigratedStructure(ctx context.Context, plan migratePlan, dest string) error {
	ui.Info("Copying repository database and working directory")
	// Copy the database rather than cloning it: cloning drops reflogs, custom
	// refs, packed stash refs, local hooks, and repository configuration.
	if err := fsutil.CopyDirContext(ctx, filepath.Join(plan.repoRoot, ".git"), filepath.Join(dest, ".bare"), nil); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dest, ".git"), []byte("gitdir: ./.bare\n"), 0o644); err != nil {
		return err
	}
	for _, kv := range [][2]string{{"core.bare", "true"}, {"core.logallrefupdates", "true"}, {"worktree.useRelativePaths", "true"}} {
		if _, err := git.RunInWithOutputContext(ctx, dest, "config", kv[0], kv[1]); err != nil {
			return err
		}
	}
	if _, err := git.QueryInContext(ctx, dest, "config", "--get", "core.worktree"); err == nil {
		if _, err := git.RunInWithOutputContext(ctx, dest, "config", "--unset-all", "core.worktree"); err != nil {
			return err
		}
	}
	if err := normalizeMigrationRemoteURLs(ctx, plan, dest); err != nil {
		return err
	}
	if err := removeMigrationCommonPrivateRefs(ctx, plan, dest); err != nil {
		return err
	}
	if err := createMigrationWorktree(ctx, dest, plan.currentBranch, "", true); err != nil {
		return err
	}
	if plan.defaultBranch != "" && plan.defaultBranch != plan.currentBranch {
		if err := createMigrationWorktree(ctx, dest, plan.defaultBranch, plan.defaultRemote, false); err != nil {
			return err
		}
	}
	wt := filepath.Join(dest, plan.currentBranch)
	if err := fsutil.CopyDirContext(ctx, plan.repoRoot, wt, []string{".git"}); err != nil {
		return err
	}
	return copyMigrationWorktreeMetadata(ctx, plan, wt)
}

func createMigrationWorktree(ctx context.Context, root, branch, remote string, empty bool) error {
	args := []string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "worktree", "add"}
	if empty {
		args = append(args, "--no-checkout")
	}
	source := branch
	if _, err := git.QueryInContext(ctx, root, "show-ref", "--verify", "refs/heads/"+branch); err != nil {
		source = "refs/remotes/" + remote + "/" + branch
		args = append(args, "-b", branch)
	}
	args = append(args, "--", branch, source)
	out, err := git.RunInWithOutputContext(ctx, root, args...)
	if err != nil {
		return fmt.Errorf("create migration worktree: %s: %w", out, err)
	}
	return nil
}

func normalizeMigrationRemoteURLs(ctx context.Context, plan migratePlan, dest string) error {
	prefixes := migrationURLPrefixes(plan.config.values)
	for key, values := range plan.config.values {
		if !migrationRemoteURLKey(key) {
			continue
		}
		urls := make([]string, len(values))
		changed := false
		for i, value := range values {
			url := strings.TrimPrefix(value, "\n")
			urls[i] = migrationURL(plan.repoRoot, url, prefixes)
			changed = changed || urls[i] != url
		}
		if !changed {
			continue
		}
		if _, err := git.RunInWithOutputContext(ctx, dest, "config", "--unset-all", key); err != nil {
			return err
		}
		for _, url := range urls {
			if _, err := git.RunInWithOutputContext(ctx, dest, "config", "--add", key, url); err != nil {
				return err
			}
		}
	}
	return nil
}

func readMigrationGitState(ctx context.Context, root string) (state migrationGitState, err error) {
	state.refs, err = git.QueryInContext(ctx, root, "for-each-ref", "--format=%(refname) %(objectname) %(symref)")
	if err != nil {
		return state, err
	}
	state.index, err = git.QueryRawInContext(ctx, root, "-c", "core.fsmonitor=false", "--no-optional-locks", "ls-files", "--stage", "-z")
	if err != nil {
		return state, err
	}
	state.stashes, err = git.QueryInContext(ctx, root, "stash", "list", "--format=%H %gs")
	return state, err
}

func verifyMigration(ctx context.Context, plan migratePlan, dest string) error {
	if err := verifyMigrationState(ctx, plan, dest); err != nil {
		return err
	}
	// Detect changes to the source while preparation was running.
	state, err := readMigrationGitState(ctx, plan.repoRoot)
	if err != nil {
		return err
	}
	if state != plan.migrationGitState {
		return fmt.Errorf("source repository changed during migration; original left untouched")
	}
	if err := fsutil.VerifySnapshot(ctx, plan.repoRoot, []string{".git"}, plan.files); err != nil {
		return err
	}
	if err := fsutil.VerifySnapshot(ctx, filepath.Join(plan.repoRoot, ".git"), []string{"objects"}, plan.gitFiles); err != nil {
		return err
	}
	objects, err := fsutil.SnapshotMetadata(ctx, filepath.Join(plan.repoRoot, ".git", "objects"), nil)
	if err != nil {
		return err
	}
	if !maps.Equal(objects, plan.objectFiles) {
		return fmt.Errorf("source Git object metadata changed during migration; original left untouched")
	}
	return verifyMigrationConfig(ctx, plan, plan.repoRoot, true)
}

func verifyMigrationState(ctx context.Context, plan migratePlan, dest string) error {
	wt := filepath.Join(dest, plan.currentBranch)
	for _, root := range []string{dest, wt} {
		if err := verifyMigrationConfig(ctx, plan, root, false); err != nil {
			return err
		}
	}
	if err := verifyMigrationPrivateRefIsolation(ctx, dest); err != nil {
		return err
	}
	// Run Git's checks before the final filesystem snapshots. Even read-only
	// commands can refresh caches; optional index writes are disabled below.
	if _, err := git.QueryInContext(ctx, wt, "-c", "core.fsmonitor=false", "--no-optional-locks", "status", "--porcelain=v2", "-z"); err != nil {
		return err
	}
	if out, err := git.QueryInContext(ctx, wt, "-c", "core.fsmonitor=false", "--no-optional-locks", "fsck", "--connectivity-only", "--no-dangling"); err != nil {
		return fmt.Errorf("migration object verification failed: %s: %w", out, err)
	}
	branch, err := git.QueryInContext(ctx, wt, "branch", "--show-current")
	if err != nil {
		return err
	}
	if branch != plan.currentBranch {
		return fmt.Errorf("migration validation failed: expected branch %s, got %s", plan.currentBranch, branch)
	}
	state, err := readMigrationGitState(ctx, wt)
	if err != nil {
		return err
	}
	if state.index != plan.index || state.stashes != plan.stashes {
		return fmt.Errorf("migration validation failed: index or stash entries changed")
	}
	available := make(map[string]bool)
	for _, ref := range strings.Split(state.refs, "\n") {
		available[ref] = true
	}
	for _, ref := range strings.Split(plan.refs, "\n") {
		if !available[ref] {
			return fmt.Errorf("migration validation failed: missing or changed ref %s", ref)
		}
	}
	if err := fsutil.VerifySnapshot(ctx, wt, []string{".git"}, plan.files); err != nil {
		return err
	}
	return verifyMigrationMetadata(ctx, plan, wt)
}

// Report what is actually left after finalization and rollback. Empty recovery
// directories do not imply that they still contain original repository data.
func migrationRecoveryError(cause error, repoRoot, stage, backup string) error {
	var details []string
	for _, dir := range []string{backup, stage} {
		entries, err := os.ReadDir(dir)
		switch {
		case os.IsNotExist(err):
		case err != nil:
			details = append(details, fmt.Sprintf("could not inspect recovery directory %s: %v; keep it for manual recovery", dir, err))
		case len(entries) > 0:
			details = append(details, "recovery files retained at "+dir)
		case dir == backup:
			if info, err := os.Lstat(filepath.Join(repoRoot, ".git")); err == nil && info.IsDir() {
				details = append(details, "original repository restored at "+repoRoot)
			}
		}
	}
	if len(details) == 0 {
		details = append(details, "inspect repository state at "+repoRoot+" before retrying")
	}
	return fmt.Errorf("%w; %s", cause, strings.Join(details, "; "))
}

// finalizeMigration deliberately retains the original backup even on success.
func finalizeMigration(repoRoot, newStructure, backup string, required []string, moves migrationMoves, verify func() error) error {
	if err := os.MkdirAll(backup, 0o755); err != nil {
		return err
	}
	names, err := moves.move(repoRoot, backup)
	if err != nil {
		return fmt.Errorf("backup original repository: %w", err)
	}
	promoted, err := moves.move(newStructure, repoRoot)
	if err != nil {
		return errors.Join(err, moves.restore(backup, repoRoot, names))
	}
	err = validateMigratedLayout(repoRoot, required)
	if err == nil {
		err = verify()
	}
	if err != nil {
		if rollbackErr := moves.restore(repoRoot, newStructure, promoted); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return errors.Join(err, moves.restore(backup, repoRoot, names))
	}
	return os.Remove(newStructure)
}

func validateMigratedLayout(root string, required []string) error {
	info, err := os.Lstat(filepath.Join(root, ".git"))
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("migration validation failed: .git should be a file")
	}
	for _, path := range required {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			return fmt.Errorf("migration validation failed: missing %s: %w", path, err)
		}
	}
	return nil
}

type migrationMoves struct{ rename func(string, string) error }

func (m migrationMoves) move(src, dst string) ([]string, error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil, err
	}
	var moved []string
	for _, entry := range entries {
		if err := m.rename(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
			return moved, errors.Join(err, m.restore(dst, src, moved))
		}
		moved = append(moved, entry.Name())
	}
	return moved, nil
}

// Never overwrite a recovery target, even when os.Rename would allow it.
func renameEntry(src, dst string) error {
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("refusing to overwrite recovery target %s", dst)
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(src, dst)
}

func (m migrationMoves) restore(src, dst string, names []string) error {
	var errs []error
	for i := len(names) - 1; i >= 0; i-- {
		if err := m.rename(filepath.Join(src, names[i]), filepath.Join(dst, names[i])); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

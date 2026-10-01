package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

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
	refs, index, stashes, status string
}

type migratePlan struct {
	migrationGitState
	repoRoot      string
	currentBranch string
	defaultBranch string
	defaultRemote string
	config        migrationConfigState
}

func runMigrate(cmd *cobra.Command, args []string) error {
	dryRun := boolFlag(cmd, "dry-run") || git.Debug()
	if root, found := findInterruptedMigration(); found {
		return recoverInterruptedMigration(root, dryRun)
	}
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
	fmt.Println("The Git database and working directory will be moved in place, not copied.")
	fmt.Println("Stop other Git operations and file writers before continuing. An interrupted migration is rolled back on the next run.")
	if dryRun {
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
	journal, err := startMigrationJournal(repoRoot)
	if err != nil {
		return err
	}
	// Conversion runs synchronously and checks for signals between steps, so
	// rollback never races a concurrent cleanup.
	if err := convertRepository(ctx, journal, plan); err != nil {
		if rollbackErr := journal.rollback(); rollbackErr != nil {
			return fmt.Errorf("%w; rollback incomplete: %v; stop other writers and run git wt migrate again in %s to finish restoring", err, rollbackErr, repoRoot)
		}
		return fmt.Errorf("%w; original repository restored at %s", err, repoRoot)
	}
	if err := journal.finish(); err != nil {
		ui.Warnf("Migration complete, but could not remove %s: %v", filepath.Join(repoRoot, migrationStateDir), err)
	} else {
		ui.Success("Migration complete")
	}
	branches := []treeBranch{{plan.currentBranch, "current branch"}}
	if plan.defaultBranch != "" && plan.defaultBranch != plan.currentBranch {
		if err := createMigrationWorktree(ctx, repoRoot, plan.defaultBranch, plan.defaultRemote); err != nil {
			ui.Warnf("Could not create a worktree for default branch %s: %v", plan.defaultBranch, err)
			fmt.Printf("Create it later with: cd %s && git wt add %s %s\n", shellQuote(repoRoot), shellQuote(plan.defaultBranch), shellQuote(plan.defaultBranch))
		} else {
			branches = append(branches, treeBranch{plan.defaultBranch, "default branch"})
		}
	}
	fmt.Println(renderRepoLayoutSection(".", branches))
	fmt.Println(renderCommandHintsSection([]commandHint{
		{Action: "Create another worktree", Command: fmt.Sprintf("cd %s && git wt add <branch-name> <branch-name>", shellQuote(repoRoot))},
		{Action: "Open your worktree", Command: "cd " + shellQuote(filepath.Join(repoRoot, plan.currentBranch))},
	}))
	return nil
}

func recoverInterruptedMigration(root string, dryRun bool) error {
	if dryRun {
		return fmt.Errorf("found an interrupted migration in %s; run git wt migrate without --dry-run to restore the original layout", root)
	}
	ui.Info("Restoring the original layout of an interrupted migration in " + root)
	if err := recoverMigration(root); err != nil {
		return fmt.Errorf("restore interrupted migration in %s: %w", root, err)
	}
	return fmt.Errorf("interrupted migration rolled back; original repository restored at %s; run git wt migrate again to migrate", root)
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
	if exists, err := pathExists(filepath.Join(root, migrationStateDir)); err != nil || exists {
		return fmt.Errorf("%s already exists; remove it before migrating", filepath.Join(root, migrationStateDir))
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
	// Rollback removes .bare/worktrees, so it must not hold anything else.
	if exists, err := pathExists(filepath.Join(gitPath, "worktrees")); err != nil || exists {
		return fmt.Errorf("stale worktree metadata in %s; run git worktree prune before migrating", filepath.Join(gitPath, "worktrees"))
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
	if err := checkMigrationHooksPath(root); err != nil {
		return err
	}
	// Refuse active Git writes rather than moving their intermediate state.
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
		return nil
	})
}

// A hooks path inside .git, often .git/hooks to opt out of a global hooksPath,
// stops resolving once .git becomes a file.
func checkMigrationHooksPath(root string) error {
	hooksPath, err := git.QueryIn(root, "config", "--type=path", "--get", "core.hooksPath")
	if err != nil || hooksPath == "" {
		return nil
	}
	resolved := hooksPath
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(root, resolved)
	}
	if !pathWithin(filepath.Join(root, ".git"), resolved) {
		return nil
	}
	return fmt.Errorf("core.hooksPath %q points inside .git, which becomes a file after migration; unset it or point it outside .git, then after migrating set it to %s", hooksPath, filepath.Join(root, ".bare", "hooks"))
}

// convertRepository restructures the repository with renames only, so file
// contents, modes, extended attributes, and ACLs move unchanged. The caller
// rolls back from the journal if any step fails.
func convertRepository(ctx context.Context, journal *migrationJournal, plan migratePlan) error {
	// Git rewrites these in place; everything else is moved or created.
	for _, name := range []string{"config", "packed-refs"} {
		if err := journal.backup(filepath.Join(".git", name)); err != nil {
			return err
		}
	}
	work := filepath.Join(migrationStateDir, "work")
	if err := os.Mkdir(journal.path(work), 0o700); err != nil {
		return err
	}
	// Stage the working entries first: the branch directory may share a name
	// with one of them.
	if err := moveMigrationEntries(ctx, journal, ".", work, ".git", migrationStateDir); err != nil {
		return err
	}
	if err := journal.move(".git", ".bare"); err != nil {
		return err
	}
	if err := journal.create(".git"); err != nil {
		return err
	}
	if err := os.WriteFile(journal.path(".git"), []byte("gitdir: ./.bare\n"), 0o644); err != nil {
		return err
	}
	if err := configureMigratedRepository(ctx, plan); err != nil {
		return err
	}
	private, err := addMigrationWorktree(ctx, journal, plan.currentBranch)
	if err != nil {
		return err
	}
	if err := moveMigrationEntries(ctx, journal, work, filepath.FromSlash(plan.currentBranch)); err != nil {
		return err
	}
	if err := moveMigrationMetadata(ctx, journal, plan, private); err != nil {
		return err
	}
	return verifyMigrationState(ctx, plan)
}

func moveMigrationEntries(ctx context.Context, journal *migrationJournal, src, dst string, skip ...string) error {
	entries, err := os.ReadDir(journal.path(src))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if slices.Contains(skip, entry.Name()) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := journal.move(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func configureMigratedRepository(ctx context.Context, plan migratePlan) error {
	root := plan.repoRoot
	for _, kv := range migrationConfigToSet(plan) {
		if _, err := git.RunInWithOutputContext(ctx, root, "config", kv[0], kv[1]); err != nil {
			return err
		}
	}
	if _, err := git.QueryInContext(ctx, root, "config", "--get", "core.worktree"); err == nil {
		if _, err := git.RunInWithOutputContext(ctx, root, "config", "--unset-all", "core.worktree"); err != nil {
			return err
		}
	}
	return normalizeMigrationRemoteURLs(ctx, plan)
}

// addMigrationWorktree registers an empty worktree for the current branch and
// returns its private Git directory.
func addMigrationWorktree(ctx context.Context, journal *migrationJournal, branch string) (string, error) {
	// Record every directory Git will create, outermost first, so undo removes
	// them innermost first once the moved entries are back in place.
	var dirs []string
	for dir := filepath.FromSlash(branch); dir != "."; dir = filepath.Dir(dir) {
		dirs = append([]string{dir}, dirs...)
	}
	for _, dir := range dirs {
		if exists, err := pathExists(journal.path(dir)); err != nil {
			return "", err
		} else if !exists {
			if err := journal.create(dir); err != nil {
				return "", err
			}
		}
	}
	wt := filepath.FromSlash(branch)
	if err := journal.create(filepath.Join(wt, ".git")); err != nil {
		return "", err
	}
	if err := journal.createTree(filepath.Join(".bare", "worktrees")); err != nil {
		return "", err
	}
	args := []string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "worktree", "add", "--no-checkout", "--", branch, branch}
	if out, err := git.RunInWithOutputContext(ctx, journal.root, args...); err != nil {
		return "", fmt.Errorf("create migration worktree: %s: %w", out, err)
	}
	return git.QueryPathInContext(ctx, journal.path(wt), "rev-parse", "--absolute-git-dir")
}

// createMigrationWorktree adds the default branch's worktree once the
// conversion is complete.
func createMigrationWorktree(ctx context.Context, root, branch, remote string) error {
	args := []string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "worktree", "add"}
	source := branch
	if _, err := git.QueryInContext(ctx, root, "show-ref", "--verify", "refs/heads/"+branch); err != nil {
		source = "refs/remotes/" + remote + "/" + branch
		args = append(args, "-b", branch)
	}
	args = append(args, "--", branch, source)
	out, err := git.RunInWithOutputContext(ctx, root, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}
	return nil
}

func normalizeMigrationRemoteURLs(ctx context.Context, plan migratePlan) error {
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
		if _, err := git.RunInWithOutputContext(ctx, plan.repoRoot, "config", "--unset-all", key); err != nil {
			return err
		}
		for _, url := range urls {
			if _, err := git.RunInWithOutputContext(ctx, plan.repoRoot, "config", "--add", key, url); err != nil {
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
	// Disable fsmonitor callbacks and optional index writes: these checks must
	// not change the repository they verify.
	readOnly := []string{"-c", "core.fsmonitor=false", "--no-optional-locks"}
	state.index, err = git.QueryRawInContext(ctx, root, append(readOnly, "ls-files", "--stage", "-z")...)
	if err != nil {
		return state, err
	}
	state.status, err = git.QueryRawInContext(ctx, root, append(readOnly, "status", "--porcelain=v2", "-z", "--ignored", "--untracked-files=normal")...)
	if err != nil {
		return state, err
	}
	state.stashes, err = git.QueryInContext(ctx, root, "stash", "list", "--format=%H %gs")
	return state, err
}

func verifyMigrationState(ctx context.Context, plan migratePlan) error {
	wt := filepath.Join(plan.repoRoot, plan.currentBranch)
	for _, root := range []string{plan.repoRoot, wt} {
		if err := verifyMigrationConfig(ctx, plan, root); err != nil {
			return err
		}
	}
	if err := verifyMigrationPrivateRefIsolation(ctx, plan.repoRoot); err != nil {
		return err
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
	for _, check := range []struct{ name, before, after string }{
		{"refs", plan.refs, state.refs},
		{"index entries", plan.index, state.index},
		{"working tree status", plan.status, state.status},
		{"stash entries", plan.stashes, state.stashes},
	} {
		if check.before != check.after {
			return fmt.Errorf("migration validation failed: %s changed", check.name)
		}
	}
	return nil
}

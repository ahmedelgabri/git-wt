package cmd

import (
	"bufio"
	"fmt"
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
	Use:           "migrate",
	Short:         "Migrate an existing repository to use worktrees [EXPERIMENTAL]",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runMigrate,
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

func runMigrate(cmd *cobra.Command, args []string) error {
	// Check if we're in a git repository
	if _, err := git.Query("rev-parse", "--git-dir"); err != nil {
		ui.Error("Not in a git repository")
		return fmt.Errorf("not in a git repository")
	}

	repoRoot, err := git.Query("rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	// Resolve symlinks (macOS /tmp -> /private/tmp)
	repoRoot, err = filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return err
	}

	repoName := filepath.Base(repoRoot)
	parentDir := filepath.Dir(repoRoot)

	currentBranch, err := git.Query("branch", "--show-current")
	if err != nil || currentBranch == "" {
		ui.Error("Not on a branch (detached HEAD state). Please check out a branch first.")
		return fmt.Errorf("detached HEAD state")
	}

	// Get remote URL
	remoteURL, _ := git.Query("remote", "get-url", "origin")
	if remoteURL != "" {
		ui.Infof("Remote URL: %s", remoteURL)
	} else {
		ui.Warn("No remote 'origin' found")
	}

	// Discover default branch
	defaultBranch := ""
	if remoteURL != "" {
		defaultBranch = worktree.DefaultBranch()
	}
	if defaultBranch == "" {
		for _, branch := range []string{"main", "master"} {
			if _, err := git.Query("rev-parse", "--verify", branch); err == nil {
				defaultBranch = branch
				break
			}
		}
	}

	ui.Infof("Migrating repository: %s", repoName)
	ui.Infof("Current branch: %s", currentBranch)
	if defaultBranch != "" {
		ui.Infof("Default branch: %s", defaultBranch)
	}
	fmt.Println()

	// Detect uncommitted changes
	hasChanges := false
	if err := checkGitDiff(repoRoot); err != nil {
		hasChanges = true
		ui.Info("Detected uncommitted changes - will preserve them in the new worktree")
	}

	// Detect untracked files
	untrackedFiles, _ := git.QueryLines("ls-files", "--others", "--exclude-standard")
	if len(untrackedFiles) > 0 {
		ui.Infof("Detected %d untracked file(s) - will preserve them", len(untrackedFiles))
	}

	// Store stash count
	stashList, _ := git.QueryLines("stash", "list")
	stashCount := len(stashList)
	if stashCount > 0 {
		ui.Infof("Detected %d stash(es) - will migrate them", stashCount)
	}

	fmt.Println()

	// Confirm
	fmt.Printf("%s ", ui.Yellow("This will restructure the repository. Continue? [y/N]:"))
	reader := bufio.NewReader(os.Stdin)
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(confirm)
	if confirm != "y" {
		fmt.Println("Migration cancelled.")
		return nil
	}

	newStructure := filepath.Join(parentDir, fmt.Sprintf("%s-new-%d", repoName, os.Getpid()))
	tempBackup := filepath.Join(parentDir, fmt.Sprintf("%s-backup-%d", repoName, os.Getpid()))

	// Setup cleanup on signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	cleanupDone := make(chan struct{})

	cleanup := func() {
		if _, err := os.Stat(newStructure); err == nil {
			os.RemoveAll(newStructure)
		}
		if _, err := os.Stat(tempBackup); err == nil {
			restoreBackup(tempBackup, repoRoot)
		}
	}

	go func() {
		<-sigCh
		cleanup()
		close(cleanupDone)
		os.Exit(1)
	}()

	// Ensure cleanup on any error
	success := false
	defer func() {
		signal.Stop(sigCh)
		if !success {
			cleanup()
		}
	}()

	ui.Info("Creating new repository structure...")
	if err := os.MkdirAll(newStructure, 0o755); err != nil {
		return err
	}

	// Clone existing repo as bare into .bare
	ui.Info("Converting to bare repository...")
	if err := git.Run("clone", "--bare", repoRoot, filepath.Join(newStructure, ".bare")); err != nil {
		return err
	}

	// Create .git file
	if err := os.WriteFile(filepath.Join(newStructure, ".git"), []byte("gitdir: ./.bare\n"), 0o644); err != nil {
		return err
	}

	// Configure bare repo
	ui.Info("Configuring bare repository...")
	gitArgs := func(args ...string) error {
		return git.RunIn(newStructure, args...)
	}

	if err := gitArgs("config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
		return err
	}
	if err := gitArgs("config", "core.logallrefupdates", "true"); err != nil {
		return err
	}

	// Fetch from remote if available
	if remoteURL, _ := git.QueryIn(newStructure, "remote", "get-url", "origin"); remoteURL != "" {
		ui.Info("Fetching all branches from remote...")
		if err := gitArgs("fetch", "--all"); err != nil {
			ui.Warn("Could not fetch from remote (remote may be unreachable) - continuing with local data")
		}
	}

	// Clean up invalid local branch refs
	refs, _ := git.QueryIn(newStructure, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if refs != "" {
		for _, ref := range strings.Split(refs, "\n") {
			ref = strings.TrimSpace(ref)
			if ref != "" {
				gitArgs("branch", "-D", ref)
			}
		}
	}

	// Restore remote URL (clone set it to the local path)
	if remoteURL != "" {
		ui.Info("Restoring remote URL...")
		if err := gitArgs("remote", "set-url", "origin", remoteURL); err != nil {
			return err
		}
	}

	// Migrate stashes
	if stashCount > 0 {
		ui.Infof("Migrating %d stash(es)...", stashCount)
		oldGitDir := filepath.Join(repoRoot, ".git")
		newBareDir := filepath.Join(newStructure, ".bare")

		stashRef := filepath.Join(oldGitDir, "refs", "stash")
		if _, err := os.Stat(stashRef); err == nil {
			copyFileSimple(stashRef, filepath.Join(newBareDir, "refs", "stash"))
		}

		stashLog := filepath.Join(oldGitDir, "logs", "refs", "stash")
		if _, err := os.Stat(stashLog); err == nil {
			os.MkdirAll(filepath.Join(newBareDir, "logs", "refs"), 0o755)
			copyFileSimple(stashLog, filepath.Join(newBareDir, "logs", "refs", "stash"))
		}
	}

	// Create worktrees
	if defaultBranch != "" && defaultBranch == currentBranch {
		ui.Infof("Creating worktree for %s (default branch)...", currentBranch)
		git.RunIn(newStructure, "worktree", "add", currentBranch, currentBranch)
	} else {
		if defaultBranch != "" {
			ui.Infof("Creating worktree for default branch: %s", defaultBranch)
			git.RunIn(newStructure, "worktree", "add", defaultBranch, defaultBranch)
		}
		ui.Infof("Creating worktree for current branch: %s", currentBranch)
		git.RunIn(newStructure, "worktree", "add", currentBranch, currentBranch)
	}

	// Restore working directory state
	ui.Info("Restoring working directory state...")
	destDir := filepath.Join(newStructure, currentBranch)
	if err := fsutil.CopyDir(repoRoot, destDir, []string{".git"}); err != nil {
		return fmt.Errorf("failed to copy working directory: %w", err)
	}

	// Restore git index (staged changes)
	oldIndex := filepath.Join(repoRoot, ".git", "index")
	if _, err := os.Stat(oldIndex); err == nil {
		newIndex := filepath.Join(newStructure, ".bare", "worktrees", currentBranch, "index")
		copyFileSimple(oldIndex, newIndex)
	}

	ui.Success("Working directory state restored (all files preserved)")

	// Atomic swap: move contents to preserve REPO_ROOT's inode
	fmt.Println()
	ui.Info("Finalizing migration...")

	if err := os.Chdir(parentDir); err != nil {
		return err
	}

	if err := os.MkdirAll(tempBackup, 0o755); err != nil {
		return err
	}

	// Move original contents to backup
	if err := moveContents(repoRoot, tempBackup); err != nil {
		return fmt.Errorf("failed to backup original repo: %w", err)
	}

	// Move new structure contents to repo root
	if err := moveContents(newStructure, repoRoot); err != nil {
		return fmt.Errorf("failed to move new structure: %w", err)
	}

	os.Remove(newStructure) // remove empty dir

	ui.Info("Cleaning up...")
	os.RemoveAll(tempBackup)

	fmt.Println()
	ui.Success("Migration complete!")
	fmt.Printf("\n  Your repository structure is now:\n")
	fmt.Printf("    %s/\n", repoRoot)
	fmt.Printf("    ├── .bare/              (git data)\n")
	fmt.Printf("    ├── .git                (pointer to .bare)\n")

	if defaultBranch != "" && defaultBranch == currentBranch {
		fmt.Printf("    └── %s/           (worktree - default branch)\n", currentBranch)
	} else {
		if defaultBranch != "" {
			fmt.Printf("    ├── %s/           (worktree - default branch)\n", defaultBranch)
		}
		fmt.Printf("    └── %s/           (worktree - current branch)\n", currentBranch)
	}
	fmt.Println()

	if remoteURL != "" {
		ui.Successf("Remote URL preserved: %s", remoteURL)
	}
	if stashCount > 0 {
		ui.Successf("Migrated %d stash(es)", stashCount)
	}
	if hasChanges {
		ui.Successf("Preserved uncommitted changes in %s/", currentBranch)
	}
	if len(untrackedFiles) > 0 {
		ui.Successf("Preserved %d untracked file(s) in %s/", len(untrackedFiles), currentBranch)
	}

	fmt.Printf("\n  To create additional worktrees:\n")
	fmt.Printf("    cd %s\n", repoRoot)
	fmt.Printf("    git wt add <branch-name> <branch-name>\n")
	fmt.Printf("\n  To view migrated stashes:\n")
	fmt.Printf("    cd %s/%s\n", repoRoot, currentBranch)
	fmt.Printf("    git stash list\n")
	fmt.Printf("\n  Navigate to your worktree:\n")
	fmt.Printf("    cd %s/%s\n", repoRoot, currentBranch)

	success = true
	return nil
}

func checkGitDiff(repoRoot string) error {
	cmd := fmt.Sprintf("diff-index --quiet HEAD --")
	_ = cmd
	_, err := git.QueryIn(repoRoot, "diff-index", "--quiet", "HEAD", "--")
	return err
}

func copyFileSimple(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func moveContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		oldPath := filepath.Join(src, entry.Name())
		newPath := filepath.Join(dst, entry.Name())
		if err := os.Rename(oldPath, newPath); err != nil {
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
		os.Rename(filepath.Join(backup, entry.Name()), filepath.Join(repoRoot, entry.Name()))
	}
	os.RemoveAll(backup)
}

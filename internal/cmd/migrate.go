package cmd

import (
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

	remote := worktree.DefaultRemote()
	remoteURL := ""
	if remote != "" {
		remoteURL, _ = git.Query("remote", "get-url", remote)
	}
	if remoteURL != "" {
		fmt.Printf("Remote URL:     %s\n", ui.Accent(remoteURL))
	} else {
		ui.Warn("No remote found")
	}

	// Discover default branch via symbolic-ref (local) or remote query
	defaultBranch := worktree.DefaultBranch(remote)

	fmt.Printf("Repository:      %s\n", ui.Bold(repoName))
	fmt.Printf("Current branch:  %s\n", ui.Accent(currentBranch))
	if defaultBranch != "" {
		fmt.Printf("Default branch:  %s\n", ui.Accent(defaultBranch))
	}
	fmt.Println()

	// Detect uncommitted changes
	hasChanges := false
	if err := checkGitDiff(repoRoot); err != nil {
		hasChanges = true
		fmt.Printf("%s uncommitted changes %s\n", ui.Yellow("!"), ui.Dim("(will preserve)"))
	}

	// Detect untracked files
	untrackedFiles, _ := git.QueryLines("ls-files", "--others", "--exclude-standard")
	if len(untrackedFiles) > 0 {
		fmt.Printf("%s %d untracked file(s) %s\n", ui.Yellow("!"), len(untrackedFiles), ui.Dim("(will preserve)"))
	}

	// Store stash count
	stashList, _ := git.QueryLines("stash", "list")
	stashCount := len(stashList)
	if stashCount > 0 {
		fmt.Printf("%s %d stash(es) %s\n", ui.Yellow("!"), stashCount, ui.Dim("(will migrate)"))
	}

	fmt.Println()

	// Confirm
	if !ui.Confirm("This will restructure the repository. Continue? [y/N]:") {
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

	if err := os.MkdirAll(newStructure, 0o755); err != nil {
		return err
	}

	// Clone existing repo as bare into .bare
	if err := ui.Spin("Converting to bare repository", func() error {
		_, err := git.RunWithOutput("clone", "--bare", repoRoot, filepath.Join(newStructure, ".bare"))
		return err
	}); err != nil {
		return err
	}

	// Create .git file
	if err := os.WriteFile(filepath.Join(newStructure, ".git"), []byte("gitdir: ./.bare\n"), 0o644); err != nil {
		return err
	}

	// Configure bare repo
	if _, err := git.RunInWithOutput(newStructure, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
		return err
	}
	if _, err := git.RunInWithOutput(newStructure, "config", "core.logallrefupdates", "true"); err != nil {
		return err
	}

	// Fetch from remote if available
	if remoteURL, _ := git.QueryIn(newStructure, "remote", "get-url", "origin"); remoteURL != "" {
		if err := ui.Spin("Fetching all branches from remote", func() error {
			_, err := git.RunInWithOutput(newStructure, "fetch", "--all")
			return err
		}); err != nil {
			ui.Warn("Could not fetch from remote (remote may be unreachable) - continuing with local data")
		}
	}

	// Clean up invalid local branch refs
	refs, _ := git.QueryIn(newStructure, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if refs != "" {
		for ref := range strings.SplitSeq(refs, "\n") {
			ref = strings.TrimSpace(ref)
			if ref != "" {
				git.RunInWithOutput(newStructure, "branch", "-D", ref)
			}
		}
	}

	// Restore remote URL (clone set it to the local path)
	if remoteURL != "" {
		if _, err := git.RunInWithOutput(newStructure, "remote", "set-url", "origin", remoteURL); err != nil {
			return err
		}
	}

	// Migrate stashes
	if stashCount > 0 {
		ui.Spin(fmt.Sprintf("Migrating %d stash(es)", stashCount), func() error {
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
			return nil
		})
	}

	// Create worktrees
	if defaultBranch != "" && defaultBranch == currentBranch {
		if err := ui.Spin(fmt.Sprintf("Creating worktree for %s", ui.Accent(currentBranch)), func() error {
			_, err := git.RunInWithOutput(newStructure, "worktree", "add", currentBranch, currentBranch)
			return err
		}); err != nil {
			return err
		}
	} else {
		if defaultBranch != "" {
			if err := ui.Spin(fmt.Sprintf("Creating worktree for %s", ui.Accent(defaultBranch)), func() error {
				_, err := git.RunInWithOutput(newStructure, "worktree", "add", defaultBranch, defaultBranch)
				return err
			}); err != nil {
				return err
			}
		}
		if err := ui.Spin(fmt.Sprintf("Creating worktree for %s", ui.Accent(currentBranch)), func() error {
			_, err := git.RunInWithOutput(newStructure, "worktree", "add", currentBranch, currentBranch)
			return err
		}); err != nil {
			return err
		}
	}

	// Restore working directory state
	destDir := filepath.Join(newStructure, currentBranch)
	if err := ui.Spin("Restoring working directory state", func() error {
		if err := fsutil.CopyDir(repoRoot, destDir, []string{".git"}); err != nil {
			return fmt.Errorf("failed to copy working directory: %w", err)
		}
		// Restore git index (staged changes)
		oldIndex := filepath.Join(repoRoot, ".git", "index")
		if _, err := os.Stat(oldIndex); err == nil {
			newIndex := filepath.Join(newStructure, ".bare", "worktrees", currentBranch, "index")
			copyFileSimple(oldIndex, newIndex)
		}
		return nil
	}); err != nil {
		return err
	}

	// Atomic swap: move contents to preserve REPO_ROOT's inode
	fmt.Println()
	if err := ui.Spin("Finalizing migration", func() error {
		if err := os.Chdir(parentDir); err != nil {
			return err
		}
		if err := os.MkdirAll(tempBackup, 0o755); err != nil {
			return err
		}
		if err := moveContents(repoRoot, tempBackup); err != nil {
			return fmt.Errorf("failed to backup original repo: %w", err)
		}
		if err := moveContents(newStructure, repoRoot); err != nil {
			return fmt.Errorf("failed to move new structure: %w", err)
		}
		os.Remove(newStructure)
		os.RemoveAll(tempBackup)
		return nil
	}); err != nil {
		return err
	}

	fmt.Println()
	ui.Success("Migration complete")
	fmt.Printf("\n  Repository structure:\n")
	// Compute padding for aligned descriptions
	treeWidth := len(".bare/")
	for _, name := range []string{currentBranch + "/", defaultBranch + "/"} {
		if len(name) > treeWidth {
			treeWidth = len(name)
		}
	}

	fmt.Printf("    %s/\n", ui.Bold(repoRoot))
	fmt.Printf("    ├── %s  %s\n", ui.Muted(fmt.Sprintf("%-*s", treeWidth, ".bare/")), ui.Dim("(git data)"))
	fmt.Printf("    ├── %s  %s\n", ui.Muted(fmt.Sprintf("%-*s", treeWidth, ".git")), ui.Dim("(pointer to .bare)"))

	if defaultBranch != "" && defaultBranch == currentBranch {
		fmt.Printf("    └── %s  %s\n", ui.Accent(fmt.Sprintf("%-*s", treeWidth, currentBranch+"/")), ui.Dim("(worktree)"))
	} else {
		if defaultBranch != "" {
			fmt.Printf("    ├── %s  %s\n", ui.Accent(fmt.Sprintf("%-*s", treeWidth, defaultBranch+"/")), ui.Dim("(default branch)"))
		}
		fmt.Printf("    └── %s  %s\n", ui.Accent(fmt.Sprintf("%-*s", treeWidth, currentBranch+"/")), ui.Dim("(current branch)"))
	}
	fmt.Println()

	if remoteURL != "" {
		ui.Successf("Remote URL preserved: %s", ui.Accent(remoteURL))
	}
	if stashCount > 0 {
		ui.Successf("Migrated %d stash(es)", stashCount)
	}
	if hasChanges {
		ui.Successf("Preserved uncommitted changes in %s/", ui.Accent(currentBranch))
	}
	if len(untrackedFiles) > 0 {
		ui.Successf("Preserved %d untracked file(s) in %s/", len(untrackedFiles), ui.Accent(currentBranch))
	}

	fmt.Printf("\n  To create additional worktrees:\n")
	fmt.Printf("    %s\n", ui.Muted("cd "+repoRoot))
	fmt.Printf("    %s\n", ui.Muted("git wt add <branch-name> <branch-name>"))
	fmt.Printf("\n  To view migrated stashes:\n")
	fmt.Printf("    %s\n", ui.Muted(fmt.Sprintf("cd %s/%s", repoRoot, currentBranch)))
	fmt.Printf("    %s\n", ui.Muted("git stash list"))
	fmt.Printf("\n  Navigate to your worktree:\n")
	fmt.Printf("    %s\n", ui.Muted(fmt.Sprintf("cd %s/%s", repoRoot, currentBranch)))

	success = true
	return nil
}

func checkGitDiff(repoRoot string) error {
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

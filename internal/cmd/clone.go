package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/ui"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
	"github.com/spf13/cobra"
)

var cloneCmd = &cobra.Command{
	Use:   "clone <repository-url> [folder-name]",
	Short: "Clone a repository with worktree structure",
	Long: `git wt clone - Clone a repository with worktree structure

Usage:
  git wt clone <repository-url> [folder-name]

Options:
  --help, -h      Show this help message

Examples:
  git wt clone https://github.com/user/repo.git
  git wt clone git@github.com:user/repo.git my-repo

Note: Creates .bare directory structure and initial worktree for default branch`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.RangeArgs(1, 2),
	RunE:          runClone,
}

func init() {
	rootCmd.AddCommand(cloneCmd)
}

func runClone(cmd *cobra.Command, args []string) error {
	repoURL := args[0]
	folderName := strings.TrimSuffix(filepath.Base(repoURL), ".git")
	if len(args) > 1 {
		folderName = args[1]
	}

	// Check if directory already exists
	if info, err := os.Stat(folderName); err == nil && info.IsDir() {
		ui.Errorf("Directory '%s' already exists", folderName)
		return fmt.Errorf("directory '%s' already exists", folderName)
	}

	if err := os.MkdirAll(folderName, 0o755); err != nil {
		ui.Errorf("Failed to create directory '%s'", folderName)
		return err
	}

	if err := os.Chdir(folderName); err != nil {
		ui.Errorf("Failed to change to directory '%s'", folderName)
		return err
	}

	// Clone with cleanup on failure
	if err := git.Run("clone", "--bare", repoURL, ".bare"); err != nil {
		ui.Error("Failed to clone repository")
		os.Chdir("..")
		os.RemoveAll(folderName)
		return err
	}

	// Create .git file pointing to .bare
	if err := os.WriteFile(".git", []byte("gitdir: ./.bare\n"), 0o644); err != nil {
		return err
	}

	// Configure the bare repo
	if err := git.Run("config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
		return err
	}
	if err := git.Run("config", "core.logallrefupdates", "true"); err != nil {
		return err
	}
	if err := git.Run("config", "worktree.useRelativePaths", "true"); err != nil {
		return err
	}

	if err := git.Run("fetch", "--all"); err != nil {
		ui.Warn("Failed to fetch all branches")
	}

	// Clean up invalid local branch refs
	refs, _ := git.Query("for-each-ref", "--format=%(refname:short)", "refs/heads")
	if refs != "" {
		for _, ref := range strings.Split(refs, "\n") {
			ref = strings.TrimSpace(ref)
			if ref != "" {
				git.Run("branch", "-D", ref)
			}
		}
	}

	ui.Info("Discovering default branch...")
	defaultBranch := worktree.DefaultBranch()

	if defaultBranch == "" {
		ui.Warn("Could not discover default branch from remote")
		fmt.Println("Available branches:")
		git.Run("branch", "-r")
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("%s ", ui.Bold("Enter default branch name (or press Enter to skip):"))
		input, _ := reader.ReadString('\n')
		defaultBranch = strings.TrimSpace(input)
	}

	if defaultBranch != "" {
		ui.Infof("Creating initial worktree for '%s'...", defaultBranch)
		if err := git.Run("worktree", "add", "-B", defaultBranch, defaultBranch, "origin/"+defaultBranch); err != nil {
			ui.Warn("Failed to create worktree for default branch")
		}
	} else {
		fmt.Println("No worktree created. Use 'git wt add' to create worktrees.")
	}

	fmt.Println()
	ui.Success("Repository cloned successfully")
	fmt.Printf("\n  Repository structure:\n")
	fmt.Printf("    %s/\n", folderName)
	fmt.Printf("    ├── .bare/              (git data)\n")
	fmt.Printf("    ├── .git                (pointer to .bare)\n")
	if defaultBranch != "" {
		fmt.Printf("    └── %s/           (worktree)\n", defaultBranch)
	}
	fmt.Printf("\n  To create additional worktrees:\n")
	fmt.Printf("    cd %s\n", folderName)
	fmt.Printf("    git wt add\n")

	return nil
}

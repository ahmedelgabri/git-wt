package cmd

import (
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
	Long: `Clone a repository and set up the bare worktree structure. Creates a .bare
directory for git data and an initial worktree for the default branch.`,
	Example: `  git wt clone https://github.com/user/repo.git
  git wt clone git@github.com:user/repo.git my-repo`,
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
	if err := ui.Spin("Cloning repository", func() error {
		_, err := git.RunWithOutput("clone", "--bare", repoURL, ".bare")
		return err
	}); err != nil {
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
	if _, err := git.RunWithOutput("config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
		return err
	}
	if _, err := git.RunWithOutput("config", "core.logallrefupdates", "true"); err != nil {
		return err
	}
	if _, err := git.RunWithOutput("config", "worktree.useRelativePaths", "true"); err != nil {
		return err
	}

	if err := ui.Spin("Fetching all branches", func() error {
		_, err := git.RunWithOutput("fetch", "--all")
		return err
	}); err != nil {
		ui.Warn("Failed to fetch all branches")
	}

	// Clean up invalid local branch refs
	refs, _ := git.Query("for-each-ref", "--format=%(refname:short)", "refs/heads")
	if refs != "" {
		for _, ref := range strings.Split(refs, "\n") {
			ref = strings.TrimSpace(ref)
			if ref != "" {
				git.RunWithOutput("branch", "-D", ref)
			}
		}
	}

	var defaultBranch string
	ui.Spin("Discovering default branch", func() error {
		defaultBranch = worktree.DefaultBranch()
		if defaultBranch == "" {
			return fmt.Errorf("not found")
		}
		return nil
	})

	if defaultBranch == "" {
		ui.Warn("Could not discover default branch from remote")
		fmt.Println("Available branches:")
		git.Run("branch", "-r")
		defaultBranch = ui.PromptInput("Enter default branch name (or press Enter to skip):")
	}

	if defaultBranch != "" {
		if err := ui.Spin(fmt.Sprintf("Creating worktree for %s", ui.Accent(defaultBranch)), func() error {
			_, err := git.RunWithOutput("worktree", "add", "-B", defaultBranch, defaultBranch, "origin/"+defaultBranch)
			return err
		}); err != nil {
			ui.Warn("Failed to create worktree for default branch")
		}
	} else {
		fmt.Printf("No worktree created. Use %s to create worktrees.\n", ui.Accent("git wt add"))
	}

	fmt.Println()
	ui.Success("Repository cloned successfully")
	fmt.Printf("\n  Repository structure:\n")
	fmt.Printf("    %s/\n", ui.Bold(folderName))
	fmt.Printf("    ├── %s              %s\n", ui.Muted(".bare/"), ui.Dim("(git data)"))
	fmt.Printf("    ├── %s                %s\n", ui.Muted(".git"), ui.Dim("(pointer to .bare)"))
	if defaultBranch != "" {
		fmt.Printf("    └── %s/           %s\n", ui.Accent(defaultBranch), ui.Dim("(worktree)"))
	}
	fmt.Printf("\n  To create additional worktrees:\n")
	fmt.Printf("    %s\n", ui.Muted("cd "+folderName))
	fmt.Printf("    %s\n", ui.Muted("git wt add"))

	return nil
}

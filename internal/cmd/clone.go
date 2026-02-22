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
	if err := configureBareRepo("."); err != nil {
		return err
	}

	if err := ui.Spin("Fetching all branches", func() error {
		_, err := git.RunWithOutput("fetch", "--all")
		return err
	}); err != nil {
		ui.Warn("Failed to fetch all branches")
	}

	cleanupLocalBranchRefs(".")

	var defaultBranch string
	// Error ignored: the fallback prompt below handles the empty-branch case
	_ = ui.Spin("Discovering default branch", func() error {
		defaultBranch = worktree.DefaultBranch("origin")
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

	ui.Success("\nRepository cloned successfully")

	var branches []treeBranch
	if defaultBranch != "" {
		branches = append(branches, treeBranch{defaultBranch, "worktree"})
	}
	printRepoTree(folderName, branches)

	fmt.Printf("\n  To create additional worktrees:\n")
	fmt.Printf("    %s\n", ui.Muted("cd "+folderName))
	fmt.Printf("    %s\n", ui.Muted("git wt add"))

	return nil
}

type treeBranch struct {
	Name string
	Desc string
}

func printRepoTree(rootDir string, branches []treeBranch) {
	// Compute padding for aligned descriptions
	treeWidth := len(".bare/")
	for _, b := range branches {
		if w := len(b.Name) + 1; w > treeWidth {
			treeWidth = w
		}
	}

	fmt.Printf("\n  Repository structure:\n")
	fmt.Printf("    %s/\n", ui.Bold(rootDir))
	fmt.Printf("    ├── %s  %s\n", ui.Muted(fmt.Sprintf("%-*s", treeWidth, ".bare/")), ui.Dim("(git data)"))

	if len(branches) == 0 {
		fmt.Printf("    └── %s  %s\n", ui.Muted(fmt.Sprintf("%-*s", treeWidth, ".git")), ui.Dim("(pointer to .bare)"))
	} else {
		fmt.Printf("    ├── %s  %s\n", ui.Muted(fmt.Sprintf("%-*s", treeWidth, ".git")), ui.Dim("(pointer to .bare)"))
		for i, b := range branches {
			connector := "├──"
			if i == len(branches)-1 {
				connector = "└──"
			}
			fmt.Printf("    %s %s  %s\n", connector, ui.Accent(fmt.Sprintf("%-*s", treeWidth, b.Name+"/")), ui.Dim("("+b.Desc+")"))
		}
	}
	fmt.Println()
}

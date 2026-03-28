package cmd

import (
	"context"
	"fmt"
	"io"
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

	if _, err := os.Stat(folderName); err == nil {
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
	if err := ui.SpinWithOutputContext("Cloning repository", func(ctx context.Context, w io.Writer) error {
		return git.RunToContext(ctx, w, "clone", "--bare", repoURL, ".bare")
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

	if err := ui.SpinWithOutputContext("Fetching all branches", func(ctx context.Context, w io.Writer) error {
		return git.RunToContext(ctx, w, "fetch", "--all")
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
		if err := ui.SpinWithOutputContext(fmt.Sprintf("Creating worktree for %s", ui.Accent(defaultBranch)), func(ctx context.Context, w io.Writer) error {
			return git.RunToContext(ctx, w, "worktree", "add", "-B", defaultBranch, defaultBranch, "origin/"+defaultBranch)
		}); err != nil {
			ui.Warn("Failed to create worktree for default branch")
		}
	} else {
		fmt.Printf("No worktree created. Use %s to create worktrees.\n", ui.Accent("git wt add"))
	}

	ui.Success("Repository cloned successfully")

	var branches []treeBranch
	if defaultBranch != "" {
		branches = append(branches, treeBranch{defaultBranch, "default worktree"})
	}

	fmt.Println()
	fmt.Println(renderRepoLayoutSection(folderName, branches))
	fmt.Println()
	fmt.Println(renderCommandHintsSection([]commandHint{{
		Action:  "Create another worktree",
		Command: fmt.Sprintf("cd %s && git wt add", folderName),
	}}))

	return nil
}

type treeBranch struct {
	Name string
	Desc string
}

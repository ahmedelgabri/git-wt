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

func runClone(cmd *cobra.Command, args []string) (resultErr error) {
	repoURL := args[0]
	folderName := strings.TrimSuffix(filepath.Base(repoURL), ".git")
	if len(args) > 1 {
		folderName = args[1]
	}

	destination, err := filepath.Abs(folderName)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(destination); err == nil {
		ui.Errorf("Directory '%s' already exists", folderName)
		return fmt.Errorf("directory '%s' already exists", folderName)
	}

	if git.Debug() {
		return git.Run("clone", "--bare", repoURL, filepath.Join(destination, ".bare"))
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(destination, 0o755); err != nil {
		ui.Errorf("Failed to create directory '%s'", folderName)
		return err
	}

	cloned := false
	defer func() {
		if !cloned {
			if err := os.RemoveAll(destination); err != nil {
				ui.Errorf("Could not clean up %s: %v", destination, err)
			}
		} else if resultErr != nil {
			fmt.Fprintf(os.Stderr, "%s Repository downloaded and retained at %s, but setup did not finish: %v\n", ui.Yellow("Warning:"), destination, resultErr)
			fmt.Fprintf(os.Stderr, "Inspect the downloaded branches with: git --git-dir=%s branch -a\n", shellQuote(filepath.Join(destination, ".bare")))
			fmt.Fprintf(os.Stderr, "Finish layout configuration if needed, then create a worktree with: git -C %s wt add <path> <branch>\n", shellQuote(destination))
		}
	}()

	var defaultBranch string
	if err := ui.RunSteps([]ui.Step{{
		Message:    "Cloning repository",
		ShowOutput: true,
		RawOutput:  true,
		Run: func(ctx context.Context, w io.Writer) error {
			if err := git.RunToContext(ctx, w, "clone", "--progress", "--bare", "--", repoURL, filepath.Join(destination, ".bare")); err != nil {
				return err
			}
			cloned = true
			return nil
		},
	}, {
		Message: "Configuring worktree layout",
		Run: func(context.Context, io.Writer) error {
			if err := os.WriteFile(filepath.Join(destination, ".git"), []byte("gitdir: ./.bare\n"), 0o644); err != nil {
				return err
			}
			return configureBareRepo(destination)
		},
	}, {
		Message:    "Fetching all branches",
		ShowOutput: true,
		RawOutput:  true,
		Run: func(ctx context.Context, w io.Writer) error {
			return git.RunInToContext(ctx, destination, w, "fetch", "--progress", "--all")
		},
	}, {
		Message: "Discovering default branch",
		Run: func(context.Context, io.Writer) error {
			cleanupLocalBranchRefs(destination)
			defaultBranch = worktree.DefaultBranchIn(destination, "origin")
			return nil
		},
	}}); err != nil {
		if !cloned {
			ui.Error("Failed to clone repository")
		}
		return err
	}

	if defaultBranch == "" {
		ui.Warn("Could not discover default branch from remote")
		fmt.Println("Available branches:")
		_ = git.RunIn(destination, "branch", "-r")
		defaultBranch, err = ui.PromptInputResult("Enter default branch name (or press Enter to skip):")
		if err != nil {
			return err
		}
	}

	if defaultBranch != "" {
		if err := ui.RunSteps([]ui.Step{{
			Message:    fmt.Sprintf("Creating worktree for %s", ui.Accent(defaultBranch)),
			ShowOutput: true,
			Run: func(ctx context.Context, w io.Writer) error {
				return git.RunInToContext(ctx, destination, w, "worktree", "add", "-B", defaultBranch, defaultBranch, "origin/"+defaultBranch)
			},
		}}); err != nil {
			return err
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

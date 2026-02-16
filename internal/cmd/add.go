package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/picker"
	"github.com/ahmedelgabri/git-wt/internal/ui"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [options] [<path>] [<commit-ish>]",
	Short: "Create a new worktree",
	Long: `Create a new worktree. With no arguments, opens an interactive picker to
select from remote branches or create a new branch. All git worktree add
flags are supported (-b, -B, -d, --lock, --quiet, etc).

Always fetches from origin before creating the worktree. When using -b/-B,
upstream tracking is set automatically if the branch exists on origin.`,
	Example: `  git wt add                               # Interactive selection
  git wt add feature origin/feature        # From remote branch
  git wt add -b new-feature new-feature    # New branch
  git wt add --detach hotfix HEAD~5        # Detached HEAD worktree`,
	FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	SilenceUsage:       true,
	SilenceErrors:      true,
	RunE:               runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	// Change to bare root
	root, err := worktree.BareRoot()
	if err != nil {
		return err
	}
	if err := os.Chdir(root); err != nil {
		return fmt.Errorf("failed to change to bare root: %w", err)
	}

	// Fetch latest remote branches
	ui.Info("Fetching from origin...")
	if err := git.Run("fetch", "origin", "--prune"); err != nil {
		return err
	}

	// If no arguments, run interactive mode
	if len(args) == 0 {
		return runAddInteractive()
	}

	// Non-interactive: parse flags and pass through to git worktree add
	return runAddDirect(args)
}

func runAddInteractive() error {
	// Get remote branches (excluding HEAD)
	lines, err := git.QueryLines("branch", "-r", "--format=%(refname:short)")
	if err != nil {
		return fmt.Errorf("failed to list remote branches: %w", err)
	}

	var items []picker.Item
	// Add "Create new branch" option first
	items = append(items, picker.Item{
		Label: "Create new branch",
		Value: "__create_new__",
	})

	for _, line := range lines {
		if strings.Contains(line, "HEAD") {
			continue
		}
		branch := strings.TrimPrefix(line, "origin/")
		items = append(items, picker.Item{
			Label: branch,
			Value: branch,
		})
	}

	result, err := picker.Run(picker.Config{
		Items:  items,
		Prompt: "Select branch or create new: ",
		PreviewFunc: func(item picker.Item) string {
			if item.Value == "__create_new__" {
				return "Create a new branch and worktree\n\nYou will be prompted to enter:\n  - Branch name\n  - Worktree path (optional, defaults to branch name)"
			}
			out, _ := git.Query("log", "--oneline", "--graph", "--date=short",
				"--color=always", "--pretty=format:%C(auto)%cd %h%d %s",
				"origin/"+item.Value, "-10", "--")
			return fmt.Sprintf("Branch: %s\n\nRecent commits:\n%s", item.Value, out)
		},
	})
	if err != nil {
		return err
	}
	if result.Canceled || len(result.Items) == 0 {
		return nil
	}

	selected := result.Items[0]

	if selected.Value == "__create_new__" {
		return createNewBranch()
	}

	// Create worktree from selected remote branch
	branch := selected.Value
	ui.Infof("Creating worktree for '%s' from origin/%s...", branch, branch)
	if err := git.Run("worktree", "add", branch, "origin/"+branch); err != nil {
		return err
	}

	// Set upstream tracking
	fmt.Printf("Setting upstream to origin/%s\n", branch)
	return git.Run("branch", "--set-upstream-to=origin/"+branch, branch)
}

func createNewBranch() error {
	branchName := ui.PromptInput("Enter new branch name:")

	if branchName == "" {
		ui.Error("Branch name cannot be empty")
		return fmt.Errorf("branch name cannot be empty")
	}

	// Validate branch name using git
	if _, err := git.Query("check-ref-format", "--branch", branchName); err != nil {
		ui.Errorf("Invalid branch name '%s'", branchName)
		return fmt.Errorf("invalid branch name '%s'", branchName)
	}

	wtPath := ui.PromptInput(fmt.Sprintf("Enter worktree path [default: %s]:", branchName))
	if wtPath == "" {
		wtPath = branchName
	}

	ui.Infof("Creating new branch '%s' and worktree at '%s'...", branchName, wtPath)
	return git.Run("worktree", "add", "-b", branchName, wtPath)
}

func runAddDirect(args []string) error {
	// Parse arguments to capture -b/-B branch name for upstream tracking
	var branch string
	var gitArgs []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-b", "-B":
			if i+1 < len(args) {
				branch = args[i+1]
				gitArgs = append(gitArgs, args[i], args[i+1])
				i++
			}
		case "--reason":
			if i+1 < len(args) {
				gitArgs = append(gitArgs, args[i], args[i+1])
				i++
			}
		default:
			gitArgs = append(gitArgs, args[i])
		}
	}

	// Create the worktree
	fullArgs := append([]string{"worktree", "add"}, gitArgs...)
	if err := git.Run(fullArgs...); err != nil {
		return err
	}

	// Set upstream tracking if -b/-B was used
	if branch != "" {
		if _, err := git.Query("rev-parse", "--verify", "origin/"+branch); err == nil {
			fmt.Printf("Setting upstream to origin/%s\n", branch)
			return git.Run("branch", "--set-upstream-to=origin/"+branch, branch)
		}
		fmt.Printf("\nBranch '%s' created locally.\nTo push and set upstream:\n  git push -u origin %s\n", branch, branch)
	}

	return nil
}

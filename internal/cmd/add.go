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
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runAdd,
}

func init() {
	f := addCmd.Flags()
	f.StringP("branch", "b", "", "Create a new branch")
	f.StringP("force-branch", "B", "", "Create or reset a branch")
	f.BoolP("detach", "d", false, "Detach HEAD at the new worktree")
	f.BoolP("force", "f", false, "Checkout even if branch is checked out in another worktree")
	f.Bool("lock", false, "Lock the worktree after creation")
	f.String("reason", "", "Lock reason (use with --lock)")
	f.BoolP("quiet", "q", false, "Suppress feedback messages")
	f.Bool("no-checkout", false, "Don't populate the worktree")
	f.Bool("no-track", false, "Don't set up upstream tracking")
	f.Bool("guess-remote", false, "Try to match new branch with remote-tracking branch")
	f.Bool("orphan", false, "Create worktree with an orphan branch")
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

	// If no arguments and no flags set, run interactive mode
	if len(args) == 0 && !cmd.Flags().Changed("branch") && !cmd.Flags().Changed("force-branch") {
		return runAddInteractive()
	}

	// Non-interactive: build git args from parsed flags
	return runAddDirect(cmd, args)
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
		Label: "➕ Create new branch",
		Value: "__create_new__",
	})

	for _, line := range lines {
		if strings.Contains(line, "HEAD") {
			continue
		}
		// Strip remote name prefix (e.g., "origin/feature" -> "feature")
		_, branch, _ := strings.Cut(line, "/")
		if branch == "" {
			continue
		}
		items = append(items, picker.Item{
			Label: branch,
			Value: branch,
		})
	}

	result, err := picker.Run(picker.Config{
		Items:      items,
		Prompt:     "Select branch or create new: ",
		PreviewCmd: previewBranchCmdStr(),
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
	if err := git.Run("worktree", "add", "-b", branch, "origin/"+branch); err != nil {
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

func runAddDirect(cmd *cobra.Command, args []string) error {
	var gitArgs []string

	branch, _ := cmd.Flags().GetString("branch")
	if branch != "" {
		gitArgs = append(gitArgs, "-b", branch)
	}
	forceBranch, _ := cmd.Flags().GetString("force-branch")
	if forceBranch != "" {
		gitArgs = append(gitArgs, "-B", forceBranch)
	}
	if detach, _ := cmd.Flags().GetBool("detach"); detach {
		gitArgs = append(gitArgs, "--detach")
	}
	if force, _ := cmd.Flags().GetBool("force"); force {
		gitArgs = append(gitArgs, "--force")
	}
	if lock, _ := cmd.Flags().GetBool("lock"); lock {
		gitArgs = append(gitArgs, "--lock")
	}
	if reason, _ := cmd.Flags().GetString("reason"); reason != "" {
		gitArgs = append(gitArgs, "--reason", reason)
	}
	if quiet, _ := cmd.Flags().GetBool("quiet"); quiet {
		gitArgs = append(gitArgs, "--quiet")
	}
	if noCheckout, _ := cmd.Flags().GetBool("no-checkout"); noCheckout {
		gitArgs = append(gitArgs, "--no-checkout")
	}
	if noTrack, _ := cmd.Flags().GetBool("no-track"); noTrack {
		gitArgs = append(gitArgs, "--no-track")
	}
	if guessRemote, _ := cmd.Flags().GetBool("guess-remote"); guessRemote {
		gitArgs = append(gitArgs, "--guess-remote")
	}
	if orphan, _ := cmd.Flags().GetBool("orphan"); orphan {
		gitArgs = append(gitArgs, "--orphan")
	}

	// Append positional args (path, commit-ish)
	gitArgs = append(gitArgs, args...)

	// Create the worktree
	fullArgs := append([]string{"worktree", "add"}, gitArgs...)
	if err := git.Run(fullArgs...); err != nil {
		return err
	}

	// Set upstream tracking if -b/-B was used
	trackBranch := branch
	if trackBranch == "" {
		trackBranch = forceBranch
	}
	if trackBranch != "" {
		if _, err := git.Query("rev-parse", "--verify", "origin/"+trackBranch); err == nil {
			fmt.Printf("Setting upstream to origin/%s\n", trackBranch)
			return git.Run("branch", "--set-upstream-to=origin/"+trackBranch, trackBranch)
		}
		fmt.Printf("\nBranch '%s' created locally.\nTo push and set upstream:\n  git push -u origin %s\n", trackBranch, trackBranch)
	}

	return nil
}

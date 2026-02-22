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

Always fetches from the remote before creating the worktree. When using -b/-B,
upstream tracking is set automatically if the branch exists on the remote.`,
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

	remote := worktree.DefaultRemote()

	if remote != "" {
		if err := ui.Spin(fmt.Sprintf("Fetching from %s", remote), func() error {
			_, err := git.RunWithOutput("fetch", remote, "--prune")
			return err
		}); err != nil {
			return err
		}
	}

	// If no arguments and no flags set, run interactive mode
	if len(args) == 0 && !cmd.Flags().Changed("branch") && !cmd.Flags().Changed("force-branch") {
		return runAddInteractive(remote)
	}

	// Non-interactive: build git args from parsed flags
	return runAddDirect(cmd, args, remote)
}

func runAddInteractive(remote string) error {
	// Build set of branches already checked out as worktrees
	checkedOut := make(map[string]bool)
	if entries, err := worktree.List(); err == nil {
		for _, e := range entries {
			if e.Branch != "" && e.Branch != "(detached)" {
				checkedOut[e.Branch] = true
			}
		}
	}

	var items []picker.Item
	// Add "Create new branch" option first
	items = append(items, picker.Item{
		Label: "➕ Create new branch",
		Value: "__create_new__",
	})

	// Get remote branches (excluding HEAD)
	lines, err := git.QueryLines("branch", "-r", "--format=%(refname:short)")
	if err != nil {
		return fmt.Errorf("failed to list remote branches: %w", err)
	}

	for _, line := range lines {
		if strings.Contains(line, "HEAD") {
			continue
		}
		// Strip remote name prefix (e.g., "origin/feature" -> "feature")
		_, branch, _ := strings.Cut(line, "/")
		if branch == "" {
			continue
		}
		if checkedOut[branch] {
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
	if err := ui.Spin(fmt.Sprintf("Creating worktree for %s", ui.Accent(branch)), func() error {
		_, err := git.RunWithOutput("worktree", "add", "-b", branch, branch, remote+"/"+branch)
		return err
	}); err != nil {
		return err
	}

	// Set upstream tracking
	if err := ui.Spin(fmt.Sprintf("Setting upstream to %s", ui.Accent(remote+"/"+branch)), func() error {
		_, err := git.RunWithOutput("branch", "--set-upstream-to="+remote+"/"+branch, branch)
		return err
	}); err != nil {
		return err
	}

	return nil
}

func createNewBranch() error {
	branchName := ui.PromptInput("Enter new branch name:")

	if branchName == "" {
		ui.Error("Branch name cannot be empty")
		return fmt.Errorf("branch name cannot be empty")
	}

	wtPath := branchName

	return ui.Spin(fmt.Sprintf("Creating worktree for %s", ui.Accent(branchName)), func() error {
		_, err := git.RunWithOutput("worktree", "add", "-b", branchName, wtPath)
		return err
	})
}

func runAddDirect(cmd *cobra.Command, args []string, remote string) error {
	var gitArgs []string

	branch, _ := cmd.Flags().GetString("branch")
	if branch != "" {
		gitArgs = append(gitArgs, "-b", branch)
	}
	forceBranch, _ := cmd.Flags().GetString("force-branch")
	if forceBranch != "" {
		gitArgs = append(gitArgs, "-B", forceBranch)
	}
	if reason, _ := cmd.Flags().GetString("reason"); reason != "" {
		gitArgs = append(gitArgs, "--reason", reason)
	}

	for _, name := range []string{"detach", "force", "lock", "quiet", "no-checkout", "no-track", "guess-remote", "orphan"} {
		if v, _ := cmd.Flags().GetBool(name); v {
			gitArgs = append(gitArgs, "--"+name)
		}
	}

	// Append positional args (path, commit-ish)
	gitArgs = append(gitArgs, args...)

	// Create the worktree
	fullArgs := append([]string{"worktree", "add"}, gitArgs...)
	if err := ui.Spin("Creating worktree", func() error {
		_, err := git.RunWithOutput(fullArgs...)
		return err
	}); err != nil {
		return err
	}

	// Set upstream tracking if -b/-B was used
	trackBranch := branch
	if trackBranch == "" {
		trackBranch = forceBranch
	}
	if trackBranch != "" && remote != "" {
		if _, err := git.Query("rev-parse", "--verify", remote+"/"+trackBranch); err == nil {
			if err := ui.Spin(fmt.Sprintf("Setting upstream to %s", ui.Accent(remote+"/"+trackBranch)), func() error {
				_, err := git.RunWithOutput("branch", "--set-upstream-to="+remote+"/"+trackBranch, trackBranch)
				return err
			}); err != nil {
				return err
			}
		} else {
			fmt.Printf("\nBranch %s created locally.\nTo push and set upstream:\n  %s\n",
				ui.Accent(trackBranch), ui.Muted("git push -u "+remote+" "+trackBranch))
		}
	}

	return nil
}

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
upstream tracking is set automatically if the branch exists on the remote.

On success, prints the absolute created worktree path to stdout. Human-oriented
progress, prompts, and hints are written to stderr.`,
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
	// Change to bare root.
	root, err := worktree.BareRoot()
	if err != nil {
		return err
	}
	if err := os.Chdir(root); err != nil {
		return fmt.Errorf("failed to change to bare root: %w", err)
	}

	interactive := len(args) == 0 && !cmd.Flags().Changed("branch") && !cmd.Flags().Changed("force-branch")
	if interactive {
		createdPath, err := runAddInteractive()
		if err != nil {
			return err
		}
		return printCreatedWorktreePath(createdPath)
	}

	remote := worktree.DefaultRemote()
	if remote != "" {
		if err := ui.SpinWithOutputRawContext(fmt.Sprintf("Fetching from %s", remote), func(ctx context.Context, w io.Writer) error {
			return git.RunToContext(ctx, w, "fetch", remote, "--prune")
		}); err != nil {
			return err
		}
	}

	// Non-interactive: build git args from parsed flags.
	createdPath, err := runAddDirect(cmd, args, remote)
	if err != nil {
		return err
	}
	return printCreatedWorktreePath(createdPath)
}

func fetchInteractiveBranches() error {
	remotes, err := git.QueryLines("remote")
	if err != nil || len(remotes) == 0 {
		return nil
	}

	return ui.SpinWithOutputRawContext("Fetching from all remotes", func(ctx context.Context, w io.Writer) error {
		return git.RunToContext(ctx, w, "fetch", "--all", "--prune")
	})
}

func runAddInteractive() (string, error) {
	items, err := loadInteractiveAddItems(context.Background())
	if errors.Is(err, context.Canceled) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", nil
	}

	result, err := picker.Run(picker.Config{
		Items:      items,
		Prompt:     "Select branch or create new: ",
		PreviewCmd: previewBranchCmdStr(),
	})
	if err != nil {
		return "", err
	}

	if result.Canceled || len(result.Items) == 0 {
		return "", nil
	}

	selected := result.Items[0]

	if selected.Value == createNewBranchValue {
		return createNewBranch()
	}

	remote, branch := splitRemoteBranchRef(selected.Value)
	if remote == "" || branch == "" {
		return "", fmt.Errorf("invalid remote branch ref: %s", selected.Value)
	}

	wtPath := promptWorktreePath(branch)

	// Create worktree from selected remote branch.
	if err := ui.SpinWithOutputContext(fmt.Sprintf("Creating worktree for %s", ui.Accent(branch)), func(ctx context.Context, w io.Writer) error {
		return git.RunToContext(ctx, w, "worktree", "add", "-b", branch, wtPath, selected.Value)
	}); err != nil {
		return "", err
	}

	// Set upstream tracking.
	if err := ui.SpinWithOutputContext(fmt.Sprintf("Setting upstream to %s", ui.Accent(selected.Value)), func(ctx context.Context, w io.Writer) error {
		return git.RunToContext(ctx, w, "branch", "--set-upstream-to="+selected.Value, branch)
	}); err != nil {
		return "", err
	}

	return wtPath, nil
}

func createNewBranch() (string, error) {
	branchName := ui.PromptInput("Enter new branch name:")

	if branchName == "" {
		ui.Error("Branch name cannot be empty")
		return "", fmt.Errorf("branch name cannot be empty")
	}

	wtPath := promptWorktreePath(branchName)

	if err := ui.SpinWithOutputContext(fmt.Sprintf("Creating worktree for %s", ui.Accent(branchName)), func(ctx context.Context, w io.Writer) error {
		return git.RunToContext(ctx, w, "worktree", "add", "-b", branchName, wtPath)
	}); err != nil {
		return "", err
	}
	return wtPath, nil
}

func promptWorktreePath(defaultPath string) string {
	wtPath := ui.PromptInput(fmt.Sprintf("Enter worktree path (optional, default: %s):", defaultPath))
	if wtPath == "" {
		return defaultPath
	}
	return wtPath
}

func runAddDirect(cmd *cobra.Command, args []string, remote string) (string, error) {
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

	// Append positional args (path, commit-ish).
	gitArgs = append(gitArgs, args...)

	createdPath := ""
	if len(args) > 0 {
		createdPath = args[0]
	}

	// Create the worktree.
	fullArgs := append([]string{"worktree", "add"}, gitArgs...)
	if err := ui.SpinWithOutputContext("Creating worktree", func(ctx context.Context, w io.Writer) error {
		return git.RunToContext(ctx, w, fullArgs...)
	}); err != nil {
		return "", err
	}

	// Set upstream tracking if -b/-B was used.
	trackBranch := branch
	if trackBranch == "" {
		trackBranch = forceBranch
	}
	if trackBranch != "" && remote != "" {
		if _, err := git.Query("rev-parse", "--verify", remote+"/"+trackBranch); err == nil {
			if err := ui.SpinWithOutputContext(fmt.Sprintf("Setting upstream to %s", ui.Accent(remote+"/"+trackBranch)), func(ctx context.Context, w io.Writer) error {
				return git.RunToContext(ctx, w, "branch", "--set-upstream-to="+remote+"/"+trackBranch, trackBranch)
			}); err != nil {
				return "", err
			}
		} else {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, renderCommandHintsSection([]commandHint{{
				Action:  fmt.Sprintf("Push %s and set upstream", ui.Accent(trackBranch)),
				Command: "git push -u " + remote + " " + trackBranch,
			}}))
		}
	}

	return createdPath, nil
}

func splitRemoteBranchRef(remoteRef string) (remote string, branch string) {
	remote, branch, _ = strings.Cut(remoteRef, "/")
	return remote, branch
}

func printCreatedWorktreePath(path string) error {
	if path == "" {
		return nil
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if resolvedPath, err := filepath.EvalSymlinks(absPath); err == nil {
		absPath = resolvedPath
	}
	fmt.Println(absPath)
	return nil
}

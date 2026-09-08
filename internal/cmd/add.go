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
	"github.com/ahmedelgabri/git-wt/internal/hook"
	"github.com/ahmedelgabri/git-wt/internal/picker"
	"github.com/ahmedelgabri/git-wt/internal/ui"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [options] [<path>] [<commit-ish>]",
	Short: "Create a new worktree",
	Long: `Create a new worktree. With no arguments, opens an interactive picker to
select from remote branches or create a new branch. Common git worktree add
flags are supported; see the options below.

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
	// Resolve paths once without changing the process working directory.
	root, err := worktree.BareRoot()
	if err != nil {
		return err
	}

	var createdPath string

	interactive := len(args) == 0 && !cmd.Flags().Changed("branch") && !cmd.Flags().Changed("force-branch")
	if interactive {
		createdPath, err = runAddInteractive(root)
		if err != nil {
			return err
		}
	} else {
		remote := worktree.DefaultRemoteIn(root)
		if remote != "" {
			if err := ui.SpinWithOutputRawContext(fmt.Sprintf("Fetching from %s", remote), func(ctx context.Context, w io.Writer) error {
				return git.RunInToContext(ctx, root, w, "fetch", remote, "--prune")
			}); err != nil {
				return err
			}
		}

		createdPath, err = runAddDirect(cmd, args, remote, root)
		if err != nil {
			return err
		}
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

func runAddInteractive(root string) (string, error) {
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
		return createNewBranch(root)
	}

	remote, branch := splitRemoteBranchRef(selected.Value)
	if remote == "" || branch == "" {
		return "", fmt.Errorf("invalid remote branch ref: %s", selected.Value)
	}

	wtPath, err := promptWorktreePath(branch)
	if err != nil {
		return "", err
	}

	wtPath = addPath(root, wtPath)
	_, existsErr := git.QueryIn(root, "show-ref", "--verify", "refs/heads/"+branch)
	existing := existsErr == nil
	err = runAddLifecycle(wtPath, branch, func() error {
		// Create worktree from selected remote branch.
		if err := ui.SpinWithOutputContext(fmt.Sprintf("Creating worktree for %s", ui.Accent(branch)), func(ctx context.Context, w io.Writer) error {
			if existing {
				return git.RunInToContext(ctx, root, w, "worktree", "add", "--", wtPath, branch)
			}
			return git.RunInToContext(ctx, root, w, "worktree", "add", "-b", branch, "--", wtPath, selected.Value)
		}); err != nil {
			return err
		}

		// Existing branches keep their configured upstream.
		if existing {
			return nil
		}
		// Set upstream tracking.
		return ui.SpinWithOutputContext(fmt.Sprintf("Setting upstream to %s", ui.Accent(selected.Value)), func(ctx context.Context, w io.Writer) error {
			return git.RunInToContext(ctx, root, w, "branch", "--set-upstream-to="+selected.Value, branch)
		})
	})
	if err != nil {
		return "", err
	}
	return wtPath, nil
}

func createNewBranch(root string) (string, error) {
	branchName, err := ui.PromptInputResult("Enter new branch name:")
	if err != nil {
		return "", err
	}

	if branchName == "" {
		ui.Error("Branch name cannot be empty")
		return "", fmt.Errorf("branch name cannot be empty")
	}

	wtPath, err := promptWorktreePath(branchName)
	if err != nil {
		return "", err
	}

	wtPath = addPath(root, wtPath)
	err = runAddLifecycle(wtPath, branchName, func() error {
		return ui.SpinWithOutputContext(fmt.Sprintf("Creating worktree for %s", ui.Accent(branchName)), func(ctx context.Context, w io.Writer) error {
			return git.RunInToContext(ctx, root, w, "worktree", "add", "-b", branchName, "--", wtPath)
		})
	})
	if err != nil {
		return "", err
	}
	return wtPath, nil
}

func promptWorktreePath(defaultPath string) (string, error) {
	wtPath, err := ui.PromptInputResult(fmt.Sprintf("Enter worktree path (optional, default: %s):", defaultPath))
	if err != nil {
		return "", err
	}
	if wtPath == "" {
		return defaultPath, nil
	}
	return wtPath, nil
}

func addPath(root, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(root, path)
}

func runAddDirect(cmd *cobra.Command, args []string, remote, root string) (string, error) {
	args = append([]string(nil), args...)
	if len(args) > 0 {
		args[0] = addPath(root, args[0])
	}
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
	gitArgs = append(gitArgs, "--")
	gitArgs = append(gitArgs, args...)

	createdPath := ""
	if len(args) > 0 {
		createdPath = args[0]
	}

	trackBranch := branch
	if trackBranch == "" {
		trackBranch = forceBranch
	}

	fullArgs := append([]string{"worktree", "add"}, gitArgs...)
	err := runAddLifecycle(createdPath, trackBranch, func() error {
		// Create the worktree.
		if err := ui.SpinWithOutputContext("Creating worktree", func(ctx context.Context, w io.Writer) error {
			return git.RunInToContext(ctx, root, w, fullArgs...)
		}); err != nil {
			return err
		}

		// Set upstream tracking if -b/-B was used.
		if trackBranch != "" && remote != "" && !boolFlag(cmd, "no-track") {
			if _, err := git.QueryIn(root, "rev-parse", "--verify", "refs/remotes/"+remote+"/"+trackBranch); err == nil {
				if err := ui.SpinWithOutputContext(fmt.Sprintf("Setting upstream to %s", ui.Accent(remote+"/"+trackBranch)), func(ctx context.Context, w io.Writer) error {
					return git.RunInToContext(ctx, root, w, "branch", "--set-upstream-to="+remote+"/"+trackBranch, trackBranch)
				}); err != nil {
					return err
				}
			} else {
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, renderCommandHintsSectionFor(os.Stderr, []commandHint{{
					Action:  fmt.Sprintf("Push %s and set upstream", ui.Accent(trackBranch)),
					Command: "git push -u " + remote + " " + trackBranch,
				}}))
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return createdPath, nil
}

func splitRemoteBranchRef(remoteRef string) (remote string, branch string) {
	remote, branch, _ = strings.Cut(remoteRef, "/")
	return remote, branch
}

func runAddLifecycle(wtPath, branch string, create func() error) error {
	if wtPath == "" {
		return create()
	}

	beforeHooks, err := hook.Load(hook.BeforeAdd)
	if err != nil {
		return err
	}
	afterHooks, err := hook.Load(hook.AfterAdd)
	if err != nil {
		return err
	}

	absPath, err := filepath.Abs(wtPath)
	if err != nil {
		return err
	}
	bareRoot, err := worktree.BareRoot()
	if err != nil {
		return err
	}

	invocation := hook.Invocation{
		Event:        hook.BeforeAdd,
		Dir:          bareRoot,
		WorktreePath: absPath,
		Branch:       branch,
		BareRoot:     bareRoot,
	}
	if err := hook.Run(context.Background(), beforeHooks, invocation, os.Stderr); err != nil {
		return err
	}
	if err := create(); err != nil {
		return err
	}

	invocation.Event = hook.AfterAdd
	invocation.Dir = absPath
	if err := hook.Run(context.Background(), afterHooks, invocation, os.Stderr); err != nil {
		if resolvedPath, pathErr := resolveCreatedPath(wtPath); pathErr == nil {
			fmt.Fprintf(os.Stderr, "Worktree was created at %s, but wt.afteradd failed\n", resolvedPath)
		}
		return err
	}
	return nil
}

func resolveCreatedPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolvedPath, err := filepath.EvalSymlinks(absPath); err == nil {
		absPath = resolvedPath
	}
	return absPath, nil
}

func printCreatedWorktreePath(path string) error {
	if path == "" {
		return nil
	}
	absPath, err := resolveCreatedPath(path)
	if err != nil {
		return err
	}
	fmt.Println(absPath)
	return nil
}

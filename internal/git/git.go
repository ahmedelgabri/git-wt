package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// ExecOptions configures git command execution.
type ExecOptions struct {
	Dir      string
	Mutating bool
	StreamTo io.Writer
	Capture  bool
	Combined bool
	Raw      bool
	Env      []string
	Context  context.Context
	Stdin    io.Reader
}

// debug returns whether mutation commands should be echoed instead of executed.
func debug() bool { return os.Getenv("DEBUG") != "" }

// Debug reports whether DEBUG mode is enabled, for callers outside this package.
func Debug() bool { return debug() }

func execGit(opts ExecOptions, args ...string) (string, error) {
	if opts.Mutating && debug() {
		msg := formatDebugCommand(opts.Dir, args)
		if opts.StreamTo != nil {
			fmt.Fprintln(opts.StreamTo, msg)
		} else {
			fmt.Println(msg)
		}
		return "", nil
	}

	cmd := newCommand(opts.Context, args...)
	cmd.Dir = opts.Dir
	cmd.Env = os.Environ()
	if opts.Dir != "" {
		cmd.Env = RepositoryEnv()
	}
	cmd.Env = append(cmd.Env, opts.Env...)
	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	}

	if opts.StreamTo != nil {
		cmd.Stdout = opts.StreamTo
		cmd.Stderr = opts.StreamTo
		if cmd.Stdin == nil {
			cmd.Stdin = os.Stdin
		}
		return "", cmd.Run()
	}

	if opts.Capture {
		var out []byte
		var err error
		if opts.Combined {
			out, err = cmd.CombinedOutput()
		} else {
			out, err = cmd.Output()
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			err = fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)), err)
		}
		if opts.Raw {
			return string(out), err
		}
		return strings.TrimSpace(string(out)), err
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if cmd.Stdin == nil {
		cmd.Stdin = os.Stdin
	}
	return "", cmd.Run()
}

// RepositoryEnv removes inherited repository selectors before changing repos.
// Explicit ExecOptions.Env overrides are applied afterward.
func RepositoryEnv() []string {
	return slices.DeleteFunc(os.Environ(), func(value string) bool {
		key, _, _ := strings.Cut(value, "=")
		switch key {
		case "GIT_DIR", "GIT_COMMON_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_PREFIX":
			return true
		default:
			return false
		}
	})
}

// QueryPath removes Git's record terminator without trimming path characters.
func QueryPath(args ...string) (string, error) {
	out, err := QueryRaw(args...)
	return strings.TrimSuffix(out, "\n"), err
}

func QueryPathInContext(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := QueryRawInContext(ctx, dir, args...)
	return strings.TrimSuffix(out, "\n"), err
}

func QueryRawInContext(ctx context.Context, dir string, args ...string) (string, error) {
	return execGit(ExecOptions{Dir: dir, Capture: true, Raw: true, Context: ctx}, args...)
}

func newCommand(ctx context.Context, args ...string) *exec.Cmd {
	if ctx == nil {
		ctx = context.Background()
	}
	return exec.CommandContext(ctx, "git", args...)
}

func formatDebugCommand(dir string, args []string) string {
	if dir == "" {
		return "git " + strings.Join(args, " ")
	}
	return fmt.Sprintf("[in %s] git %s", dir, strings.Join(args, " "))
}

// Run executes a git mutation command. In DEBUG mode, it prints the command
// instead of executing it.
func Run(args ...string) error {
	return RunContext(context.Background(), args...)
}

// RunContext executes a git mutation command with an optional context.
func RunContext(ctx context.Context, args ...string) error {
	_, err := execGit(ExecOptions{Mutating: true, Context: ctx}, args...)
	return err
}

// RunIn executes a git mutation command in the specified directory.
func RunIn(dir string, args ...string) error {
	return RunInContext(context.Background(), dir, args...)
}

// RunInContext executes a git mutation command in the specified directory with
// an optional context.
func RunInContext(ctx context.Context, dir string, args ...string) error {
	_, err := execGit(ExecOptions{Dir: dir, Mutating: true, Context: ctx}, args...)
	return err
}

// RunWithOutput executes a git mutation command and returns its combined output.
func RunWithOutput(args ...string) (string, error) {
	return RunWithOutputContext(context.Background(), args...)
}

// RunWithOutputContext executes a git mutation command with an optional
// context and returns its combined output.
func RunWithOutputContext(ctx context.Context, args ...string) (string, error) {
	return execGit(ExecOptions{Mutating: true, Capture: true, Combined: true, Context: ctx}, args...)
}

// RunInWithOutput executes a git mutation command in the specified directory
// and returns its combined output.
func RunInWithOutput(dir string, args ...string) (string, error) {
	return RunInWithOutputContext(context.Background(), dir, args...)
}

// RunInWithOutputContext executes a git mutation command in the specified
// directory with an optional context and returns its combined output.
func RunInWithOutputContext(ctx context.Context, dir string, args ...string) (string, error) {
	return execGit(ExecOptions{Dir: dir, Mutating: true, Capture: true, Combined: true, Context: ctx}, args...)
}

// RunTo executes a git mutation command, streaming stdout and stderr to w.
func RunTo(w io.Writer, args ...string) error {
	return RunToContext(context.Background(), w, args...)
}

// RunToContext executes a git mutation command with an optional context,
// streaming stdout and stderr to w.
func RunToContext(ctx context.Context, w io.Writer, args ...string) error {
	_, err := execGit(ExecOptions{Mutating: true, StreamTo: w, Context: ctx}, args...)
	return err
}

// RunInTo executes a git mutation command in the specified directory,
// streaming stdout and stderr to w.
func RunInTo(dir string, w io.Writer, args ...string) error {
	return RunInToContext(context.Background(), dir, w, args...)
}

// RunInToContext executes a git mutation command in the specified directory
// with an optional context, streaming stdout and stderr to w.
func RunInToContext(ctx context.Context, dir string, w io.Writer, args ...string) error {
	_, err := execGit(ExecOptions{Dir: dir, Mutating: true, StreamTo: w, Context: ctx}, args...)
	return err
}

// Query executes a read-only git command (always runs, even in DEBUG mode).
func Query(args ...string) (string, error) {
	return QueryContext(context.Background(), args...)
}

// QueryContext executes a read-only git command with an optional context.
func QueryContext(ctx context.Context, args ...string) (string, error) {
	return execGit(ExecOptions{Capture: true, Context: ctx}, args...)
}

// QueryWithInput executes a read-only command with explicit stdin and captures
// stdout. Input is separate from argv, so large revision lists remain usable.
func QueryWithInput(input io.Reader, args ...string) (string, error) {
	return execGit(ExecOptions{Capture: true, Stdin: input, Context: context.Background()}, args...)
}

// QueryRaw executes a read-only git command without trimming its output.
func QueryRaw(args ...string) (string, error) {
	return QueryRawContext(context.Background(), args...)
}

// QueryRawContext executes a read-only git command without trimming its output.
func QueryRawContext(ctx context.Context, args ...string) (string, error) {
	return execGit(ExecOptions{Capture: true, Raw: true, Context: ctx}, args...)
}

// QueryRun executes a read-only git command, streaming stdout/stderr directly.
func QueryRun(args ...string) error {
	_, err := execGit(ExecOptions{Context: context.Background()}, args...)
	return err
}

// QueryIn executes a read-only git command in the specified directory.
func QueryIn(dir string, args ...string) (string, error) {
	return QueryInContext(context.Background(), dir, args...)
}

// QueryInContext executes a read-only git command in the specified directory
// with an optional context.
func QueryInContext(ctx context.Context, dir string, args ...string) (string, error) {
	return execGit(ExecOptions{Dir: dir, Capture: true, Context: ctx}, args...)
}

// QueryCombined executes a read-only git command and returns combined output.
func QueryCombined(args ...string) (string, error) {
	return execGit(ExecOptions{Capture: true, Combined: true, Context: context.Background()}, args...)
}

// QueryLines executes a read-only git command and returns output lines.
func QueryLines(args ...string) ([]string, error) {
	out, err := Query(args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

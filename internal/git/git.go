package git

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// ExecOptions configures git command execution.
type ExecOptions struct {
	Dir      string
	Mutating bool
	StreamTo io.Writer
	Capture  bool
	Combined bool
	Env      []string
	Context  context.Context
	Stdin    io.Reader
}

// debug returns whether mutation commands should be echoed instead of executed.
func debug() bool { return os.Getenv("DEBUG") != "" }

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
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}
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
		return strings.TrimSpace(string(out)), err
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if cmd.Stdin == nil {
		cmd.Stdin = os.Stdin
	}
	return "", cmd.Run()
}

func newCommand(ctx context.Context, args ...string) *exec.Cmd {
	if ctx != nil {
		return exec.CommandContext(ctx, "git", args...)
	}
	return exec.Command("git", args...)
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
	_, err := execGit(ExecOptions{Mutating: true}, args...)
	return err
}

// RunIn executes a git mutation command in the specified directory.
func RunIn(dir string, args ...string) error {
	_, err := execGit(ExecOptions{Dir: dir, Mutating: true}, args...)
	return err
}

// RunWithOutput executes a git mutation command and returns its combined output.
func RunWithOutput(args ...string) (string, error) {
	return execGit(ExecOptions{Mutating: true, Capture: true, Combined: true}, args...)
}

// RunInWithOutput executes a git mutation command in the specified directory
// and returns its combined output.
func RunInWithOutput(dir string, args ...string) (string, error) {
	return execGit(ExecOptions{Dir: dir, Mutating: true, Capture: true, Combined: true}, args...)
}

// RunTo executes a git mutation command, streaming stdout and stderr to w.
func RunTo(w io.Writer, args ...string) error {
	_, err := execGit(ExecOptions{Mutating: true, StreamTo: w}, args...)
	return err
}

// RunInTo executes a git mutation command in the specified directory,
// streaming stdout and stderr to w.
func RunInTo(dir string, w io.Writer, args ...string) error {
	_, err := execGit(ExecOptions{Dir: dir, Mutating: true, StreamTo: w}, args...)
	return err
}

// Query executes a read-only git command (always runs, even in DEBUG mode).
func Query(args ...string) (string, error) {
	return execGit(ExecOptions{Capture: true}, args...)
}

// QueryRun executes a read-only git command, streaming stdout/stderr directly.
func QueryRun(args ...string) error {
	_, err := execGit(ExecOptions{}, args...)
	return err
}

// QueryIn executes a read-only git command in the specified directory.
func QueryIn(dir string, args ...string) (string, error) {
	return execGit(ExecOptions{Dir: dir, Capture: true}, args...)
}

// QueryCombined executes a read-only git command and returns combined output.
func QueryCombined(args ...string) (string, error) {
	return execGit(ExecOptions{Capture: true, Combined: true}, args...)
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

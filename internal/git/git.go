package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Debug indicates whether mutation commands should be echoed instead of executed.
var Debug = os.Getenv("DEBUG") != ""

// Run executes a git mutation command. In DEBUG mode, it prints the command
// instead of executing it.
func Run(args ...string) error {
	if Debug {
		fmt.Println("git " + strings.Join(args, " "))
		return nil
	}
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// RunIn executes a git mutation command in the specified directory.
func RunIn(dir string, args ...string) error {
	if Debug {
		fmt.Printf("git -C %s %s\n", dir, strings.Join(args, " "))
		return nil
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// RunWithOutput executes a git mutation command and returns its combined output.
func RunWithOutput(args ...string) (string, error) {
	if Debug {
		s := "git " + strings.Join(args, " ")
		fmt.Println(s)
		return "", nil
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// RunInWithOutput executes a git mutation command in the specified directory
// and returns its combined output.
func RunInWithOutput(dir string, args ...string) (string, error) {
	if Debug {
		s := fmt.Sprintf("git -C %s %s", dir, strings.Join(args, " "))
		fmt.Println(s)
		return "", nil
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Query executes a read-only git command (always runs, even in DEBUG mode).
func Query(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// QueryIn executes a read-only git command in the specified directory.
func QueryIn(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// QueryCombined executes a read-only git command and returns combined output.
func QueryCombined(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
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

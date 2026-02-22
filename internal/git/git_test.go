package git

import (
	"os/exec"
	"strings"
	"testing"
)

func TestQueryVersion(t *testing.T) {
	out, err := Query("--version")
	if err != nil {
		t.Fatalf("Query(--version) error: %v", err)
	}
	if out == "" {
		t.Error("Query(--version) returned empty output")
	}
}

func TestQueryLines(t *testing.T) {
	// Query something that returns multiple lines
	lines, err := QueryLines("help", "-a")
	if err != nil {
		t.Fatalf("QueryLines(help, -a) error: %v", err)
	}
	if len(lines) == 0 {
		t.Error("QueryLines(help, -a) returned no lines")
	}
}

func TestDebugMode(t *testing.T) {
	t.Setenv("DEBUG", "1")

	// In debug mode, Run should not actually execute git commands
	err := Run("status")
	if err != nil {
		t.Errorf("Run() in debug mode should not error, got: %v", err)
	}
}

func TestDebugEnvVar(t *testing.T) {
	t.Setenv("DEBUG", "1")

	if !debug() {
		t.Error("debug() should be true when DEBUG env is set")
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return dir
}

func TestRunNonDebug(t *testing.T) {
	t.Setenv("DEBUG", "")

	if err := Run("--version"); err != nil {
		t.Errorf("Run(--version) error: %v", err)
	}
}

func TestRunInDebug(t *testing.T) {
	t.Setenv("DEBUG", "1")

	dir := t.TempDir()
	if err := RunIn(dir, "status"); err != nil {
		t.Errorf("RunIn() in debug mode should not error, got: %v", err)
	}
}

func TestRunInNonDebug(t *testing.T) {
	t.Setenv("DEBUG", "")

	repo := initGitRepo(t)
	if err := RunIn(repo, "status"); err != nil {
		t.Errorf("RunIn(%s, status) error: %v", repo, err)
	}
}

func TestRunWithOutputDebug(t *testing.T) {
	t.Setenv("DEBUG", "1")

	out, err := RunWithOutput("status")
	if err != nil {
		t.Errorf("RunWithOutput() in debug mode error: %v", err)
	}
	if out != "" {
		t.Errorf("RunWithOutput() in debug mode = %q, want empty", out)
	}
}

func TestRunWithOutputNonDebug(t *testing.T) {
	t.Setenv("DEBUG", "")

	out, err := RunWithOutput("--version")
	if err != nil {
		t.Errorf("RunWithOutput(--version) error: %v", err)
	}
	if !strings.Contains(out, "git version") {
		t.Errorf("RunWithOutput(--version) = %q, want to contain 'git version'", out)
	}
}

func TestRunInWithOutputDebug(t *testing.T) {
	t.Setenv("DEBUG", "1")

	out, err := RunInWithOutput(t.TempDir(), "status")
	if err != nil {
		t.Errorf("RunInWithOutput() in debug mode error: %v", err)
	}
	if out != "" {
		t.Errorf("RunInWithOutput() in debug mode = %q, want empty", out)
	}
}

func TestRunInWithOutputNonDebug(t *testing.T) {
	t.Setenv("DEBUG", "")

	repo := initGitRepo(t)
	out, err := RunInWithOutput(repo, "status")
	if err != nil {
		t.Errorf("RunInWithOutput(%s, status) error: %v", repo, err)
	}
	if out == "" {
		t.Error("RunInWithOutput() returned empty output")
	}
}

func TestQueryIn(t *testing.T) {
	repo := initGitRepo(t)
	out, err := QueryIn(repo, "rev-parse", "--git-dir")
	if err != nil {
		t.Fatalf("QueryIn(rev-parse --git-dir) error: %v", err)
	}
	if !strings.Contains(out, ".git") {
		t.Errorf("QueryIn output = %q, want to contain '.git'", out)
	}
}

func TestQueryCombined(t *testing.T) {
	out, err := QueryCombined("--version")
	if err != nil {
		t.Fatalf("QueryCombined(--version) error: %v", err)
	}
	if !strings.Contains(out, "git version") {
		t.Errorf("QueryCombined(--version) = %q, want to contain 'git version'", out)
	}
}

func TestQueryCombinedError(t *testing.T) {
	_, err := QueryCombined("nonexistent-subcommand")
	if err == nil {
		t.Error("QueryCombined(nonexistent-subcommand) should return error")
	}
}

func TestQueryLinesEmpty(t *testing.T) {
	repo := initGitRepo(t)
	lines, err := QueryLines("-C", repo, "tag", "-l")
	if err != nil {
		t.Fatalf("QueryLines(tag -l) error: %v", err)
	}
	if lines != nil {
		t.Errorf("QueryLines(tag -l) = %v, want nil", lines)
	}
}

func TestQueryLinesError(t *testing.T) {
	_, err := QueryLines("nonexistent-subcommand")
	if err == nil {
		t.Error("QueryLines(nonexistent-subcommand) should return error")
	}
}

func TestDebugDefaultOff(t *testing.T) {
	t.Setenv("DEBUG", "")

	if debug() {
		t.Error("debug() should be false when DEBUG env is empty")
	}
}

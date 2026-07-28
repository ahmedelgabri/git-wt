package git

import (
	"bytes"
	"context"
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

func TestQueryRawPreservesWhitespace(t *testing.T) {
	out, err := QueryRaw("-c", "raw.value=  value  ", "config", "--null", "--get-regexp", "^raw\\.")
	if err != nil {
		t.Fatalf("QueryRaw(config) error: %v", err)
	}
	if out != "raw.value\n  value  \x00" {
		t.Errorf("QueryRaw(config) = %q, want untrimmed output", out)
	}
}

func TestQueryRun(t *testing.T) {
	if err := QueryRun("--version"); err != nil {
		t.Fatalf("QueryRun(--version) error: %v", err)
	}
}

func TestExecGitWithContext(t *testing.T) {
	out, err := execGit(ExecOptions{Capture: true, Context: context.Background()}, "--version")
	if err != nil {
		t.Fatalf("execGit(--version) error: %v", err)
	}
	if !strings.Contains(out, "git version") {
		t.Errorf("execGit(--version) = %q, want to contain 'git version'", out)
	}
}

func TestExecGitWithEnv(t *testing.T) {
	out, err := execGit(ExecOptions{Capture: true, Env: []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=user.name", "GIT_CONFIG_VALUE_0=Env Test"}}, "config", "user.name")
	if err != nil {
		t.Fatalf("execGit(config user.name) error: %v", err)
	}
	if out != "Env Test" {
		t.Errorf("execGit(config user.name) = %q, want %q", out, "Env Test")
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

func TestRunToDebug(t *testing.T) {
	t.Setenv("DEBUG", "1")

	var buf bytes.Buffer
	if err := RunTo(&buf, "status"); err != nil {
		t.Errorf("RunTo() in debug mode error: %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "git status") {
		t.Errorf("RunTo() debug output = %q, want to contain 'git status'", got)
	}
}

func TestRunToNonDebug(t *testing.T) {
	t.Setenv("DEBUG", "")

	var buf bytes.Buffer
	if err := RunTo(&buf, "--version"); err != nil {
		t.Errorf("RunTo(--version) error: %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "git version") {
		t.Errorf("RunTo(--version) = %q, want to contain 'git version'", got)
	}
}

func TestRunInToDebug(t *testing.T) {
	t.Setenv("DEBUG", "1")

	dir := t.TempDir()
	var buf bytes.Buffer
	if err := RunInTo(dir, &buf, "status"); err != nil {
		t.Errorf("RunInTo() in debug mode error: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "git status") {
		t.Errorf("RunInTo() debug output = %q, want to contain 'git status'", got)
	}
	if !strings.Contains(got, dir) {
		t.Errorf("RunInTo() debug output = %q, want to contain dir %q", got, dir)
	}
}

func TestRunInToNonDebug(t *testing.T) {
	t.Setenv("DEBUG", "")

	repo := initGitRepo(t)
	var buf bytes.Buffer
	if err := RunInTo(repo, &buf, "status"); err != nil {
		t.Errorf("RunInTo(%s, status) error: %v", repo, err)
	}
	if buf.Len() == 0 {
		t.Error("RunInTo() returned empty output")
	}
}

func TestRunContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := RunContext(ctx, "--version"); err == nil {
		t.Fatal("RunContext() with canceled context should return error")
	}
}

func TestRunToContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var buf bytes.Buffer
	if err := RunToContext(ctx, &buf, "--version"); err == nil {
		t.Fatal("RunToContext() with canceled context should return error")
	}
}

func TestQueryContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := QueryContext(ctx, "--version"); err == nil {
		t.Fatal("QueryContext() with canceled context should return error")
	}
}

func TestQueryInContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repo := initGitRepo(t)
	if _, err := QueryInContext(ctx, repo, "status", "--short"); err == nil {
		t.Fatal("QueryInContext() with canceled context should return error")
	}
}

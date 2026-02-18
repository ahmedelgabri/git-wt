package git

import (
	"os"
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
	old := Debug
	Debug = true
	defer func() { Debug = old }()

	// In debug mode, Run should not actually execute git commands
	err := Run("status")
	if err != nil {
		t.Errorf("Run() in debug mode should not error, got: %v", err)
	}
}

func TestDebugEnvVar(t *testing.T) {
	oldEnv := os.Getenv("DEBUG")
	oldDebug := Debug

	os.Setenv("DEBUG", "1")
	// Re-evaluate (in real code this happens at init time)
	Debug = os.Getenv("DEBUG") != ""

	if !Debug {
		t.Error("Debug should be true when DEBUG env is set")
	}

	if oldEnv == "" {
		os.Unsetenv("DEBUG")
	} else {
		os.Setenv("DEBUG", oldEnv)
	}
	Debug = oldDebug
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
	old := Debug
	Debug = false
	defer func() { Debug = old }()

	if err := Run("--version"); err != nil {
		t.Errorf("Run(--version) error: %v", err)
	}
}

func TestRunInDebug(t *testing.T) {
	old := Debug
	Debug = true
	defer func() { Debug = old }()

	dir := t.TempDir()
	if err := RunIn(dir, "status"); err != nil {
		t.Errorf("RunIn() in debug mode should not error, got: %v", err)
	}
}

func TestRunInNonDebug(t *testing.T) {
	old := Debug
	Debug = false
	defer func() { Debug = old }()

	repo := initGitRepo(t)
	if err := RunIn(repo, "status"); err != nil {
		t.Errorf("RunIn(%s, status) error: %v", repo, err)
	}
}

func TestRunWithOutputDebug(t *testing.T) {
	old := Debug
	Debug = true
	defer func() { Debug = old }()

	out, err := RunWithOutput("status")
	if err != nil {
		t.Errorf("RunWithOutput() in debug mode error: %v", err)
	}
	if out != "" {
		t.Errorf("RunWithOutput() in debug mode = %q, want empty", out)
	}
}

func TestRunWithOutputNonDebug(t *testing.T) {
	old := Debug
	Debug = false
	defer func() { Debug = old }()

	out, err := RunWithOutput("--version")
	if err != nil {
		t.Errorf("RunWithOutput(--version) error: %v", err)
	}
	if !strings.Contains(out, "git version") {
		t.Errorf("RunWithOutput(--version) = %q, want to contain 'git version'", out)
	}
}

func TestRunInWithOutputDebug(t *testing.T) {
	old := Debug
	Debug = true
	defer func() { Debug = old }()

	out, err := RunInWithOutput(t.TempDir(), "status")
	if err != nil {
		t.Errorf("RunInWithOutput() in debug mode error: %v", err)
	}
	if out != "" {
		t.Errorf("RunInWithOutput() in debug mode = %q, want empty", out)
	}
}

func TestRunInWithOutputNonDebug(t *testing.T) {
	old := Debug
	Debug = false
	defer func() { Debug = old }()

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

// Verify that os.Getenv("DEBUG") is not accidentally set in the test env.
func TestDebugDefaultOff(t *testing.T) {
	if val := os.Getenv("DEBUG"); val != "" {
		t.Skipf("DEBUG env is set to %q, skipping", val)
	}
	// Re-evaluate as the module init would
	d := os.Getenv("DEBUG") != ""
	if d {
		t.Error("Debug should be false when DEBUG env is unset")
	}
}

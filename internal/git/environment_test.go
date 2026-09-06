package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestQueryInIgnoresInheritedRepositorySelectors(t *testing.T) {
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "--bare", dir).CombinedOutput(); err != nil {
		t.Fatalf("init: %s: %v", out, err)
	}
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "external-index"))
	got, err := QueryIn(dir, "rev-parse", "--is-bare-repository")
	if err != nil || got != "true" {
		t.Fatalf("query resolved wrong repository: %q, %v", got, err)
	}
	if _, err := os.Stat(os.Getenv("GIT_INDEX_FILE")); !os.IsNotExist(err) {
		t.Fatalf("external index modified: %v", err)
	}
}

func TestQueryPreservesGitStderrAndExitError(t *testing.T) {
	_, err := QueryInContext(context.Background(), t.TempDir(), "rev-parse", "--git-dir")
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("lost Git diagnostic: %v", err)
	}
}

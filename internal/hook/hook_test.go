package hook

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoadConfigPreservesMultilineValues(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "2")
	t.Setenv("GIT_CONFIG_KEY_0", "wt.testhook")
	t.Setenv("GIT_CONFIG_VALUE_0", "printf 'first\\nsecond\\n'")
	t.Setenv("GIT_CONFIG_KEY_1", "wt.testhook")
	t.Setenv("GIT_CONFIG_VALUE_1", "touch done")

	hooks, err := LoadConfig("wt.testhook")
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	want := []string{"printf 'first\\nsecond\\n'", "touch done"}
	if !slices.Equal(hooks, want) {
		t.Fatalf("LoadConfig() = %#v, want %#v", hooks, want)
	}
}

func TestLoadConfigReturnsNilForMissingKey(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "0")

	hooks, err := LoadConfig("wt.missing")
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if hooks != nil {
		t.Fatalf("LoadConfig() = %#v, want nil", hooks)
	}
}

func TestRunSetsLifecycleEnvironmentAndDirectory(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "context")
	invocation := Invocation{
		Event:        BeforeAdd,
		Dir:          dir,
		WorktreePath: filepath.Join(dir, "worktree"),
		Branch:       "feature",
		BareRoot:     dir,
	}
	hooks := []string{"printf '%s\\n%s\\n%s\\n%s\\n%s\\n' \"$PWD\" \"$GIT_WT_EVENT\" \"$GIT_WT_PATH\" \"$GIT_WT_BRANCH\" \"$GIT_WT_BARE_ROOT\" > context"}

	if err := Run(context.Background(), hooks, invocation, os.Stderr); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks() error: %v", err)
	}
	want := strings.Join([]string{resolvedDir, "beforeadd", invocation.WorktreePath, "feature", dir, ""}, "\n")
	if string(got) != want {
		t.Fatalf("hook context = %q, want %q", got, want)
	}
}

func TestRunStopsAfterFirstFailure(t *testing.T) {
	dir := t.TempDir()
	invocation := Invocation{Event: AfterAdd, Dir: dir}
	hooks := []string{"exit 7", "touch second-ran"}

	err := Run(context.Background(), hooks, invocation, os.Stderr)
	if err == nil || !strings.Contains(err.Error(), "wt.afteradd") {
		t.Fatalf("Run() error = %v, want wt.afteradd failure", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "second-ran")); !os.IsNotExist(err) {
		t.Fatalf("second hook ran after failure: %v", err)
	}
}

func TestRunDebugEchoesWithoutExecuting(t *testing.T) {
	t.Setenv("DEBUG", "1")
	dir := t.TempDir()
	invocation := Invocation{Event: BeforeRemove, Dir: dir}
	var output bytes.Buffer

	if err := Run(context.Background(), []string{"touch should-not-run"}, invocation, &output); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !strings.Contains(output.String(), "wt.beforeremove") {
		t.Fatalf("debug output = %q, want event name", output.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "should-not-run")); !os.IsNotExist(err) {
		t.Fatalf("debug hook executed: %v", err)
	}
}

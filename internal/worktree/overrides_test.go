package worktree

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultRemoteHonorsConfiguredDestinations(t *testing.T) {
	dir := t.TempDir()
	initTestRepo(t, dir)
	for _, remote := range []string{"origin", "other"} {
		if out, err := exec.Command("git", "-C", dir, "remote", "add", remote, filepath.Join(dir, remote)).CombinedOutput(); err != nil {
			t.Fatalf("add remote: %s: %v", out, err)
		}
	}
	for _, configured := range []string{".", filepath.Join(dir, "custom-remote"), "other"} {
		t.Setenv("GIT_CONFIG_COUNT", "1")
		t.Setenv("GIT_CONFIG_KEY_0", "branch.main.remote")
		t.Setenv("GIT_CONFIG_VALUE_0", configured)
		if got := DefaultRemoteIn(dir); got != configured {
			t.Fatalf("remote = %q, want %q", got, configured)
		}
	}
}

func TestDefaultBranchUsesConfiguredTimeout(t *testing.T) {
	dir := t.TempDir()
	initTestRepo(t, dir)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "wt.remoteTimeout")
	t.Setenv("GIT_CONFIG_VALUE_0", "1ns")
	if got := DefaultBranchIn(dir, dir); got != "" {
		t.Fatalf("expired lookup returned %q", got)
	}
	t.Setenv("GIT_CONFIG_VALUE_0", "0")
	if got := DefaultBranchIn(dir, dir); got != "main" {
		t.Fatalf("unlimited lookup returned %q", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := DefaultBranchInContext(ctx, dir, dir); got != "" {
		t.Fatalf("disabled timeout ignored parent cancellation: %q", got)
	}
}

func TestRemoteTimeoutConfiguration(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  time.Duration
	}{
		{"30s", 30 * time.Second},
		{"2m", 2 * time.Minute},
		{"50ms", 50 * time.Millisecond},
		{"0", 0},
		{"-1s", 10 * time.Second},
		{"invalid", 10 * time.Second},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("GIT_CONFIG_COUNT", "1")
			t.Setenv("GIT_CONFIG_KEY_0", "wt.remoteTimeout")
			t.Setenv("GIT_CONFIG_VALUE_0", tc.value)
			if got := remoteTimeout(context.Background(), t.TempDir()); got != tc.want {
				t.Fatalf("timeout = %v, want %v", got, tc.want)
			}
		})
	}
	t.Setenv("GIT_CONFIG_COUNT", "0")
	if got := remoteTimeout(context.Background(), t.TempDir()); got != 10*time.Second {
		t.Fatalf("default timeout = %v", got)
	}
}

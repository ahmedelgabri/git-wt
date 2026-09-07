package cmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanupBasePreservesDiscoveryCancellation(t *testing.T) {
	t.Chdir(initGitRepo(t))
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(realGit, "config", "branch.main.remote", "file:///unavailable").CombinedOutput(); err != nil {
		t.Fatalf("configure remote: %v: %s", err, out)
	}
	bin := t.TempDir()
	marker := filepath.Join(bin, "discovery-started")
	script := "#!/bin/sh\nif [ \"$1\" = ls-remote ]; then\n: > " + shellQuote(marker) + "\nexec sleep 30\nfi\nexec " + shellQuote(realGit) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := resolveCleanupBase(ctx)
		done <- err
	}()
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("discovery returned before cancellation: %v", err)
		case <-ctx.Done():
			t.Fatal("discovery did not start")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestCleanupBaseBranchRefs(t *testing.T) {
	t.Chdir(initGitRepo(t))
	for _, args := range [][]string{
		{"remote", "add", "origin", "/unavailable"},
		{"remote", "add", "company/origin", "/unavailable"},
		{"update-ref", "refs/remotes/origin/main", "refs/heads/main"},
		{"update-ref", "refs/remotes/origin/release/main", "refs/heads/main"},
		{"update-ref", "refs/remotes/company/origin/main", "refs/heads/main"},
		{"update-ref", "refs/remotes/archive/main", "refs/heads/main"},
		{"update-ref", "refs/heads/origin/main", "refs/heads/main"},
		{"update-ref", "refs/remotes/bare-name", "refs/heads/main"},
		{"tag", "tag", "refs/heads/main"},
		{"symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main"},
		{"symbolic-ref", "refs/remotes/origin/tag-alias", "refs/tags/tag"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	for _, tc := range []struct {
		value  string
		ref    string
		branch string
	}{
		{"refs/remotes/origin/main", "refs/remotes/origin/main", "main"},
		{"refs/remotes/origin/HEAD", "refs/remotes/origin/main", "main"},
		{"refs/remotes/origin/release/main", "refs/remotes/origin/release/main", "release/main"},
		{"refs/remotes/company/origin/main", "refs/remotes/company/origin/main", "main"},
		{"refs/remotes/archive/main", "refs/remotes/archive/main", "main"},
		{"origin/main", "refs/heads/origin/main", "origin/main"},
		{"refs/tags/tag", "", ""},
		{"refs/remotes/origin/tag-alias", "", ""},
		{"refs/remotes/origin/missing", "", ""},
		{"refs/remotes/origin/main~1", "", ""},
		{"refs/remotes/bare-name", "", ""},
	} {
		t.Run(tc.value, func(t *testing.T) {
			if out, err := exec.Command("git", "config", "wt.cleanupBase", tc.value).CombinedOutput(); err != nil {
				t.Fatalf("configure base: %v: %s", err, out)
			}
			base, err := resolveCleanupBase(context.Background())
			if tc.ref == "" {
				if err == nil {
					t.Fatalf("expected invalid base refusal, got %+v", base)
				}
				return
			}
			if err != nil || base.ref != tc.ref || base.protectedBranch != tc.branch {
				t.Fatalf("base = %+v, %v; want %s protecting %s", base, err, tc.ref, tc.branch)
			}
		})
	}
}

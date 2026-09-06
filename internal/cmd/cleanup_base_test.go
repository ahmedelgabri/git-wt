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

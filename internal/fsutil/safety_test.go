package fsutil

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyReplacesSymlinkWithoutFollowingIt(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	external := filepath.Join(t.TempDir(), "external")
	if err := os.WriteFile(external, []byte("external"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(dst, "file")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "file"), []byte("replacement"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := CopyDir(src, dst, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(external)
	if err != nil || string(data) != "external" {
		t.Fatalf("external modified: %q, %v", data, err)
	}
	state, err := Snapshot(context.Background(), src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySnapshot(context.Background(), dst, nil, state); err != nil {
		t.Fatal(err)
	}
}

func TestCopyRootExclusionKeepsNestedGit(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	for _, path := range []string{".git", "nested/.git"} {
		if err := os.MkdirAll(filepath.Join(src, path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(src, path, "config"), []byte("config"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := CopyDir(src, dst, []string{".git"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "nested/.git/config")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
		t.Fatalf("root .git copied: %v", err)
	}
}

func TestCopyCanceledDoesNotWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	src, dst := t.TempDir(), t.TempDir()
	if err := CopyDirContext(ctx, src, dst, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestCopyRejectsUnexpectedDestinationDirectory(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "file"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dst, "file"), 0o755); err != nil {
		t.Fatal(err)
	}
	valuable := filepath.Join(dst, "file", "valuable")
	if err := os.WriteFile(valuable, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyDir(src, dst, nil); err == nil {
		t.Fatal("expected refusal to delete nonempty directory")
	}
	if _, err := os.Stat(valuable); err != nil {
		t.Fatal(err)
	}
}

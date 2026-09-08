//go:build darwin || linux

package fsutil

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ahmedelgabri/git-wt/internal/testutil"
	"golang.org/x/sys/unix"
)

func TestCopyAndVerifyExtendedAttributes(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	testutil.RequireXattrs(t, src)
	if err := os.WriteFile(filepath.Join(src, "file"), []byte("contents"), 0o640); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		path, name string
		value      []byte
	}{
		{src, "user.root", []byte("directory metadata")},
		{filepath.Join(src, "file"), "user.binary", []byte("a\x00b\n")},
		{filepath.Join(src, "file"), "user.empty", nil},
		{dst, "user.extra", []byte("not in source")},
	} {
		if err := unix.Lsetxattr(item.path, item.name, item.value, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(filepath.Join(src, "file"), 0o444); err != nil {
		t.Fatal(err)
	}
	state, err := Snapshot(context.Background(), src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := CopyDir(src, dst, nil); err != nil {
		t.Fatal(err)
	}
	if err := VerifySnapshot(context.Background(), dst, nil, state); err != nil {
		t.Fatal(err)
	}
	value := make([]byte, 16)
	n, err := unix.Lgetxattr(filepath.Join(dst, "file"), "user.binary", value)
	if err != nil || !bytes.Equal(value[:n], []byte("a\x00b\n")) {
		t.Fatalf("copied value = %q, %v", value, err)
	}
	if err := unix.Lsetxattr(dst, "user.root", []byte("changed"), 0); err != nil {
		t.Fatal(err)
	}
	if err := VerifySnapshot(context.Background(), dst, nil, state); err == nil {
		t.Fatal("verification accepted changed root directory metadata")
	}
}

func TestCopySymlinkMetadataDoesNotFollowTarget(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	testutil.RequireXattrs(t, src)
	target := filepath.Join(t.TempDir(), "external")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := unix.Lsetxattr(target, "user.external", []byte("unchanged"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}
	if err := CopyDir(src, dst, nil); err != nil {
		t.Fatal(err)
	}
	value := make([]byte, 16)
	n, err := unix.Lgetxattr(target, "user.external", value)
	if err != nil || string(value[:n]) != "unchanged" {
		t.Fatalf("external target changed: %q, %v", value, err)
	}
}

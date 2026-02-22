package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create source structure
	os.MkdirAll(filepath.Join(src, "subdir"), 0o755)
	os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(src, "subdir", "nested.txt"), []byte("world"), 0o644)

	if err := CopyDir(src, dst, nil); err != nil {
		t.Fatalf("CopyDir error: %v", err)
	}

	// Verify files were copied
	data, err := os.ReadFile(filepath.Join(dst, "file.txt"))
	if err != nil {
		t.Fatalf("read file.txt: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("file.txt = %q, want %q", data, "hello")
	}

	data, err = os.ReadFile(filepath.Join(dst, "subdir", "nested.txt"))
	if err != nil {
		t.Fatalf("read nested.txt: %v", err)
	}
	if string(data) != "world" {
		t.Errorf("nested.txt = %q, want %q", data, "world")
	}
}

func TestCopyDirExcludes(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.MkdirAll(filepath.Join(src, ".git", "objects"), 0o755)
	os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(src, ".git", "config"), []byte("config"), 0o644)

	if err := CopyDir(src, dst, []string{".git"}); err != nil {
		t.Fatalf("CopyDir error: %v", err)
	}

	// file.txt should exist
	if _, err := os.Stat(filepath.Join(dst, "file.txt")); err != nil {
		t.Error("file.txt should exist in dst")
	}

	// .git should NOT exist
	if _, err := os.Stat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
		t.Error(".git should not exist in dst")
	}
}

func TestCopyDirPreservesPermissions(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.WriteFile(filepath.Join(src, "script.sh"), []byte("#!/bin/bash"), 0o755)

	if err := CopyDir(src, dst, nil); err != nil {
		t.Fatalf("CopyDir error: %v", err)
	}

	info, err := os.Stat(filepath.Join(dst, "script.sh"))
	if err != nil {
		t.Fatalf("stat script.sh: %v", err)
	}

	if info.Mode().Perm() != 0o755 {
		t.Errorf("permissions = %o, want 755", info.Mode().Perm())
	}
}

func TestCopyDirSymlinks(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.WriteFile(filepath.Join(src, "target.txt"), []byte("target"), 0o644)
	os.Symlink("target.txt", filepath.Join(src, "link.txt"))

	if err := CopyDir(src, dst, nil); err != nil {
		t.Fatalf("CopyDir error: %v", err)
	}

	link, err := os.Readlink(filepath.Join(dst, "link.txt"))
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if link != "target.txt" {
		t.Errorf("symlink target = %q, want %q", link, "target.txt")
	}
}

func TestCopyDirExcludesFile(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.WriteFile(filepath.Join(src, "keep.txt"), []byte("keep"), 0o644)
	os.WriteFile(filepath.Join(src, "skip.log"), []byte("skip"), 0o644)

	if err := CopyDir(src, dst, []string{"skip.log"}); err != nil {
		t.Fatalf("CopyDir error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dst, "keep.txt")); err != nil {
		t.Error("keep.txt should exist in dst")
	}
	if _, err := os.Stat(filepath.Join(dst, "skip.log")); !os.IsNotExist(err) {
		t.Error("skip.log should not exist in dst")
	}
}

func TestCopyDirNonExistentSource(t *testing.T) {
	dst := t.TempDir()
	err := CopyDir(filepath.Join(t.TempDir(), "nonexistent"), dst, nil)
	if err == nil {
		t.Error("CopyDir with nonexistent source should return error")
	}
}

func TestCopyDirEmptySource(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	if err := CopyDir(src, dst, nil); err != nil {
		t.Fatalf("CopyDir error: %v", err)
	}

	entries, err := os.ReadDir(dst)
	if err != nil {
		t.Fatalf("ReadDir error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("dst should be empty, got %d entries", len(entries))
	}
}

func TestCopyDirDeepNesting(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	deep := filepath.Join(src, "a", "b", "c")
	os.MkdirAll(deep, 0o755)
	os.WriteFile(filepath.Join(deep, "file.txt"), []byte("deep"), 0o644)

	if err := CopyDir(src, dst, nil); err != nil {
		t.Fatalf("CopyDir error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dst, "a", "b", "c", "file.txt"))
	if err != nil {
		t.Fatalf("read deep file: %v", err)
	}
	if string(data) != "deep" {
		t.Errorf("deep file = %q, want %q", data, "deep")
	}
}

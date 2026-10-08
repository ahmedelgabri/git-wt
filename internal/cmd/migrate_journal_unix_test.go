//go:build unix

package cmd

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestMigrationRollbackKeepsSavedFileModes(t *testing.T) {
	root := migrationFixture(t)
	config := filepath.Join(root, ".git", "config")
	if err := os.Chmod(config, 0o664); err != nil {
		t.Fatal(err)
	}
	want := snapshotTree(t, root)
	// A restrictive umask must not narrow restored permissions, which can
	// break group access to shared repositories.
	defer syscall.Umask(syscall.Umask(0o077))
	journal, _, err := convertWithFailure(t, root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.rollback(); err != nil {
		t.Fatal(err)
	}
	requireSameTree(t, want, snapshotTree(t, root))
}

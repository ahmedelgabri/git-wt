//go:build darwin || linux

package cmd

import (
	"context"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestMigrationDetectsSourceGitAttributeChanges(t *testing.T) {
	root := initGitRepo(t)
	plan, err := buildMigratePlan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	stage := t.TempDir()
	if err := buildMigratedStructure(context.Background(), plan, stage); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".git", "config")
	if err := unix.Lsetxattr(path, "user.changed", []byte("concurrent metadata"), 0); err != nil {
		t.Fatal(err)
	}
	if err := verifyMigration(context.Background(), plan, stage); err == nil {
		t.Fatal("source Git metadata change should abort migration")
	}
	data := make([]byte, 64)
	n, err := unix.Lgetxattr(path, "user.changed", data)
	if err != nil || string(data[:n]) != "concurrent metadata" {
		t.Fatalf("source metadata was not retained: %v", err)
	}
}

func TestMigrationRejectsLostXattrsBeforeAndAfterPromotion(t *testing.T) {
	for _, location := range []struct{ source, dest string }{
		{"README.md", "main/README.md"},
		{".git/config", ".bare/config"},
		{".git/objects", ".bare/objects"},
	} {
		for _, promoted := range []bool{false, true} {
			phase := "preparation"
			if promoted {
				phase = "promotion"
			}
			t.Run(location.source+"/"+phase, func(t *testing.T) {
				testMigrationMetadataDamage(t, promoted, func(root string) error {
					return unix.Lsetxattr(filepath.Join(root, location.source), "user.migration-test", []byte("keep me"), 0)
				}, func(dest string) error {
					return unix.Lremovexattr(filepath.Join(dest, location.dest), "user.migration-test")
				})
			})
		}
	}
}

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreBackupRetainsFailedEntries(t *testing.T) {
	root, backup := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(backup, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(backup, ".git", "original")
	if err := os.WriteFile(marker, []byte("database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: ./.bare\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := restoreBackup(backup, root); err == nil {
		t.Fatal("expected restore failure")
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "database" {
		t.Fatalf("original lost: %q, %v", data, err)
	}
}

func TestMigrationRollbackAtEveryRename(t *testing.T) {
	// Two original entries and three promoted entries. Fail each boundary once
	// and require that rollback restores the original without discarding data.
	for failAt := 1; failAt <= 5; failAt++ {
		t.Run(fmt.Sprint(failAt), func(t *testing.T) {
			root, stage, backup := t.TempDir(), t.TempDir(), t.TempDir()
			mustWrite := func(path, content string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
				t.Fatal(err)
			}
			mustWrite(filepath.Join(root, ".git", "database"), "original database")
			mustWrite(filepath.Join(root, "file"), "original work")
			for _, name := range []string{".bare", "main"} {
				if err := os.Mkdir(filepath.Join(stage, name), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			mustWrite(filepath.Join(stage, ".git"), "gitdir: ./.bare\n")
			calls := 0
			moves := migrationMoves{rename: func(src, dst string) error {
				calls++
				if calls == failAt {
					return errors.New("injected rename failure")
				}
				return renameEntry(src, dst)
			}}
			err := finalizeMigrationUsing(root, stage, backup, []string{".bare", ".git", "main"}, moves, func() error { return nil })
			if err == nil {
				t.Fatal("expected injected failure")
			}
			for path, want := range map[string]string{".git/database": "original database", "file": "original work"} {
				got, err := os.ReadFile(filepath.Join(root, path))
				if err != nil || string(got) != want {
					t.Fatalf("%s: %q, %v", path, got, err)
				}
			}
		})
	}
}

func TestMigrationRepositoryVerificationRollsBack(t *testing.T) {
	root, stage, backup := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "valuable"), []byte("database"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".bare", "main"} {
		if err := os.Mkdir(filepath.Join(stage, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(stage, ".git"), []byte("gitdir: ./.bare\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := finalizeMigrationUsing(root, stage, backup, []string{".bare", ".git", "main"}, migrationMoves{rename: renameEntry}, func() error { return errors.New("invalid migrated index") })
	if err == nil {
		t.Fatal("expected verification failure")
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "valuable")); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationDetectsSourceChanges(t *testing.T) {
	root := initGitRepo(t)
	plan, err := buildMigratePlan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	stage := t.TempDir()
	if err := buildMigratedStructure(context.Background(), plan, stage); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("concurrent edit"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyMigration(context.Background(), plan, stage); err == nil {
		t.Fatal("source edit should abort migration")
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
}

package cmd

import (
	"bytes"
	"context"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ahmedelgabri/git-wt/internal/fsutil"
	"github.com/ahmedelgabri/git-wt/internal/git"
)

func testMigrationMetadataDamage(t *testing.T, promoted bool, prepare, damage func(string) error) {
	t.Helper()
	root := initGitRepo(t)
	if prepare != nil {
		if err := prepare(root); err != nil {
			t.Fatal(err)
		}
	}
	originalLog, err := os.ReadFile(filepath.Join(root, ".git", "logs", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := buildMigratePlan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	stage := t.TempDir()
	if err := buildMigratedStructure(context.Background(), plan, stage); err != nil {
		t.Fatal(err)
	}
	check := func(dest string) error {
		if err := damage(dest); err != nil {
			t.Fatal(err)
		}
		return verifyMigrationState(context.Background(), plan, dest)
	}
	if promoted {
		err = finalizeMigration(root, stage, t.TempDir(), []string{".git", ".bare", "main"}, migrationMoves{rename: renameEntry}, func() error { return check(root) })
	} else {
		err = check(stage)
	}
	if err == nil || !strings.Contains(err.Error(), "verification") && !strings.Contains(err.Error(), "metadata") {
		t.Fatalf("expected metadata verification failure, got %v", err)
	}
	actual, err := os.ReadFile(filepath.Join(root, ".git", "logs", "HEAD"))
	if err != nil || !bytes.Equal(actual, originalLog) {
		t.Fatalf("original reflog was not retained or restored: %v", err)
	}
	if err := fsutil.VerifySnapshot(context.Background(), root, []string{".git"}, plan.files); err != nil {
		t.Fatal(err)
	}
	if err := fsutil.VerifySnapshot(context.Background(), filepath.Join(root, ".git"), []string{"objects"}, plan.gitFiles); err != nil {
		t.Fatal(err)
	}
	objects, err := fsutil.SnapshotMetadata(context.Background(), filepath.Join(root, ".git", "objects"), nil)
	if err != nil || !maps.Equal(objects, plan.objectFiles) {
		t.Fatalf("original object metadata was not retained: %v", err)
	}
}

func TestMigrationRejectsExistingControlMetadata(t *testing.T) {
	for _, name := range []string{"commondir", "gitdir"} {
		t.Run(name, func(t *testing.T) {
			root := initGitRepo(t)
			if err := os.WriteFile(filepath.Join(root, ".git", name), []byte("external directory\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := preflightMigrateRepo(root); err == nil || !strings.Contains(err.Error(), "control file "+name) {
				t.Fatalf("expected control-file refusal, got %v", err)
			}
		})
	}
}

func TestMigrationChecksPrivateReflogBeforeAndAfterPromotion(t *testing.T) {
	for _, promoted := range []bool{false, true} {
		name := "preparation"
		if promoted {
			name = "promotion"
		}
		t.Run(name, func(t *testing.T) {
			testMigrationMetadataDamage(t, promoted, nil, func(dest string) error {
				gitDir, err := git.QueryPathInContext(context.Background(), filepath.Join(dest, "main"), "rev-parse", "--absolute-git-dir")
				if err != nil {
					return err
				}
				// The unchanged .bare/logs/HEAD must not hide this corruption.
				return os.WriteFile(filepath.Join(gitDir, "logs", "HEAD"), nil, 0o644)
			})
		})
	}
}

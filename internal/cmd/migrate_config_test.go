package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ahmedelgabri/git-wt/internal/git"
)

func TestMigrationConfigAndRefIsolationBeforeAndAfterPromotion(t *testing.T) {
	for _, promoted := range []bool{false, true} {
		phase := "preparation"
		if promoted {
			phase = "promotion"
		}
		t.Run("configuration/"+phase, func(t *testing.T) {
			testMigrationMetadataDamage(t, promoted, func(root string) error {
				_, err := git.RunInWithOutputContext(context.Background(), root, "config", "custom.secret", "do-not-print-this")
				return err
			}, func(root string) error {
				_, err := git.RunInWithOutputContext(context.Background(), root, "config", "--unset-all", "custom.secret")
				return err
			})
		})
		t.Run("packed-private-ref/"+phase, func(t *testing.T) {
			testMigrationMetadataDamage(t, promoted, nil, func(root string) error {
				head, err := git.QueryInContext(context.Background(), root, "rev-parse", "HEAD")
				if err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(root, ".bare", "packed-refs"), []byte("# pack-refs with: sorted\n"+head+" refs/worktree/leaked\n"), 0o644)
			})
		})
	}
}

func TestMigrationConfigDetectsExternalChanges(t *testing.T) {
	ctx := context.Background()
	root := initGitRepo(t)
	included := filepath.Join(t.TempDir(), "external.config")
	if err := os.WriteFile(included, []byte("[custom]\nsecret = before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := git.RunInWithOutputContext(ctx, root, "config", "include.path", included); err != nil {
		t.Fatal(err)
	}
	plan, err := buildMigratePlan(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(included, []byte("[custom]\nsecret = changed-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err = verifyMigrationConfig(ctx, plan, root, true)
	if err == nil || !strings.Contains(err.Error(), "custom.secret") || strings.Contains(err.Error(), "changed-secret") {
		t.Fatalf("expected redacted configuration-change error, got %v", err)
	}
}

func TestMigrationConfigAllowsLayoutChanges(t *testing.T) {
	root := initGitRepo(t)
	plan, err := buildMigratePlan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	stage := t.TempDir()
	if err := buildMigratedStructure(context.Background(), plan, stage); err != nil {
		t.Fatal(err)
	}
	if err := verifyMigration(context.Background(), plan, stage); err != nil {
		t.Fatal(err)
	}
}

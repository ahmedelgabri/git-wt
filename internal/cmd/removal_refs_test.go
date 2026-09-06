package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ahmedelgabri/git-wt/internal/git"
)

func TestSupportsRemoteURLReset(t *testing.T) {
	for _, tc := range []struct {
		version string
		want    bool
	}{
		{"git version 2.39.5 (Apple Git-154)", false},
		{"git version 2.45.4", false},
		{"git version 2.46.0", true},
		{"git version 2.46.0.windows.1", true},
		{"git version 2.54.0", true},
		{"git version 3.0.0", true},
		{"git version 3.-1.0", false},
		{"git version unknown", false},
		{"not a Git version", false},
	} {
		t.Run(tc.version, func(t *testing.T) {
			if got := supportsRemoteURLReset(tc.version); got != tc.want {
				t.Fatalf("supports reset = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRemovalSafetyWithLargeRetainedRefList(t *testing.T) {
	root := initGitRepo(t)
	t.Chdir(root)
	head, err := git.Query("rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := git.Run("update-ref", "refs/heads/feature", head); err != nil {
		t.Fatal(err)
	}
	// A packed fixture is cheap to create, but its ref names alone exceed the
	// usual process argument limit. Tags preserve main, not the later commit.
	var packed strings.Builder
	packed.WriteString("# pack-refs with: sorted\n")
	for i := range 12000 {
		fmt.Fprintf(&packed, "%s refs/tags/release-%05d-%s\n", head, i, strings.Repeat("x", 180))
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "packed-refs"), []byte(packed.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	target := removalTarget{path: filepath.Join(root, "missing-worktree"), branch: "feature", upstreamRef: "refs/remotes/origin/feature"}
	if err := validateRemovalSafety(target, true, false); err != nil {
		t.Fatalf("retained commit rejected: %v", err)
	}
	unique, err := git.RunWithOutput("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit-tree", "HEAD^{tree}", "-p", head, "-m", "unique")
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"refs/heads/feature", target.upstreamRef} {
		if err := git.Run("update-ref", ref, unique); err != nil {
			t.Fatal(err)
		}
	}
	if err := git.Run("symbolic-ref", "refs/remotes/origin/HEAD", target.upstreamRef); err != nil {
		t.Fatal(err)
	}
	if err := validateRemovalSafety(target, false, false); err != nil {
		t.Fatalf("retained upstream rejected: %v", err)
	}
	if err := validateRemovalSafety(target, true, false); err == nil || !strings.Contains(err.Error(), unique) {
		t.Fatalf("symbolic alias must not preserve a deleted upstream: %v", err)
	}
}

func TestRemoteDeletionRejectsRewritingAnAlreadyResolvedURL(t *testing.T) {
	root := initGitRepo(t)
	t.Chdir(root)
	for _, args := range [][]string{
		{"remote", "add", "origin", root + "/fetch"},
		{"config", "remote.origin.pushurl", "alias:repo"},
		{"config", "url." + root + "/intended/.insteadOf", "alias:"},
		{"config", "url." + root + "/wrong/.insteadOf", root + "/intended/"},
	} {
		if err := git.Run(args...); err != nil {
			t.Fatal(err)
		}
	}
	_, err := planRemoteDeletions(removalTarget{remote: "origin", remoteBranch: "feature"}, "HEAD", false)
	if err == nil || !strings.Contains(err.Error(), "URL rewrites change the push destination") {
		t.Fatalf("expected refusal before accessing the wrong destination: %v", err)
	}
}

func TestDeleteLocalBranchCleansOnlyLocalConfiguration(t *testing.T) {
	t.Chdir(initGitRepo(t))
	for _, args := range [][]string{{"branch", "feature"}, {"config", "branch.feature.rebase", "true"}} {
		if err := git.Run(args...); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "branch.feature.rebase")
	t.Setenv("GIT_CONFIG_VALUE_0", "false")
	head, err := git.Query("rev-parse", "feature")
	if err != nil {
		t.Fatal(err)
	}
	if err := deleteLocalBranch("feature", head); err != nil {
		t.Fatal(err)
	}
	if err := git.Run("branch", "feature"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Query("config", "--local", "--get", "branch.feature.rebase"); err == nil {
		t.Fatal("recreated branch inherited stale local configuration")
	}
	if got, err := git.Query("config", "--get", "branch.feature.rebase"); err != nil || got != "false" {
		t.Fatalf("inherited configuration changed: %q, %v", got, err)
	}
}

func TestDeleteLocalBranchRetainsConfigurationOnFailedLease(t *testing.T) {
	t.Chdir(initGitRepo(t))
	if err := git.Run("branch", "feature"); err != nil {
		t.Fatal(err)
	}
	if err := git.Run("config", "branch.feature.rebase", "true"); err != nil {
		t.Fatal(err)
	}
	head, err := git.Query("rev-parse", "feature")
	if err != nil {
		t.Fatal(err)
	}
	if err := deleteLocalBranch("feature", strings.Repeat("1", len(head))); err == nil {
		t.Fatal("expected failed ref comparison")
	}
	if got, err := git.Query("rev-parse", "feature"); err != nil || got != head {
		t.Fatalf("branch changed: %q, %v", got, err)
	}
	if got, err := git.Query("config", "--get", "branch.feature.rebase"); err != nil || got != "true" {
		t.Fatalf("configuration changed: %q, %v", got, err)
	}
}

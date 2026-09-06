package cmd

import (
	"strings"
	"testing"

	"github.com/ahmedelgabri/git-wt/internal/git"
)

func TestRemoteDeletionRejectsRewritingAnAlreadyResolvedURL(t *testing.T) {
	root := initGitRepo(t)
	t.Chdir(root)
	for _, args := range [][]string{
		{"remote", "add", "origin", "alias:repo"},
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

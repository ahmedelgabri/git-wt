package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ahmedelgabri/git-wt/internal/worktree"
)

func TestIsKnownCommand(t *testing.T) {
	known := []string{"add", "clone", "help", "--help", "-h"}
	for _, name := range known {
		if !isKnownCommand(name) {
			t.Errorf("isKnownCommand(%q) = false, want true", name)
		}
	}

	unknown := []string{"nonexistent", ""}
	for _, name := range unknown {
		if isKnownCommand(name) {
			t.Errorf("isKnownCommand(%q) = true, want false", name)
		}
	}

	// Test alias
	if !isKnownCommand("rm") {
		t.Error("isKnownCommand(rm) = false, want true (alias for remove)")
	}
}

func TestEntriesToPickerItems(t *testing.T) {
	entries := []worktree.Entry{
		{Path: "/tmp/project/main", Branch: "main", Head: "abc1234"},
		{Path: "/tmp/project/detached-wt", Detached: true, Head: "def5678"},
		{Path: "/tmp/project/no-branch", Branch: "", Head: "111aaaa"},
	}

	items := entriesToPickerItems(entries)

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Normal branch: label should contain branch in brackets
	if items[0].Value != "/tmp/project/main" {
		t.Errorf("item[0].Value = %q, want /tmp/project/main", items[0].Value)
	}
	if items[0].Label == "" {
		t.Error("item[0].Label should not be empty")
	}

	// Detached HEAD: label should contain "detached HEAD"
	if items[1].Value != "/tmp/project/detached-wt" {
		t.Errorf("item[1].Value = %q, want /tmp/project/detached-wt", items[1].Value)
	}

	// Empty branch
	if items[2].Value != "/tmp/project/no-branch" {
		t.Errorf("item[2].Value = %q, want /tmp/project/no-branch", items[2].Value)
	}

	// All items should have a Desc
	for i, item := range items {
		if item.Desc == "" {
			t.Errorf("item[%d].Desc should not be empty", i)
		}
	}
}

func TestPreviewWorktreeCmdStr(t *testing.T) {
	got := previewWorktreeCmdStr(previewModeRemove)
	if !strings.Contains(got, "sh -c") || !strings.Contains(got, `_preview worktree "$2" "$3"`) {
		t.Errorf("previewWorktreeCmdStr(remove) = %q, want sh -c positional args", got)
	}
	if !strings.Contains(got, "{1}") {
		t.Errorf("previewWorktreeCmdStr(remove) = %q, want to contain {1}", got)
	}

	got = previewWorktreeCmdStr(previewModeDeleteRemote)
	if !strings.Contains(got, shellQuote(previewModeDeleteRemote)) {
		t.Errorf("previewWorktreeCmdStr(remove-remote) = %q, want quoted remove-remote mode", got)
	}
}

func TestPreviewBranchCmdStr(t *testing.T) {
	got := previewBranchCmdStr()
	if !strings.Contains(got, "sh -c") || !strings.Contains(got, `_preview branch "$2"`) {
		t.Errorf("previewBranchCmdStr() = %q, want sh -c positional args", got)
	}
	if !strings.Contains(got, "{1}") {
		t.Errorf("previewBranchCmdStr() = %q, want to contain {1}", got)
	}
}

func TestPreviewGitColorArgRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if got := previewGitColorArg(); got != "--color=never" {
		t.Fatalf("previewGitColorArg() with NO_COLOR = %q, want %q", got, "--color=never")
	}

	t.Setenv("NO_COLOR", "")
	if got := previewGitColorArg(); got != "--color=always" {
		t.Fatalf("previewGitColorArg() without NO_COLOR = %q, want %q", got, "--color=always")
	}
}

func TestSplitRemoteBranchRef(t *testing.T) {
	remote, branch := splitRemoteBranchRef("origin/feature/nested")
	if remote != "origin" || branch != "feature/nested" {
		t.Fatalf("splitRemoteBranchRef() = (%q, %q), want (%q, %q)", remote, branch, "origin", "feature/nested")
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	// Create an initial commit so HEAD exists
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("init"), 0o644)
	run("add", "README.md")
	run("-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "-m", "init")
	return dir
}

func TestEntriesToPickerItemsWithBareRoot(t *testing.T) {
	// Set up a bare repo structure so entriesToPickerItems uses relative paths
	dir := t.TempDir()
	bareDir := filepath.Join(dir, ".bare")
	cmd := exec.Command("git", "init", "--bare", bareDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ./.bare\n"), 0o644)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	// Resolve symlinks for path comparison
	resolved, _ := filepath.EvalSymlinks(dir)

	entries := []worktree.Entry{
		{Path: filepath.Join(resolved, "main"), Branch: "main", Head: "abc1234"},
		{Path: filepath.Join(resolved, "feat", "login"), Branch: "feat/login", Head: "def5678"},
	}

	items := entriesToPickerItems(entries)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// With a bare root, labels should use relative paths, not just basename
	if !strings.Contains(items[0].Label, "main") {
		t.Errorf("item[0].Label = %q, want to contain 'main'", items[0].Label)
	}
	if !strings.Contains(items[1].Label, "feat/login") {
		t.Errorf("item[1].Label = %q, want to contain 'feat/login'", items[1].Label)
	}
}

func TestGenerateWorktreePreviewRemoveMode(t *testing.T) {
	// Set up a bare repo with a worktree so generateWorktreePreview can query it
	dir := t.TempDir()
	bareDir := filepath.Join(dir, ".bare")

	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	cmd := exec.Command("git", "init", "--bare", bareDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ./.bare\n"), 0o644)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	// Create a worktree
	wtPath := filepath.Join(dir, "main")
	run("worktree", "add", "-b", "main", wtPath)

	// Create a commit inside the worktree
	os.WriteFile(filepath.Join(wtPath, "file.txt"), []byte("hello"), 0o644)
	c := exec.Command("git", "add", "file.txt")
	c.Dir = wtPath
	c.CombinedOutput()
	c = exec.Command("git", "-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "-m", "init")
	c.Dir = wtPath
	c.CombinedOutput()

	out := generateWorktreePreview(wtPath, previewModeRemove)
	if !strings.Contains(out, "Worktree") {
		t.Errorf("preview should contain 'Worktree', got %q", out)
	}
	if !strings.Contains(out, "Status") {
		t.Errorf("preview should contain 'Status', got %q", out)
	}
	if !strings.Contains(out, "Recent Commits") {
		t.Errorf("preview should contain 'Recent Commits', got %q", out)
	}
	if strings.Contains(out, "Actions") {
		t.Error("remove mode should not contain 'Actions'")
	}
	if strings.Contains(out, "Delete remote branch") {
		t.Error("remove mode should not mention remote branch deletion")
	}
}

func TestGenerateWorktreePreviewDeleteRemoteMode(t *testing.T) {
	dir := t.TempDir()
	bareDir := filepath.Join(dir, ".bare")

	cmd := exec.Command("git", "init", "--bare", bareDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ./.bare\n"), 0o644)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	wtPath := filepath.Join(dir, "feat")
	c := exec.Command("git", "worktree", "add", "-b", "feat", wtPath)
	c.Dir = dir
	c.CombinedOutput()

	// A configured remote is not an upstream for this branch.
	if out, err := exec.Command("git", "--git-dir", bareDir, "remote", "add", "origin", "https://example.invalid/repo.git").CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}

	out := generateWorktreePreview(wtPath, previewModeDeleteRemote)
	if !strings.Contains(out, "Actions") {
		t.Errorf("remove-remote mode should contain 'Actions', got %q", out)
	}
	if !strings.Contains(out, "No remote upstream; remote branch deletion skipped") {
		t.Errorf("remove-remote mode should say the branch has no upstream, got %q", out)
	}
}

func TestGenerateWorktreePreviewDeleteRemoteModeDetached(t *testing.T) {
	repo := initGitRepo(t)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(repo)

	shaCmd := exec.Command("git", "rev-parse", "HEAD")
	shaCmd.Dir = repo
	shaBytes, err := shaCmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	sha := strings.TrimSpace(string(shaBytes))

	wtPath := filepath.Join(repo, "detached;$(preview)")
	c := exec.Command("git", "worktree", "add", "--detach", wtPath, sha)
	c.Dir = repo
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add --detach: %v\n%s", err, out)
	}

	out := generateWorktreePreview(wtPath, previewModeDeleteRemote)
	if !strings.Contains(out, "detached HEAD") {
		t.Errorf("detached preview should mention detached HEAD, got %q", out)
	}
	if strings.Contains(out, "Delete remote branch") {
		t.Errorf("detached preview should not include remote branch deletion, got %q", out)
	}
}

func TestShellQuoteHandlesShellMetacharacters(t *testing.T) {
	values := []string{
		`feature;$(echo shell)`,
		`feat;$(echo preview)`,
		`quote's-and-$dollars`,
	}

	for _, value := range values {
		cmd := exec.Command("sh", "-c", "printf '%s' "+shellQuote(value))
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("shellQuote(%q): %v", value, err)
		}
		if string(out) != value {
			t.Fatalf("shellQuote(%q) roundtrip = %q, want %q", value, out, value)
		}
	}
}

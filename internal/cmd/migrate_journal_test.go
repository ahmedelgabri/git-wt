package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=Test", "-c", "user.email=test@test.com"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// migrationFixture has state in every place migration touches: working files,
// a stash, pseudorefs, loose, packed, and shadowed private refs, and config.
func migrationFixture(t *testing.T) string {
	t.Helper()
	root := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("stashed"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "stash", "push", "-m", "saved")
	head := gitIn(t, root, "rev-parse", "HEAD")
	gitIn(t, root, "update-ref", "--create-reflog", "refs/worktree/packed", head)
	gitIn(t, root, "update-ref", "--create-reflog", "refs/bisect/shadowed", head)
	gitIn(t, root, "pack-refs", "--all")
	gitIn(t, root, "update-ref", "--create-reflog", "refs/worktree/loose", head)
	gitIn(t, root, "update-ref", "refs/bisect/shadowed", head)
	gitIn(t, root, "update-ref", "ORIG_HEAD", head)
	gitIn(t, root, "config", "custom.secret", "do-not-print-this")
	for name, content := range map[string]string{"README.md": "modified", "untracked.txt": "new", "dir/nested.txt": "nested"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// snapshotTree records every path with its mode and content, so rollback must
// restore the repository byte for byte.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		value := info.Mode().String()
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += " " + string(data)
		}
		snapshot[rel] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func requireSameTree(t *testing.T, want, got map[string]string) {
	t.Helper()
	for _, path := range slices.Sorted(maps.Keys(want)) {
		if got[path] != want[path] {
			t.Fatalf("%s changed: want %q, got %q", path, want[path], got[path])
		}
	}
	for _, path := range slices.Sorted(maps.Keys(got)) {
		if _, ok := want[path]; !ok {
			t.Fatalf("unexpected path %s", path)
		}
	}
}

// convertWithFailure runs a migration that fails before journal entry failAt.
// It returns the number of entries recorded.
func convertWithFailure(t *testing.T, root string, failAt int) (*migrationJournal, int, error) {
	t.Helper()
	plan, err := buildMigratePlan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	journal, err := startMigrationJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	records := 0
	journal.beforeRecord = func() error {
		records++
		if records == failAt {
			return errors.New("injected interruption")
		}
		return nil
	}
	err = convertRepository(context.Background(), journal, plan)
	return journal, records, err
}

func TestMigrationConvertsFixture(t *testing.T) {
	root := migrationFixture(t)
	journal, records, err := convertWithFailure(t, root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.finish(); err != nil {
		t.Fatal(err)
	}
	if records < 10 {
		t.Fatalf("expected the fixture to exercise most steps, got %d journal entries", records)
	}
	wt := filepath.Join(root, "main")
	for _, ref := range []string{"refs/worktree/packed", "refs/worktree/loose", "refs/bisect/shadowed", "ORIG_HEAD"} {
		gitIn(t, wt, "rev-parse", "--verify", ref)
	}
	if out := gitIn(t, root, "for-each-ref", "refs/worktree/", "refs/bisect/"); out != "" {
		t.Fatalf("private refs leaked into the common directory: %s", out)
	}
	for _, stale := range []string{"index", "ORIG_HEAD", "logs/HEAD", "refs/worktree"} {
		if exists, _ := pathExists(filepath.Join(root, ".bare", stale)); exists {
			t.Fatalf("%s left in the common directory", stale)
		}
	}
	if exists, _ := pathExists(filepath.Join(root, migrationStateDir)); exists {
		t.Fatal("journal not removed after success")
	}
	if path, _ := migrationIDPath(journal.id); path == "" {
		t.Fatal("journal has no ID")
	} else if exists, _ := pathExists(path); exists {
		t.Fatal("journal ID not removed after success")
	}
}

func TestMigrationRollsBackAtEveryStep(t *testing.T) {
	journal, total, err := convertWithFailure(t, migrationFixture(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	_ = journal.finish()
	// total+1 never fails a step, like a migration that fails verification.
	for failAt := 1; failAt <= total+1; failAt++ {
		for _, crash := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/crash=%v", failAt, crash), func(t *testing.T) {
				root := migrationFixture(t)
				want := snapshotTree(t, root)
				journal, _, err := convertWithFailure(t, root, failAt)
				if (err == nil) != (failAt > total) {
					t.Fatalf("unexpected conversion result: %v", err)
				}
				if crash {
					// Simulate a killed process: nothing undone, lock released.
					journal.unlock()
					_ = journal.file.Close()
					_, err = recoverMigration(root)
				} else {
					err = journal.rollback()
				}
				if err != nil {
					t.Fatal(err)
				}
				requireSameTree(t, want, snapshotTree(t, root))
			})
		}
	}
}

func TestMigrationRefusesConcurrentRecovery(t *testing.T) {
	root := migrationFixture(t)
	journal, err := startMigrationJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	defer journal.rollback()
	if _, err := recoverMigration(root); err == nil || !strings.Contains(err.Error(), "another git wt migrate is running") {
		t.Fatalf("expected a running migration to block recovery, got %v", err)
	}
}

func TestMigrationJournalIgnoresTruncatedEntry(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, migrationStateDir), 0o700); err != nil {
		t.Fatal(err)
	}
	id := registerTestMigration(t, root)
	journal := "id " + id + "\nmove \"a\" \"b\"\nmove \"b\" \"c"
	if err := os.WriteFile(migrationJournalPath(root), []byte(journal), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "a"), filepath.Join(root, "b")); err != nil {
		t.Fatal(err)
	}
	if _, err := recoverMigration(root); err != nil {
		t.Fatal(err)
	}
	if exists, _ := pathExists(filepath.Join(root, "a")); !exists {
		t.Fatal("completed move was not undone")
	}
	if exists, _ := pathExists(filepath.Join(root, migrationStateDir)); exists {
		t.Fatal("journal not removed")
	}
}

func TestMigrationUndoRefusesConflicts(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "b"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	err := undoMigrationRecord(root, migrationRecord{op: "move", paths: []string{"a", "b"}})
	if err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("expected conflict, got %v", err)
	}
	// A directory that is not empty is never removed by a create entry.
	if err := os.WriteFile(filepath.Join(root, "a", "user-data"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := undoMigrationRecord(root, migrationRecord{op: "create", paths: []string{"a"}}); err == nil {
		t.Fatal("expected refusal to remove a non-empty directory")
	}
}

func TestMigrationVerificationDetectsDamage(t *testing.T) {
	for name, damage := range map[string]func(t *testing.T, root string){
		"configuration": func(t *testing.T, root string) {
			gitIn(t, root, "config", "--unset-all", "custom.secret")
		},
		"packed private ref": func(t *testing.T, root string) {
			head := gitIn(t, root, "rev-parse", "HEAD")
			data := "# pack-refs with: sorted\n" + head + " refs/worktree/leaked\n"
			if err := os.WriteFile(filepath.Join(root, ".bare", "packed-refs"), []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"working tree": func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "main", "untracked.txt")); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := migrationFixture(t)
			plan, err := buildMigratePlan(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			journal, err := startMigrationJournal(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := convertRepository(context.Background(), journal, plan); err != nil {
				t.Fatal(err)
			}
			damage(t, root)
			err = verifyMigrationState(context.Background(), plan)
			if err == nil || strings.Contains(err.Error(), "do-not-print-this") {
				t.Fatalf("expected redacted verification failure, got %v", err)
			}
			_ = journal.finish()
		})
	}
}

// convertFixture converts a fixture without finishing, as if verification
// failed, and returns the journal and the original tree.
func convertFixture(t *testing.T) (string, *migrationJournal, map[string]string) {
	t.Helper()
	root := migrationFixture(t)
	want := snapshotTree(t, root)
	journal, _, err := convertWithFailure(t, root, 0)
	if err != nil {
		t.Fatal(err)
	}
	return root, journal, want
}

func TestMigrationRollbackResumesAtEveryStep(t *testing.T) {
	_, journal, _ := convertFixture(t)
	state, err := readMigrationJournal(journal.root)
	if err != nil {
		t.Fatal(err)
	}
	_ = journal.rollback()
	// Stop before each step, and after each step before its progress is
	// recorded, as a crash would.
	for stopAt := 1; stopAt <= 2*len(state.records); stopAt++ {
		t.Run(fmt.Sprint(stopAt), func(t *testing.T) {
			root, journal, want := convertFixture(t)
			calls := 0
			migrationUndoHook = func(bool) error {
				calls++
				if calls == stopAt {
					return errors.New("injected rollback interruption")
				}
				return nil
			}
			err := journal.rollback()
			migrationUndoHook = nil
			if err == nil {
				t.Fatal("expected interrupted rollback")
			}
			if _, err := recoverMigration(root); err != nil {
				t.Fatal(err)
			}
			requireSameTree(t, want, snapshotTree(t, root))
		})
	}
}

func TestMigrationRollbackResumesAfterConflict(t *testing.T) {
	root, journal, want := convertFixture(t)
	// An editor recreates a moved file while the repository is migrated.
	conflict := filepath.Join(root, "README.md")
	if err := os.WriteFile(conflict, []byte("recreated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := journal.rollback(); err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("expected a conflict, got %v", err)
	}
	if err := os.Remove(conflict); err != nil {
		t.Fatal(err)
	}
	if _, err := recoverMigration(root); err != nil {
		t.Fatal(err)
	}
	requireSameTree(t, want, snapshotTree(t, root))
}

func TestMigrationRecoveryFinishesACommittedMigration(t *testing.T) {
	root, journal, _ := convertFixture(t)
	if err := journal.commit(); err != nil {
		t.Fatal(err)
	}
	// Simulate a crash part way through cleanup: saved files are gone.
	for _, dir := range []string{"work", "backup"} {
		if err := os.RemoveAll(filepath.Join(root, migrationStateDir, dir)); err != nil {
			t.Fatal(err)
		}
	}
	journal.unlock()
	_ = journal.file.Close()
	completed, err := recoverMigration(root)
	if err != nil || !completed {
		t.Fatalf("expected cleanup of a completed migration, got %v, %v", completed, err)
	}
	if exists, _ := pathExists(filepath.Join(root, migrationStateDir)); exists {
		t.Fatal("journal not removed")
	}
	if branch := gitIn(t, filepath.Join(root, "main"), "branch", "--show-current"); branch != "main" {
		t.Fatalf("migrated worktree damaged: on %q", branch)
	}
	gitIn(t, filepath.Join(root, "main"), "rev-parse", "--verify", "refs/worktree/packed")
}

// registerTestMigration records a migration for root as register does and
// returns its ID.
func registerTestMigration(t *testing.T, root string) string {
	t.Helper()
	id := strings.Repeat("ab", 16)
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	path, err := migrationIDPath(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(root), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	return id
}

func TestMigrationRecoveryRefusesForeignJournals(t *testing.T) {
	registered := strings.Repeat("ab", 16)
	for name, tc := range map[string]struct {
		journal  string
		register bool
		prepare  func(t *testing.T, root, outside string)
		want     string
	}{
		"no ID":        {journal: "create \"a\"\n", want: "was not written by git wt migrate"},
		"unregistered": {journal: "id " + strings.Repeat("cd", 16) + "\ncreate \"a\"\n", register: true, want: "was not written by git wt migrate"},
		"path as ID":   {journal: "id ../../../outside\ncreate \"a\"\n", want: "was not written by git wt migrate"},
		// A clone can track files inside .bare/ and .git-wt-migrate/, but not
		// the user's state directory.
		"marker in the working tree": {
			journal: "id " + registered + "\ncreate \"a\"\n",
			prepare: func(t *testing.T, root, _ string) {
				for _, dir := range []string{".bare", ".git"} {
					if err := os.WriteFile(filepath.Join(root, dir, "git-wt-migration"), []byte(registered), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			},
			want: "was not written by git wt migrate",
		},
		"another repository": {
			journal: "id " + registered + "\ncreate \"a\"\n",
			prepare: func(t *testing.T, _, outside string) {
				registerTestMigration(t, outside)
			},
			want: "was not written by git wt migrate",
		},
		"outside":     {journal: "id " + registered + "\ncreate-tree \"../outside\"\n", register: true, want: "corrupt migration journal"},
		"absolute":    {journal: "id " + registered + "\nmove \"a\" \"/tmp/a\"\n", register: true, want: "corrupt migration journal"},
		"other tree":  {journal: "id " + registered + "\ncreate-tree \".git\"\n", register: true, want: "corrupt migration journal"},
		"unknown op":  {journal: "id " + registered + "\nremove \"a\"\n", register: true, want: "corrupt migration journal"},
		"backup path": {journal: "id " + registered + "\nrestore \".git/config\" \"README.md\"\n", register: true, want: "corrupt migration journal"},
		"symlinked directory": {
			journal:  "id " + registered + "\ncreate \"link/keep\"\n",
			register: true,
			prepare: func(t *testing.T, root, outside string) {
				if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
					t.Fatal(err)
				}
			},
			want: "refusing to follow symlinked directory",
		},
	} {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			root, outside := filepath.Join(parent, "repo"), filepath.Join(parent, "outside")
			for _, dir := range []string{filepath.Join(root, migrationStateDir), filepath.Join(root, ".git"), filepath.Join(root, ".bare"), outside} {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(outside, "keep"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(migrationJournalPath(root), []byte(tc.journal), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.register {
				if got := registerTestMigration(t, root); got != registered {
					t.Fatal("unexpected test ID")
				}
			}
			if tc.prepare != nil {
				tc.prepare(t, root, outside)
			}
			if _, err := recoverMigration(root); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
			if exists, _ := pathExists(filepath.Join(outside, "keep")); !exists {
				t.Fatal("recovery deleted files outside the repository")
			}
		})
	}
}

//go:build unix

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/spf13/cobra"
)

func TestMigrateCommitWriteFailure(t *testing.T) {
	partials := []int64{0, 3}
	// The file-size limit also affects Go's cache log. A child without
	// -test.testlogfile keeps fault injection out of the parent test runner.
	const child = "GIT_WT_TEST_MIGRATE_COMMIT_FAILURE"
	if os.Getenv(child) != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestMigrateCommitWriteFailure$", "-test.v")
		cmd.Env = append(os.Environ(), child+"=1")
		cmd.Stdin = strings.NewReader(strings.Repeat("y\n", len(partials)))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("commit failure regression: %v\n%s", err, out)
		}
		return
	}
	for _, partial := range partials {
		t.Run(fmt.Sprintf("partial=%d", partial), func(t *testing.T) {
			root := initGitRepo(t)
			gitIn(t, root, "checkout", "-b", "feature")
			gitIn(t, root, "remote", "add", "origin", root)
			gitIn(t, root, "update-ref", "refs/remotes/origin/main", "main")
			gitIn(t, root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
			// The journal must exceed every Git file and the captured output,
			// so the file-size limit affects only the final journal append.
			for i := range 32 {
				name := fmt.Sprintf("untracked-%02d-%s", i, strings.Repeat("x", 64))
				if err := os.WriteFile(filepath.Join(root, name), []byte("keep\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want := snapshotTree(t, root)
			journal, _, err := convertWithFailure(t, root, 0)
			if err != nil {
				t.Fatal(err)
			}
			info, err := journal.file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if err := journal.rollback(); err != nil {
				t.Fatal(err)
			}
			requireSameTree(t, want, snapshotTree(t, root))

			t.Chdir(root)
			output, err := migrateWithFileLimit(t, uint64(info.Size()+partial))
			if !errors.Is(err, syscall.EFBIG) || !strings.Contains(err.Error(), "commit migration journal") {
				t.Fatalf("expected journal commit failure, got %v\n%s", err, output)
			}
			for _, forbidden := range []string{"Migration complete", "Creating worktree for main", "Open your worktree"} {
				if strings.Contains(output, forbidden) {
					t.Fatalf("commit failure continued with %q:\n%s", forbidden, output)
				}
			}
			if exists, err := pathExists(filepath.Join(root, "main")); err != nil || exists {
				t.Fatalf("default worktree created after commit failure: exists=%v, err=%v", exists, err)
			}
			// Rollback can also hit the limit while recording undo progress.
			// With the limit lifted, recovery must restore all original state.
			if exists, _ := pathExists(migrationJournalPath(root)); exists {
				if completed, err := recoverMigration(root); err != nil || completed {
					t.Fatalf("expected recovery of an uncommitted migration, got %v, %v", completed, err)
				}
			}
			requireSameTree(t, want, snapshotTree(t, root))
		})
	}
}

func migrateWithFileLimit(t *testing.T, limit uint64) (string, error) {
	t.Helper()
	output, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	stdout, stderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = output, output
	defer func() { os.Stdout, os.Stderr = stdout, stderr }()

	var original syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &original); err != nil {
		t.Fatal(err)
	}
	signal.Ignore(syscall.SIGXFSZ)
	defer signal.Reset(syscall.SIGXFSZ)
	defer func() {
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &original); err != nil {
			t.Fatal(err)
		}
	}()
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: limit, Max: original.Max}); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	migrationErr := runMigrate(cmd, nil)
	data, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data), migrationErr
}

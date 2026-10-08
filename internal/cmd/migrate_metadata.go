package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ahmedelgabri/git-wt/internal/git"
)

// worktreeStateFile reports top-level Git files that belong to one worktree:
// the index and its split-index parts, and all-caps pseudorefs and state files
// such as ORIG_HEAD, FETCH_HEAD, and BISECT_LOG. The common HEAD stays: a bare
// repository needs it, and the new worktree already has its own.
func worktreeStateFile(name string) bool {
	if name == "index" || strings.HasPrefix(name, "sharedindex.") {
		return true
	}
	if name == "HEAD" {
		return false
	}
	for _, r := range name {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return name != ""
}

// moveMigrationMetadata moves worktree-local Git metadata from the common
// directory into the worktree's private directory. Git decides ownership via
// --git-path. Unknown directories such as lfs/ stay common, where tools that
// share data between worktrees look for them.
func moveMigrationMetadata(ctx context.Context, journal *migrationJournal, plan migratePlan, private string) error {
	common := journal.path(".bare")
	wt := journal.path(filepath.FromSlash(plan.currentBranch))
	// Each candidate is classified by the path Git uses for it. Reflogs follow
	// their ref, so they are classified by the ref's name.
	type candidate struct{ rel, ref string }
	var candidates []candidate
	for _, dir := range []string{"", "logs"} {
		entries, err := os.ReadDir(filepath.Join(common, dir))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.Type().IsRegular() && (worktreeStateFile(name) || dir == "logs" && name == "HEAD") {
				candidates = append(candidates, candidate{path.Join(dir, name), name})
			}
		}
	}
	for _, dir := range []string{"refs", "logs/refs"} {
		entries, err := os.ReadDir(filepath.Join(common, filepath.FromSlash(dir)))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		for _, entry := range entries {
			candidates = append(candidates, candidate{dir + "/" + entry.Name(), "refs/" + entry.Name()})
		}
	}
	for _, c := range candidates {
		resolved, err := git.QueryPathInContext(ctx, wt, "rev-parse", "--path-format=absolute", "--git-path", c.ref)
		if err != nil {
			return err
		}
		if !pathWithin(private, resolved) {
			continue
		}
		dst := filepath.Join(private, filepath.FromSlash(c.rel))
		// The new worktree's own HEAD reflog is replaced by the original one.
		if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o777); err != nil {
			return err
		}
		relDst, err := filepath.Rel(journal.root, dst)
		if err != nil {
			return err
		}
		if err := journal.move(filepath.Join(".bare", filepath.FromSlash(c.rel)), relDst); err != nil {
			return err
		}
	}
	return isolateMigrationPrivateRefs(ctx, journal, plan, private)
}

type migrationRef struct {
	name, object, target string
}

var migrationPrivateNamespaces = []string{"worktree", "bisect", "rewritten"}

// migrationGitCreatedDirs are the common directories Git may create while
// deleting packed private refs, once migration has moved the originals away.
func migrationGitCreatedDirs() []string {
	var dirs []string
	for _, namespace := range migrationPrivateNamespaces {
		dirs = append(dirs, filepath.Join(".bare", "refs", namespace), filepath.Join(".bare", "logs", "refs", namespace))
	}
	return dirs
}

func migrationPrivateRef(name string) bool {
	return strings.HasPrefix(name, "refs/worktree/") || strings.HasPrefix(name, "refs/bisect/") || strings.HasPrefix(name, "refs/rewritten/")
}

func migrationPrivateRefs(refs string) []migrationRef {
	var result []migrationRef
	for line := range strings.SplitSeq(refs, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !migrationPrivateRef(fields[0]) {
			continue
		}
		ref := migrationRef{name: fields[0], object: fields[1]}
		if len(fields) == 3 {
			ref.target = fields[2]
		}
		result = append(result, ref)
	}
	return result
}

// isolateMigrationPrivateRefs finishes moving private refs. Loose refs moved
// as files with their reflogs; packed refs cannot, so they are written as
// loose refs in the worktree and deleted from common packed-refs through Git.
// Everything this creates is journaled: undo removes it before moving the
// original namespaces back.
func isolateMigrationPrivateRefs(ctx context.Context, journal *migrationJournal, plan migratePlan, private string) error {
	for _, ref := range migrationPrivateRefs(plan.refs) {
		loose := filepath.Join(private, filepath.FromSlash(ref.name))
		if exists, err := pathExists(loose); err != nil {
			return err
		} else if exists {
			continue
		}
		rel, err := filepath.Rel(journal.root, loose)
		if err != nil {
			return err
		}
		if err := journal.mkdirAll(filepath.Dir(rel)); err != nil {
			return err
		}
		if err := journal.create(rel); err != nil {
			return err
		}
		// Writing the file directly leaves the moved reflog untouched.
		content := ref.object + "\n"
		if ref.target != "" {
			content = "ref: " + ref.target + "\n"
		}
		if err := os.WriteFile(loose, []byte(content), 0o666); err != nil {
			return err
		}
	}
	// update-ref -d recreates directories for the namespaces moved away.
	var created []string
	for _, dir := range migrationGitCreatedDirs() {
		if exists, err := pathExists(journal.path(dir)); err != nil {
			return err
		} else if !exists {
			if err := journal.createTree(dir); err != nil {
				return err
			}
			created = append(created, dir)
		}
	}
	// Whatever the common directory still resolves is a packed copy, possibly
	// one that a loose ref used to shadow.
	leaked, err := git.QueryInContext(ctx, plan.repoRoot, "for-each-ref", "--format=%(refname)", "refs/worktree/", "refs/bisect/", "refs/rewritten/")
	if err != nil {
		return err
	}
	for _, name := range strings.Fields(leaked) {
		if _, err := git.RunInWithOutputContext(ctx, plan.repoRoot, "-c", "core.hooksPath=/dev/null", "update-ref", "--no-deref", "-d", name); err != nil {
			return err
		}
	}
	for _, dir := range created {
		if err := removeEmptyDirs(journal.path(dir)); err != nil {
			return err
		}
	}
	return verifyMigrationPrivateRefIsolation(ctx, plan.repoRoot)
}

// removeEmptyDirs removes dir and its subdirectories if none of them hold a
// file. Anything else stays for verification to report.
func removeEmptyDirs(dir string) error {
	var dirs []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipAll
			}
			return err
		}
		if entry.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Remove(dirs[i]); err != nil && !os.IsNotExist(err) && !errors.Is(err, syscall.ENOTEMPTY) && !errors.Is(err, syscall.EEXIST) {
			return err
		}
	}
	return nil
}

func verifyMigrationPrivateRefIsolation(ctx context.Context, root string) error {
	out, err := git.QueryInContext(ctx, root, "for-each-ref", "--format=%(refname)", "refs/worktree/", "refs/bisect/", "refs/rewritten/")
	if err != nil {
		return err
	}
	if out != "" {
		return fmt.Errorf("migration metadata verification failed: worktree-local refs remain in the common database")
	}
	// A loose ref can hide a packed entry from for-each-ref. Check storage as
	// well so a hidden packed copy cannot become visible in another worktree.
	packed, err := os.ReadFile(filepath.Join(root, ".bare", "packed-refs"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for line := range strings.SplitSeq(string(packed), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && migrationPrivateRef(fields[1]) {
			return fmt.Errorf("migration metadata verification failed: worktree-local ref %s remains in common packed-refs", fields[1])
		}
	}
	return nil
}

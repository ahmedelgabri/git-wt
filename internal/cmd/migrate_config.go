package cmd

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
)

type migrationConfigState struct {
	// Keep all values in order, including the distinction between implicit
	// booleans (empty payload) and explicit empty values (a newline payload).
	values  map[string][]string
	remotes string
}

func readMigrationConfig(ctx context.Context, root string) (migrationConfigState, error) {
	state := migrationConfigState{values: make(map[string][]string)}
	out, err := git.QueryRawInContext(ctx, root, "config", "--includes", "--null", "--list")
	if err != nil {
		return state, err
	}
	for record := range strings.SplitSeq(strings.TrimSuffix(out, "\x00"), "\x00") {
		if record == "" {
			continue
		}
		key, _, _ := strings.Cut(record, "\n")
		state.values[key] = append(state.values[key], record[len(key):])
	}
	state.remotes, err = git.QueryRawInContext(ctx, root, "remote")
	return state, err
}

func migrationPathWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func checkMigrationIncludes(ctx context.Context, root string) error {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	// Git expands ~ and installation prefixes, traverses active nested includes,
	// and reports even inactive includeIf directives in files it reads.
	out, err := git.QueryRawInContext(ctx, root, "config", "--includes", "--null", "--show-origin", "--type=path", "--get-regexp", `^include(if\..*)?\.path$`)
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return nil
	}
	if err != nil {
		return err
	}
	fields := strings.Split(strings.TrimSuffix(out, "\x00"), "\x00")
	if len(fields)%2 != 0 {
		return fmt.Errorf("cannot inspect config include origins")
	}
	gitDir := filepath.Join(root, ".git")
	for i := 0; i < len(fields); i += 2 {
		origin, file := strings.CutPrefix(fields[i], "file:")
		_, include, value := strings.Cut(fields[i+1], "\n")
		if !value || !file && (fields[i] != "command line:" || !filepath.IsAbs(include)) {
			return fmt.Errorf("unsupported config include origin %q", fields[i])
		}
		if !file {
			origin = ""
		} else if !filepath.IsAbs(origin) {
			origin = filepath.Join(root, origin)
		}
		target := include
		relative := !filepath.IsAbs(target)
		if relative {
			target = filepath.Join(filepath.Dir(origin), target)
		}
		// Resolve existing symlinks too: an external path pointing into the
		// repository will stop working when its target moves.
		for _, path := range []*string{&origin, &target} {
			if *path == "" {
				continue
			}
			resolved, err := filepath.EvalSymlinks(*path)
			if err == nil {
				*path = resolved
			} else if !os.IsNotExist(err) {
				return err
			}
		}
		if relative && migrationPathWithin(gitDir, origin) && migrationPathWithin(gitDir, target) {
			continue
		}
		if migrationPathWithin(root, origin) && relative || migrationPathWithin(root, target) {
			return fmt.Errorf("unsupported config include %q from %s: its path changes meaning during migration; keep included settings inside .git with relative includes, or use an absolute path outside the repository", include, fields[i])
		}
	}
	return nil
}

func migrationURLPrefixes(values map[string][]string) []string {
	var prefixes []string
	for key, entries := range values {
		if strings.HasPrefix(key, "url.") && (strings.HasSuffix(key, ".insteadof") || strings.HasSuffix(key, ".pushinsteadof")) {
			for _, value := range entries {
				prefixes = append(prefixes, strings.TrimPrefix(value, "\n"))
			}
		}
	}
	return prefixes
}

func migrationURL(source, url string, prefixes []string) string {
	alias := slices.ContainsFunc(prefixes, func(prefix string) bool { return strings.HasPrefix(url, prefix) })
	if !alias && !filepath.IsAbs(url) && !strings.Contains(url, ":") {
		return filepath.Join(source, url)
	}
	return url
}

func verifyMigrationConfig(ctx context.Context, plan migratePlan, root string, source bool) error {
	actual, err := readMigrationConfig(ctx, root)
	if err != nil {
		return fmt.Errorf("migration configuration verification failed at %s: %w", root, err)
	}
	if actual.remotes != plan.config.remotes {
		return fmt.Errorf("migration configuration verification failed at %s: effective remotes changed", root)
	}
	expected := maps.Clone(plan.config.values)
	if !source {
		for _, setting := range [][2]string{
			{"core.bare", "true"},
			{"core.logallrefupdates", "true"},
			{"worktree.userelativepaths", "true"},
			{"core.repositoryformatversion", "1"},
			{"extensions.relativeworktrees", "true"},
		} {
			values := actual.values[setting[0]]
			if len(values) == 0 || values[len(values)-1] != "\n"+setting[1] {
				return fmt.Errorf("migration configuration verification failed at %s: %s must be %s", root, setting[0], setting[1])
			}
			delete(expected, setting[0])
			delete(actual.values, setting[0])
		}
		delete(expected, "core.worktree")
		prefixes := migrationURLPrefixes(plan.config.values)
		for key, values := range expected {
			if strings.HasPrefix(key, "remote.") && (strings.HasSuffix(key, ".url") || strings.HasSuffix(key, ".pushurl")) {
				normalized := make([]string, len(values))
				for i, value := range values {
					normalized[i] = "\n" + migrationURL(plan.repoRoot, strings.TrimPrefix(value, "\n"), prefixes)
				}
				expected[key] = normalized
			}
		}
		// Git may configure tracking for a newly created default branch. Never
		// exempt a pre-existing setting, or unrelated branch configuration.
		if plan.defaultBranch != "" && plan.defaultBranch != plan.currentBranch && !strings.Contains(plan.refs, "refs/heads/"+plan.defaultBranch+" ") {
			for _, setting := range []string{"remote", "merge", "rebase"} {
				key := "branch." + plan.defaultBranch + "." + setting
				if _, exists := expected[key]; !exists {
					delete(actual.values, key)
				}
			}
		}
	}
	for _, key := range slices.Sorted(maps.Keys(expected)) {
		if !slices.Equal(expected[key], actual.values[key]) {
			// Values may contain credentials. Report keys, never their contents.
			return fmt.Errorf("migration configuration verification failed at %s: effective setting %s changed", root, key)
		}
	}
	for _, key := range slices.Sorted(maps.Keys(actual.values)) {
		if _, exists := expected[key]; !exists {
			return fmt.Errorf("migration configuration verification failed at %s: unexpected effective setting %s", root, key)
		}
	}
	return nil
}

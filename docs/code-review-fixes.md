# Code Review Fixes

Addresses bugs, correctness issues, code quality problems, and test safety
concerns identified during code review.

## Changes

### Bug fixes

1. **Race condition in picker output goroutine** (`internal/picker/picker.go`):
   Added `sync.WaitGroup` to ensure the output-collecting goroutine finishes
   draining `outputChan` before reading `selected`.

2. **Ignored `ui.Spin` error in clone** (`internal/cmd/clone.go`): Explicitly
   assigned the return value to `_` with a comment explaining the fallback
   handles it.

3. **Ignored `copyFileSimple` error in migrate** (`internal/cmd/migrate.go`):
   Now checks and returns the error (wrapped) when restoring the git index
   fails.

4. **`.bare` substring false positives in cache** (`internal/worktree/cache.go`):
   Replaced `strings.Contains(path, ".bare")` with
   `filepath.Base(path) != ".bare"` to avoid matching paths that contain
   `.bare` as a substring.

5. **`copyFileSimple` hardcoded permissions** (`internal/cmd/migrate.go`):
   Now preserves source file permissions via `os.Stat` instead of hardcoding
   `0o644`.

6. **Misleading debug output** (`internal/git/git.go`): `RunIn` and
   `RunInWithOutput` debug output changed from `git -C %s` to `[in %s] git %s`
   since the actual execution uses `cmd.Dir`, not `-C`.

### Code quality

7. **Extracted shared helpers** (`internal/cmd/helpers.go`):
   - `configureBareRepo(dir)`: Sets the three git config keys needed after
     creating a bare repo structure. This also fixes migrate missing the
     `worktree.useRelativePaths` config.
   - `cleanupLocalBranchRefs(dir)`: Removes invalid local branch refs created
     by bare clone.
   - Both clone and migrate now use these shared functions.

8. **Consolidated redundant color definitions** (`internal/ui/ui.go`): Removed
   `mutedColor` and `mutedStyle` (identical to `subtleColor`/`subtleStyle`).
   `Muted()` now uses `subtleStyle`, `MutedColor()` returns `subtleColor`.

9. **Refactored global state to functions**:
   - `git.Debug` var replaced with `git.debug()` function that reads
     `os.Getenv("DEBUG")` each time.
   - `ui.noColor` var replaced with `ui.noColor()` function that reads
     `os.Getenv("NO_COLOR")` each time.
   - Tests updated from direct variable mutation to `t.Setenv`, which is
     safer for parallel test execution.

## Verification

- `go build ./...` -- compiles
- `go vet ./...` -- no issues
- `go test ./...` -- all unit tests pass
- `go test -race ./...` -- no race conditions
- `bats tests/` -- all 109 E2E tests pass
- `nix fmt -- --fail-on-change` -- formatting clean (0 changed)

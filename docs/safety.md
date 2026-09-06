# Repository safety

## Clone

A failed initial bare clone cleans up only the destination git-wt created. Once the bare clone succeeds, the download is retained even if writing the `.git` pointer, configuring the repository, fetching, entering a branch name, or creating a worktree subsequently fails. These late failures return non-zero and print a warning to stderr with the retained path and recovery instructions. Inspect the downloaded branches with `git --git-dir=<destination>/.bare branch -a`. Complete layout configuration if needed, then use `git -C <destination> wt add <path> <branch>` to create a worktree.

## Migration

`git wt migrate` is experimental. Stop editors, background agents, builds, and other Git operations that write to the repository before starting. Migration detects many concurrent changes, but cannot lock out arbitrary external file writers.

Preparation copies the complete `.git` database into a temporary sibling directory, creates an empty worktree for the current branch, and copies the original working directory into it. Nested repositories, ignored files, symlinks, staged deletions, unstaged deletions, and executable modes are preserved. Split indexes are materialized into a standalone worktree index. Local configuration, hooks, refs, and reflogs are copied rather than reconstructed from a whitelist. Relative local remote URLs become absolute so they continue to resolve from linked worktrees. URLs matching user-defined `insteadOf` or `pushInsteadOf` prefixes stay unchanged so Git's rewrite settings still apply. Native checkout hooks remain disabled during migration preparation; their configuration is preserved for subsequent Git operations.

Verification compares filesystem contents and modes, index entries, original refs, and stash entries. Git also checks object connectivity and worktree usability. Finalization moves entries synchronously, preserves the repository root inode, and repeats verification at the final paths. An interruption during preparation cancels preparation; finalization finishes or restores the original layout without a concurrent cleanup goroutine.

The original repository is retained at the printed sibling `<repo>-backup-*` path. It is a normal Git repository that can be inspected or used directly. Keep it until you have checked the migrated worktrees, hooks, and configuration. Absolute paths in custom commands or configuration may need manual adjustment. Migration needs disk space for a full copy, including ignored files and the object database.

Failure messages distinguish an original repository restored at its initial path from recovery files still present in the backup or staging directories. Empty directories are not reported as containing recovery files. If restoration fails, the command reports the nonempty recovery directories and does not delete their remaining contents; locations it cannot inspect are flagged for manual recovery. Do not run another migration over a partially restored layout. Stop writers, inspect the original backup and staging directories, and recover from those copies before proceeding.

Migration refuses submodules, existing linked worktrees, sparse checkout, alternate object directories, per-worktree configuration, unborn branches, Git lock files, symlinked Git metadata, and in-progress merge, rebase, cherry-pick, revert, or sequencer operations. Unsupported special files also stop preparation. `--dry-run` and `DEBUG=1` do not restructure the filesystem.

## Removal

Explicit removal and cleanup refuse tracked modifications, non-ignored untracked files, and commits that have no other retained branch or tag. Ignored files do not block removal and are deleted with the worktree, as with native Git. This includes `node_modules/`, `target/`, build output, and ignored `.env` files. Save any valuable ignored files before removing or cleaning up a worktree. `--force` permits discarding protected changes and unpreserved commits only for explicit targets. Current and locked worktrees remain protected. The plan warns when force is enabled, and the command checks identity and safety again after before-remove hooks run.

Cleanup filters remain conservative:

- `--merged` selects clean branches with a remote upstream that are fully merged into the cleanup base.
- `--gone` also requires full merge into the cleanup base. A deleted remote branch does not prove local commits are preserved.
- `--stale` selects missing, unlocked worktree paths with attached branches. It removes metadata without deleting their branches. Detached metadata is retained because its HEAD may be the only remaining reference to a commit. Existing directories are excluded even when Git reports their metadata prunable, such as when the worktree's `.git` file is broken. Inspect the files and use `git wt repair <path>` before removal.
- `--sweep` combines these filters. It cannot be combined with `--force`.

Cleanup selection and the post-hook safety check use the same cleanup-base resolver. `wt.cleanupBase` takes precedence and must name an existing local branch, either `main` or `refs/heads/main`. That branch is protected from cleanup, including when a hook changes the setting. Tags cannot shadow it. Without this setting, cleanup uses the invoking branch's configured remote, or normal remote discovery if none is configured, to discover the default branch. `branch.<name>.remote=.` is ambiguous because its HEAD is the current branch, so cleanup refuses with the branch name, config key, and an explicit-base example: `git config wt.cleanupBase refs/heads/main`. Unavailable defaults also produce an error. Raw remote URLs require network discovery on each run, bounded by `wt.remoteTimeout`; an explicit local base requires no network lookup. `--stale` alone does not need a cleanup base. This setting does not change the remote used by other commands.

`--delete-remote` uses each target's configured upstream remote and branch name. It does not guess from the invoking worktree. The command resolves every configured push destination, including `pushurl` and URL rewrites, and checks all of them before local removal. Each destination gets its own expected-commit lease. Remote-specific transport settings remain in effect. If cascading URL rewrites would redirect verification to a different destination, removal stops before changing anything; use native Git for that configuration. An already-absent remote branch produces a notice and skips deletion for that destination. Network errors and rejected deletion return a non-zero exit. If remote deletion fails after local removal, the error explains that local removal has completed.

When there is exactly one push URL and it matches the effective fetch URL, verification and deletion use the named remote without URL overrides, including on older Git. Git's `insteadOf` and `pushInsteadOf` expansion applies before comparison. Multiple push URLs or differing fetch/push URLs need destination overrides and require Git 2.46 or newer, which supports clearing URL lists with empty configuration values. Compatibility is checked for the entire selection before hooks or local changes, including through `destroy`. Older or unrecognized versions are refused with upgrade and local-only removal guidance. Dry-run plans remain available. This check prevents partial completion, such as a stranded remote branch or an error after local removal; the lease still protects against deleting a changed remote tip.

As with native Git, separate worktree and remote operations are not one atomic transaction. Avoid concurrent branch rewrites during removal. Local branch deletion uses an expected object ID so a concurrently advanced branch is retained rather than silently deleted. After successful deletion, repository-local `branch.<name>.*` settings are removed as with native `git branch -D`; inherited defaults are preserved.

## Updates and scripting

Default-branch discovery uses the local remote HEAD when available. Otherwise, `wt.remoteTimeout` sets the discovery deadline as a duration such as `30s` or `2m`. The default is `10s`; `0` disables the deadline without disabling cancellation. Invalid or negative values use the default. Git's usual global, repository, and one-off configuration precedence applies, for example `git -c wt.remoteTimeout=0 wt update`. Fetch and pull retain their own transport behavior.

`git wt update` runs `git fetch --all --prune`, then plain `git pull` in the default branch's worktree. It does not force a tag-pruning or pull policy. Tag pruning follows `fetch.pruneTags`, `remote.<name>.pruneTags`, and explicit tag refspecs; it can delete local-only tags when enabled. The pull strategy follows `pull.rebase`, `branch.<name>.rebase`, and `pull.ff`. Configure these globally, per repository, or for one invocation:

```sh
git -c fetch.pruneTags=true wt update
git -c pull.rebase=true wt update
git -c pull.ff=only wt update
```

Git's normal configuration precedence applies. For example, a per-remote pruning setting takes precedence over the generic `fetch.pruneTags` setting. To override that setting for one invocation, use `git -c remote.origin.pruneTags=false wt update`. Explicit tag refspecs remain subject to `--prune` even when automatic tag pruning is disabled.

`git wt ls` is an alias for `git wt list`. Without `--json`, both commands pass native Git options and output through unchanged. For native machine-readable records, use `git wt list --porcelain -z` and parse NUL-delimited records rather than splitting paths on whitespace or newlines.

`git wt list --json` and `git wt ls --json` emit a JSON array of non-bare worktrees in Git's listing order. The bare database entry is excluded, while a standard repository's main worktree is included. Empty results are `[]`. Every object always contains these fields:

| Field             | Type    | Meaning                                                            |
| ----------------- | ------- | ------------------------------------------------------------------ |
| `path`            | string  | Absolute worktree path as reported by Git, including missing paths |
| `branch`          | string  | Local branch name without `refs/heads/`; empty for detached HEAD   |
| `head`            | string  | Full HEAD object ID; Git reports an all-zero ID for an unborn HEAD |
| `detached`        | boolean | Whether the worktree has detached HEAD                             |
| `locked`          | boolean | Whether the worktree is locked                                     |
| `locked_reason`   | string  | Lock reason, or an empty string                                    |
| `prunable`        | boolean | Whether Git marks the worktree metadata prunable                   |
| `prunable_reason` | string  | Git's prune reason, or an empty string                             |

JSON escapes tabs, newlines, and quotes in paths and reasons. Invalid UTF-8 metadata returns an error instead of silently changing paths; use native porcelain for those repositories. JSON mode works from worktree subdirectories and in `DEBUG` mode without adding human output to stdout. Errors return non-zero and are written to stderr. `--json` cannot be combined with native options such as `--porcelain`, `-z`, `--verbose`, or `--expire`; `--json=false` selects native output.

`git wt add` prints its created path on stdout and sends human output to stderr. Accepting a blank path uses the suggested default. Escape, Ctrl-C, and empty EOF cancel input instead of accepting that default. Nonempty input ending at EOF is accepted without requiring a final newline. `add` requires the `.bare` layout; it will not create worktrees inside a standard repository's `.git` directory.

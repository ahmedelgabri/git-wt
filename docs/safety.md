# Migration and removal safety

## Migration

`git wt migrate` is experimental. Stop editors, background agents, builds, and other Git operations that write to the repository before starting. Migration detects many concurrent changes, but cannot lock out arbitrary external file writers.

Preparation copies the complete `.git` database into a temporary sibling directory, creates an empty worktree for the current branch, and copies the original working directory into it. Nested repositories, ignored files, symlinks, staged deletions, unstaged deletions, and executable modes are preserved. Split indexes are materialized into a standalone worktree index. Local configuration, hooks, refs, and reflogs are copied rather than reconstructed from a whitelist. Relative local remote URLs become absolute so they continue to resolve from linked worktrees.

Verification compares filesystem contents and modes, index entries, original refs, and stash entries. Git also checks object connectivity and worktree usability. Finalization moves entries synchronously, preserves the repository root inode, and repeats verification at the final paths. An interruption during preparation cancels preparation; finalization finishes or restores the original layout without a concurrent cleanup goroutine.

The original repository is retained at the printed sibling `<repo>-backup-*` path. It is a normal Git repository that can be inspected or used directly. Keep it until you have checked the migrated worktrees, hooks, and configuration. Absolute paths in custom commands or configuration may need manual adjustment. Migration needs disk space for a full copy, including ignored files and the object database.

If restoration fails, the command reports the recovery directories and does not delete their remaining contents. Do not run another migration over a partially restored layout. Stop writers, inspect the original backup and staging directories, and recover from those copies before proceeding.

Migration refuses submodules, existing linked worktrees, sparse checkout, alternate object directories, per-worktree configuration, unborn branches, Git lock files, symlinked Git metadata, and in-progress merge, rebase, cherry-pick, revert, or sequencer operations. Unsupported special files also stop preparation. `--dry-run` and `DEBUG=1` do not restructure the filesystem.

## Removal

Explicit removal refuses dirty worktrees, including ignored files, and commits that have no other retained branch or tag. `--force` permits discarding that state only for explicit targets. Current and locked worktrees remain protected. The plan warns when force is enabled, and the command checks identity and safety again after before-remove hooks run.

Cleanup filters remain conservative:

- `--merged` selects clean branches with a remote upstream that are fully merged into the default branch.
- `--gone` also requires full merge into the default branch. A deleted remote branch does not prove local commits are preserved.
- `--stale` selects missing, unlocked worktree paths with attached branches. It removes metadata without deleting their branches. Detached metadata is retained because its HEAD may be the only remaining reference to a commit.
- `--sweep` combines these filters. It cannot be combined with `--force`.

`--delete-remote` uses each target's configured upstream remote and branch name. It does not guess from the invoking worktree. The command checks remote state before local removal and uses a lease to reject concurrent remote changes. Network errors and rejected deletion return a non-zero exit. If remote deletion fails after local removal, the error explains that local removal has completed.

As with native Git, separate worktree and remote operations are not one atomic transaction. Avoid concurrent branch rewrites during removal. Local branch deletion uses an expected object ID so a concurrently advanced branch is retained rather than silently deleted.

## Updates and scripting

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

`git wt add` prints its created path on stdout and sends human output to stderr. Accepting a blank path uses the suggested default. Escape, Ctrl-C, and EOF cancel input instead of accepting that default. `add` requires the `.bare` layout; it will not create worktrees inside a standard repository's `.git` directory.

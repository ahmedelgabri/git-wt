# Repository safety

## Clone

A failed initial bare clone cleans up only the destination git-wt created. Once the bare clone succeeds, the download is retained even if writing the `.git` pointer, configuring the repository, fetching, entering a branch name, or creating a worktree subsequently fails. These late failures return non-zero and print a warning to stderr with the retained path and recovery instructions. Inspect the downloaded branches with `git --git-dir=<destination>/.bare branch -a`. Complete layout configuration if needed, then use `git -C <destination> wt add <path> <branch>` to create a worktree.

## Migration

`git wt migrate` is experimental. Stop editors, background agents, builds, and other Git operations that write to the repository before starting. Migration cannot lock out arbitrary external file writers.

Migration converts the repository in place with renames. Nothing is copied, so it needs no extra disk space, and moved files and directories keep their contents, modes (including setgid and sticky bits), symlinks, extended attributes, and ACLs. The repository root keeps its inode, so shells inside it keep working. The steps are:

1. Save `.git/config` and `.git/packed-refs`, the only files Git rewrites in place, under `.git-wt-migrate/backup/`.
2. Move every working entry, including ignored and untracked files and nested repositories, into `.git-wt-migrate/work/`. The branch directory may share a name with one of them.
3. Rename `.git` to `.bare`, write the `.git` pointer file, and configure the bare repository. If `core.logAllRefUpdates` is unset, migration sets it to `true`, because bare repositories otherwise stop writing reflogs; an explicit value such as `always` or `false` is kept. Relative local remote URLs become absolute so they resolve from linked worktrees. URLs matching user-defined `insteadOf` or `pushInsteadOf` prefixes stay unchanged.
4. Register an empty worktree for the current branch and move the working entries into it.
5. Move worktree-local Git metadata into the directory reported by `git -C <worktree> rev-parse --absolute-git-dir`: the index and split-index files, pseudorefs and state files such as `ORIG_HEAD`, `FETCH_HEAD`, and `BISECT_LOG`, the HEAD and pseudoref reflogs, and the `refs/bisect`, `refs/worktree`, and `refs/rewritten` namespaces with their reflogs. Git's `--git-path` decides what is worktree-local. Unknown directories such as `lfs/` stay in `.bare`, where tools that share data between worktrees look for them. Packed private refs are written as loose refs in the worktree and deleted from common storage, so existing and future worktrees cannot resolve them.
6. Verify at the final paths: effective configuration and remote names at the repository root and in the worktree, private ref isolation, refs, index entries, stash entries, and `git status` including ignored files.

Native hooks are disabled during worktree creation and ref cleanup; their configuration is preserved. Filesystem monitor callbacks and optional index writes are disabled for the status and index checks.

Git rewrites `config` and `packed-refs` by replacing the files, as any `git config` command does. Their extended attributes and ACLs are not kept. Everything else keeps its metadata because it is moved, not rewritten. New directories created for the worktree, such as `.bare/worktrees/<branch>`, are created with default permissions inside `.bare`, which keeps the permissions of the original `.git`.

Configuration includes must keep their meaning after relocation. Relative includes from repository config files that escape `.git`, and absolute includes pointing back into the moving repository, are refused before anything moves. Keep included settings inside `.git` with relative include paths, or use stable absolute paths outside the repository. Nested includes are inspected through Git; stable external includes and one-off include overrides remain supported. Conditional `includeIf` rules are not rewritten. A `gitdir:` condition on the repository directory, such as `gitdir:~/code/project/`, still matches after migration. A condition on the old `.git` path does not, so verification fails and migration rolls back. Diagnostics name changed keys without printing their values.

### Recovery

Every step is written to `.git-wt-migrate/journal` and synced before it runs. If a step or the verification fails, or migration receives SIGINT, SIGTERM, or SIGHUP (for example when its terminal closes), it replays the journal in reverse: moves go back, created paths are removed, and saved files are restored. The original repository is then back in place, byte for byte, and the journal is removed.

If the process is killed or the machine loses power, the journal stays. Run `git wt migrate` again from anywhere inside the repository. It finds the journal, restores the original layout, and exits with an error telling you to run it again to migrate. `--dry-run` only reports the interrupted migration. A lock on the journal stops a second `git wt migrate` from undoing one that is still running. Undo never removes a non-empty directory it did not create, and it stops if a path it needs to restore already exists, for example because an editor recreated a file. Rollback records each step it completes in the journal, so after you resolve the conflict, `git wt migrate` resumes where it stopped.

After a successful migration the journal and saved files are removed. There is no backup copy: migration rewrote nothing but `config` and `packed-refs`, and verification compared the result with the original. Absolute paths in custom commands or configuration may need manual adjustment.

If the extra worktree for the default branch cannot be created after the conversion, migration still succeeds and prints a warning with the `git wt add` command to create it later.

Migration refuses submodules, existing linked worktrees or stale `.git/worktrees` metadata, linked-worktree control files in the original `.git`, sparse checkout, alternate object directories, per-worktree configuration, unborn branches, Git lock files, symlinked Git metadata, a `core.hooksPath` inside `.git` (set it to `<repo>/.bare/hooks` after migrating instead), and in-progress merge, rebase, cherry-pick, revert, or sequencer operations. Only the files ref backend is supported. `--dry-run` and `DEBUG=1` do not change the filesystem.

## Removal

Explicit removal and cleanup refuse tracked modifications, non-ignored untracked files, and commits that have no other retained branch or tag. Ignored files do not block removal and are deleted with the worktree, as with native Git. This includes `node_modules/`, `target/`, build output, and ignored `.env` files. Save any valuable ignored files before removing or cleaning up a worktree. Every target is checked before anything is removed. On a terminal, a target with work to discard lists it and asks you to type the worktree name to discard it, or press Enter to skip that target; the rest of the selection continues. Approving one target forces only that target. Without a terminal, such targets are skipped with their reasons, the rest are removed, and the command exits 1. Targets that fail other checks, such as a locked worktree or an unreachable remote, are skipped the same way. `--force` discards protected changes and unpreserved commits without asking, only for explicit targets. With `--delete-remote`, it also skips the check that the remote branch has no commits missing from the local branch, so unfetched commits pushed by others are deleted too. The expected-commit lease still stops deletion if the remote branch changes after that point. Current and locked worktrees remain protected. The plan warns when force is enabled. Because checks run before the confirmation prompt, a skipped target never runs its hooks. Targets are checked again after before-remove hooks run.

Cleanup filters remain conservative:

- `--merged` selects clean branches with a remote upstream that are fully merged into the cleanup base.
- `--gone` also requires full merge into the cleanup base. A deleted remote branch does not prove local commits are preserved.
- `--stale` selects missing, unlocked worktree paths with attached branches. It removes metadata without deleting their branches. Detached metadata is retained because its HEAD may be the only remaining reference to a commit. Existing directories are excluded even when Git reports their metadata prunable, such as when the worktree's `.git` file is broken. Inspect the files and use `git wt repair <path>` before removal.
- `--sweep` combines these filters. It cannot be combined with `--force`.

Cleanup selection and the post-hook safety check use the same cleanup-base resolver. `wt.cleanupBase` takes precedence and accepts an existing local branch, such as `main` or `refs/heads/main`, or an explicitly qualified remote-tracking branch, such as `refs/remotes/origin/main`.

Remote-tracking bases use the locally stored tip from the last fetch, without contacting the remote or fetching implicitly. No corresponding local branch or worktree is required. Cleanup protects the same-name local branch if one exists, even when it lags the remote-tracking tip. The name is the suffix after the longest matching configured remote prefix, or after the first remote-name component when none is configured. Symbolic aliases such as `refs/remotes/origin/HEAD` resolve to their target branch. Cleanup never deletes an upstream whose name differs from its local branch, so a branch created from the base, such as `feat` tracking `origin/main`, cannot delete it. These protections apply again after hooks. Tags and revision expressions are rejected; short names always select local branches.

Without `wt.cleanupBase`, cleanup uses the invoking branch's configured remote, or normal remote discovery if none is configured, to discover the default branch. `branch.<name>.remote=.` is ambiguous because its HEAD is the current branch, so cleanup refuses with the branch name, config key, and an explicit-base example: `git config wt.cleanupBase refs/heads/main`. Unavailable defaults also produce an error. Raw remote URLs require network discovery on each run, bounded by `wt.remoteTimeout`; an explicit local or remote-tracking base requires no network lookup. `--stale` alone does not need a cleanup base. This setting does not change the remote used by other commands.

`--delete-remote` uses each target's configured upstream remote and branch name. It does not guess from the invoking worktree. An upstream with a different name than the local branch is often a shared branch the target was created from, such as `origin/release`. Explicit removal asks you to type that remote name, not the local branch name, before deleting it. Cleanup filters keep such upstreams and say so. The command resolves every configured push destination, including `pushurl` and URL rewrites, and checks all of them before local removal. Each destination gets its own expected-commit lease. Remote-specific transport settings remain in effect. If cascading URL rewrites would redirect verification to a different destination, removal stops before changing anything; use native Git for that configuration. An already-absent remote branch produces a notice and skips deletion for that destination. Network errors and rejected deletion return a non-zero exit. If remote deletion fails after local removal, the error explains that local removal has completed.

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

`git wt add` prints its created path on stdout and sends human output to stderr. Accepting a blank path uses the suggested default. Escape, Ctrl-C, and empty EOF cancel input instead of accepting that default. Cancelling the branch picker or a prompt prints `Cancelled` and exits 0, like declining a confirmation. If `clone` cannot discover the default branch and gets no input, it skips worktree creation as if you pressed Enter. Nonempty input ending at EOF is accepted without requiring a final newline. `add` requires the `.bare` layout; it will not create worktrees inside a standard repository's `.git` directory.

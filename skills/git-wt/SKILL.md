---
name: git-wt
description: Use the `git-wt` CLI to manage Git worktrees in the bare repository layout. Load when a task mentions `git wt`, `git-wt`, creating/removing/switching worktrees, or when the repository has a `.bare` directory with a `.git` file pointing to it.
---

# git-wt

Use this skill when the user asks to manage Git worktrees with `git-wt` or when
a repository uses the bare worktree layout: Git data lives in `.bare`, `.git` is
a file pointing at `./.bare`, and branch worktrees are sibling directories.

## Detecting `git-wt` repositories

- From any directory in the repo, run:

  ```shell
  git rev-parse --git-common-dir
  ```

  If the result ends in `.bare`, treat the repo as a `git-wt` bare worktree
  layout.

- If you are at a repository root, a `.git` file containing `gitdir: ./.bare`
  and a `.bare` directory are also strong signals.
- If uncertain, run:

  ```shell
  git wt doctor
  ```

## Operating rules for agents

- Prefer `git wt` (or `git-wt`) over raw `git worktree` commands in a `git-wt`
  layout so path resolution, fetching, branch cleanup, and safety prompts stay
  consistent.
- Prefer explicit, non-interactive commands. Avoid bare `git wt add`,
  `git wt remove`, and `git wt switch` in automation because they may open
  `fzf` pickers or prompts.
- `git wt add` prints the absolute created worktree path on `stdout`. Capture
  that path if you need to `cd` into the new worktree. Human progress and
  prompts are written to `stderr`.
- Use dry-runs before destructive cleanup. Only delete remote branches when the
  user explicitly asks for remote deletion.
- `DEBUG=1` before a mutating command prints the underlying `git` operations
  instead of executing them.

## Common commands

Inspect repository health and worktrees:

```shell
git wt doctor
git wt status
git wt list --json
git wt ls --json
git wt list --porcelain -z
```

Migration is experimental. Stop other writers first. It verifies copied files, index entries, refs, and stashes, and retains the original repository at a printed sibling backup path. Keep the backup until the user has checked the new layout; never delete it automatically. Unsupported layouts or failed verification must stop migration.

Clone or migrate repositories:

```shell
git wt clone <url> [folder]
git wt migrate --dry-run
git wt migrate
```

Create worktrees with explicit arguments:

```shell
git wt add feature origin/feature
git wt add -b new-feature new-feature
git wt add --detach hotfix HEAD~5
```

Update the default branch worktree using the user's Git configuration. Tag pruning follows `fetch.pruneTags`, `remote.<name>.pruneTags`, and fetch refspecs; it may delete local-only tags. Pull follows `pull.rebase`, `branch.<name>.rebase`, and `pull.ff`. Do not assume updates are fast-forward-only or override the user's settings without authorization:

```shell
git wt update
```

Switch worktrees:

- For an interactive human workflow, use:

  ```shell
  cd "$(git wt switch)"
  ```

- For an agent workflow, parse `git wt list --json` or `git wt ls --json` and `cd` to an object's `path` instead of invoking the interactive switch picker. JSON returns an array of non-bare worktrees with full HEAD IDs, branch names, and detached/locked/prunable flags and reasons. Empty results are `[]`; do not combine `--json` with native Git list options.

Removal refuses dirty worktrees and commits without another retained branch or tag. Do not retry failures with `--force` unless the user explicitly authorizes discarding that work. `--gone` and `--sweep` require full merge into the default branch, even if the upstream was deleted. `--delete-remote` follows the target's configured upstream and skips remote deletion when none is configured.

Remove worktrees safely:

```shell
git wt remove feature --dry-run
printf 'y\n' | git wt remove feature
git wt remove --sweep --dry-run
printf 'cleanup\n' | git wt remove --sweep
```

Remote branch deletion is destructive. Use it only when explicitly requested,
and confirm with the branch name expected by the prompt:

```shell
git wt remove feature --delete-remote --dry-run
printf 'feature\n' | git wt remove feature --delete-remote
```

## Suggested workflow

1. Inspect first with `git wt doctor`, `git wt status`, or `git wt list`.
2. Explain the planned operation when it will create, remove, or migrate
   worktrees.
3. Prefer a dry-run or `DEBUG=1` preview where available.
4. Execute the explicit `git wt` command.
5. Report created paths, removed branches, and any follow-up command the user
   should run.

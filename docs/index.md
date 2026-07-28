---
layout: default
title: git-wt
---

<p align="center">
  <img src="logo-light.svg" alt="git-wt logo">
</p>

# git-wt

A Git custom command that makes Git worktrees easier to use with interactive
selection, safer destructive flows, repository migration, diagnostics, and
compact dashboards.

`git-wt` uses a [**bare repository** structure](https://gabri.me/blog/git-worktrees-done-right)
where Git data lives in `.bare/` and each branch gets its own sibling
worktree directory.

## Features

- **Bare clone structure** with `.bare/` for Git data
- **Interactive add / switch / remove** flows with fzf
- **Repository migration** from a standard repo to the bare worktree layout
- **Safe cleanup filters** with `git wt remove --sweep`
- **Repository diagnostics** with `git wt doctor`
- **Status dashboard** with `git wt status`
- **Structured output** with `git wt list --json`
- **Agent skill installer** with `git wt agent-skill`
- **Dry-run support** for destructive operations

## Quick Start

### Installation

**Homebrew**

```bash
brew install ahmedelgabri/tap/git-wt
```

**Nix Flakes**

```bash
nix run github:ahmedelgabri/git-wt
```

### Basic Usage

```bash
# Clone with the bare worktree layout
git wt clone https://github.com/user/repo.git

# Migrate an existing repo
git wt migrate

# Create a worktree interactively
git wt add

# Switch between worktrees
cd "$(git wt switch)"

# Or source the shell integration once (bash/zsh/fish) so
# `git wt switch` changes directory by itself
eval "$(git-wt init zsh)"

# Show repository health
git wt doctor

# Show status for all worktrees
git wt status

# Install the agent skill
git wt agent-skill

# Sweep safe cleanup candidates
git wt remove --sweep --dry-run
```

## Repository Structure

When you clone with `git wt clone`, you get:

```text
repo/
├── .bare/          # Git data (bare repository)
├── .git            # Points to .bare
└── main/           # Worktree for default branch
```

## Hooks

Run shell commands around worktree creation and removal. Hooks are configured through Git config, so they can be scoped per repository or globally with `--global`.

```bash
# Validate the repository before creating a worktree
git config --add wt.beforeadd './scripts/check-worktree.sh'

# Copy generated files into each new worktree
git config --add wt.afteradd 'cp ../main/compile_commands.json .'

# Clean up files while the worktree still exists
git config --add wt.beforeremove './scripts/cleanup-worktree.sh'

# Notify another tool after removal is complete
git config --add wt.afterremove 'workspace-registry remove "$GIT_WT_PATH"'
```

| Hook              | When it runs                                                               | Working directory      | Failure behavior                                        |
| ----------------- | -------------------------------------------------------------------------- | ---------------------- | ------------------------------------------------------- |
| `wt.beforeadd`    | After add arguments and fetching are complete, immediately before creation | Bare repository root   | Prevents worktree creation                              |
| `wt.afteradd`     | After creation and upstream configuration                                  | New worktree           | Leaves the worktree in place and exits non-zero         |
| `wt.beforeremove` | Immediately before removal                                                 | Worktree being removed | Preserves the worktree and exits non-zero               |
| `wt.afterremove`  | After worktree and branch cleanup                                          | Bare repository root   | Removal remains complete and the command exits non-zero |

Every hook receives the lifecycle context through environment variables:

- `GIT_WT_EVENT`: `beforeadd`, `afteradd`, `beforeremove`, or `afterremove`
- `GIT_WT_PATH`: absolute worktree path
- `GIT_WT_BRANCH`: branch name, or empty for detached or unresolved cases
- `GIT_WT_BARE_ROOT`: absolute bare repository root

Each configured value runs with `sh -c`. Repeated `git config --add` values run in order and stop at the first failure for that event; multiline values are supported. Hook output goes to stderr so successful `git wt add` output remains machine-readable.

Before-hooks are not transactional: the subsequent Git operation can still fail after a hook succeeds, so side effects should be idempotent. After-hook failures cannot roll back an operation that already completed. Removing the current worktree or a locked worktree is rejected before any hook runs; for stale, missing, or prunable worktrees the removal proceeds with the hooks skipped. `DEBUG=1` echoes hooks instead of running them.

Hooks apply to `git wt add` and `git wt remove`; the initial worktree created by `git wt clone` does not trigger add hooks.

## Commands

| Command             | Description                                                |
| ------------------- | ---------------------------------------------------------- |
| `clone <url>`       | Clone a repo with the bare worktree structure              |
| `migrate`           | Convert an existing repo to the bare worktree structure    |
| `add [options] ...` | Create a new worktree                                      |
| `remove [worktree]` | Remove worktrees directly or by safe cleanup filters       |
| `doctor`            | Run repository diagnostics                                 |
| `agent-skill`       | Install the git-wt agent skill                             |
| `init <shell>`      | Print shell integration for automatic directory switching  |
| `status`            | Show a compact dashboard for linked worktrees              |
| `list`              | List worktrees with table, JSON, or passthrough Git output |
| `switch`            | Interactive worktree selection                             |
| `update`            | Fetch remotes and update the default branch                |

Native `git worktree` commands (`lock`, `unlock`, `move`, `prune`, `repair`) are also supported as pass-through commands.

## Agent Skill

Install an [Agent Skills](https://agentskills.io/)-compatible skill so coding
agents can discover and use `git-wt` workflows:

```bash
git wt agent-skill
```

By default this writes `~/.agents/skills/git-wt/SKILL.md`. Use
`git wt agent-skill --dir ~/.claude/skills` for a different skill root,
`--print` to review the skill, or `--force` to overwrite an existing copy.

## Claude Code Integration

[Claude Code](https://claude.ai/code) can create and remove worktrees
automatically during agentic sessions. Configure the `WorktreeCreate` and
`WorktreeRemove` hooks in your project or user `settings.json` to delegate
those operations to `git wt`, keeping every worktree consistent with the bare
repository layout:

```json
{
  "WorktreeCreate": [
    {
      "hooks": [
        {
          "type": "command",
          "command": "git wt add \"$(cat /dev/stdin | jq -r '.name')\""
        }
      ]
    }
  ],
  "WorktreeRemove": [
    {
      "hooks": [
        {
          "type": "command",
          "command": "echo y | git wt rm \"$(cat /dev/stdin | jq -r '.worktree_path')\""
        }
      ]
    }
  ]
}
```

The hooks receive a JSON payload on stdin. `WorktreeCreate` reads the `.name`
field (the branch name) and passes it to `git wt add`. `WorktreeRemove` reads
`.worktree_path` and passes it to `git wt rm`; the leading `echo y |` confirms
the interactive prompt non-interactively.

## Dependencies

- `git` (`2.48.0+` for relative worktree support)

## License

[MIT](https://github.com/ahmedelgabri/git-wt/blob/main/LICENSE)

---

[View on GitHub](https://github.com/ahmedelgabri/git-wt)

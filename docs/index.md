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
- **Dry-run support** for destructive operations

## Quick Start

### Installation

**Homebrew**

```bash
brew tap ahmedelgabri/git-wt
brew install git-wt
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

# Show repository health
git wt doctor

# Show status for all worktrees
git wt status

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

## Commands

| Command             | Description                                                |
| ------------------- | ---------------------------------------------------------- |
| `clone <url>`       | Clone a repo with the bare worktree structure              |
| `migrate`           | Convert an existing repo to the bare worktree structure    |
| `add [options] ...` | Create a new worktree                                      |
| `remove [worktree]` | Remove worktrees directly or by safe cleanup filters       |
| `doctor`            | Run repository diagnostics                                 |
| `status`            | Show a compact dashboard for linked worktrees              |
| `list`              | List worktrees with table, JSON, or passthrough Git output |
| `switch`            | Interactive worktree selection                             |
| `update`            | Fetch remotes and update the default branch                |

Native `git worktree` commands (`lock`, `unlock`, `move`, `prune`, `repair`) are also supported as pass-through commands.

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

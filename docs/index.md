---
layout: default
title: git-wt
---

# git-wt

A Git custom command that enhances Git's native worktree functionality with interactive features, automation, and repository migration capabilities.

git-wt uses a [**bare repository** structure](https://gabri.me/blog/git-worktrees-done-right) where the git data lives in a `.bare` directory and each branch gets its own worktree directory.

## Why Git Worktrees?

Git worktrees allow you to have multiple branches checked out simultaneously in different directories:

- Work on multiple features in parallel without stashing
- Review PRs while keeping your work intact
- Run tests on one branch while developing on another
- Compare implementations across branches side-by-side

## Features

- **Bare clone structure** - Git data in `.bare/`, each branch in its own directory
- **Interactive branch selection** with fzf for creating and switching worktrees
- **Repository migration** - convert existing repos to worktree structure
- **Automatic upstream tracking** when creating worktrees from remote branches
- **Multi-select support** for batch operations (remove, destroy)
- **Dry-run mode** for destructive operations
- **Preserves uncommitted changes** during migration

## Quick Start

### Installation

**Homebrew:**

```bash
brew tap ahmedelgabri/git-wt
brew install git-wt
```

**Nix Flakes:**

```bash
nix run github:ahmedelgabri/git-wt
```

**Manual:**

```bash
curl -o ~/.local/bin/git-wt https://raw.githubusercontent.com/ahmedelgabri/git-wt/main/git-wt
chmod +x ~/.local/bin/git-wt
```

### Basic Usage

```bash
# Clone with worktree structure
git wt clone https://github.com/user/repo.git

# Migrate existing repo
git wt migrate

# Create new worktree (interactive)
git wt add

# Switch between worktrees
cd $(git wt switch)

# Remove worktree
git wt remove feature-branch
```

## Repository Structure

When you clone with `git wt clone`, you get:

```
repo/
├── .bare/          # Git data (bare repository)
├── .git            # Points to .bare
└── main/           # Worktree for default branch
```

## Commands

| Command              | Description                                           |
| -------------------- | ----------------------------------------------------- |
| `clone <url>`        | Clone repo with worktree structure                    |
| `migrate`            | Convert existing repo to worktree structure           |
| `add [options] ...`  | Create new worktree (supports all git worktree flags) |
| `remove [worktree]`  | Remove worktree and local branch                      |
| `destroy [worktree]` | Remove worktree and delete local + remote branches    |
| `update`             | Fetch all and update default branch                   |
| `switch`             | Interactive worktree selection                        |

All native `git worktree` commands (list, lock, unlock, move, prune, repair) are also supported as pass-through.

## Dependencies

- `git` (2.7+ for worktree support)
- `fzf` (optional, for interactive commands)

## License

[MIT](https://github.com/ahmedelgabri/git-wt/blob/main/LICENSE)

---

[View on GitHub](https://github.com/ahmedelgabri/git-wt)

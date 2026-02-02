# git-wt

A git worktree management tool that enhances Git's native worktree functionality
with interactive features, automation, and repository migration capabilities.

## Why git worktrees?

Git worktrees allow you to have multiple branches checked out simultaneously in
different directories. This is useful for:

- Working on multiple features in parallel without stashing
- Reviewing PRs while keeping your work intact
- Running tests on one branch while developing on another
- Comparing implementations across branches side-by-side

## Features

- **Interactive branch selection** with fzf for creating and switching worktrees
- **Repository migration** - convert existing repos to worktree structure
- **Automatic upstream tracking** when creating worktrees from remote branches
- **Multi-select support** for batch operations (remove, destroy)
- **Dry-run mode** for destructive operations
- **Preserves uncommitted changes** during migration (staged, unstaged, stashes)

## Installation

### Using Nix Flakes (recommended)

Add to your flake inputs:

```nix
{
  inputs.git-wt.url = "github:ahmedelgabri/git-wt";
}
```

Then add to your packages:

```nix
inputs.git-wt.packages.${system}.default
```

Or run directly:

```bash
nix run github:ahmedelgabri/git-wt
```

### Manual Installation

```bash
curl -o ~/.local/bin/git-wt https://raw.githubusercontent.com/ahmedelgabri/git-wt/main/git-wt
chmod +x ~/.local/bin/git-wt
```

### Dependencies

- `git` (2.7+ for worktree support)
- `fzf` (for interactive commands)
- `bash` 4+

## Usage

### Clone a repository with worktree structure

```bash
git wt clone https://github.com/user/repo.git
```

This creates:

```
repo/
├── .bare/          # Git data (bare repository)
├── .git           # Points to .bare
└── main/          # Worktree for default branch
```

### Migrate an existing repository

```bash
cd existing-repo
git wt migrate
```

Converts your repo to the worktree structure while preserving all uncommitted
changes, staged files, and stashes.

### Create a new worktree

```bash
# Interactive mode - select from remote branches with fzf
git wt add

# From a remote branch
git wt add feature origin/feature

# Create new branch
git wt add -b new-feature new-feature
```

### Switch between worktrees

```bash
cd $(git wt switch)
```

### Remove worktrees

```bash
# Interactive multi-select
git wt remove

# Direct removal (local branch only)
git wt remove feature-branch

# Preview what would be removed
git wt remove --dry-run
```

### Destroy worktrees (removes remote branch too)

```bash
# Interactive with confirmation
git wt destroy

# Direct destruction
git wt destroy feature-branch
```

### Update default branch

```bash
git wt update # or: git wt u
```

Fetches all remotes and pulls the default branch (main/master).

### List worktrees

```bash
git wt list
```

## Commands

| Command              | Description                                        |
| -------------------- | -------------------------------------------------- |
| `clone <url>`        | Clone repo with worktree structure                 |
| `migrate`            | Convert existing repo to worktree structure        |
| `add [branch]`       | Create new worktree (interactive if no args)       |
| `remove [worktree]`  | Remove worktree and local branch                   |
| `destroy [worktree]` | Remove worktree and delete local + remote branches |
| `update` / `u`       | Fetch all and update default branch                |
| `switch`             | Interactive worktree selection                     |
| `list`               | List all worktrees                                 |

All native `git worktree` commands (lock, unlock, move, prune, repair) are also
supported as pass-through.

## Shell Completions

Completions are included for Bash, Zsh, and Fish. When installed via Nix, they
are automatically available.

For manual installation, source the appropriate file from `completions/`.

## Development

```bash
# Enter development shell
nix develop

# Format code
nix fmt

# Run checks
nix flake check
```

## License

MIT

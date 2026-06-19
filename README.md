<center>
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/logo-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="docs/logo-light.svg">
  <img alt="Logo" src="docs/logo-light.svg">
</picture>
</center>

# git-wt

A Git custom command that makes Git worktrees easier to use with interactive
selection, safer destructive flows, repository migration, diagnostics, and
compact dashboards.

`git-wt` uses the [**bare repository** structure](https://gabri.me/blog/git-worktrees-done-right)
where Git data lives in `.bare/` and each branch gets its own sibling
worktree directory.

## Why Git Worktrees?

Git worktrees let you keep multiple branches checked out at the same time in
separate directories. They are useful for:

- working on multiple features in parallel without stashing
- reviewing PRs while keeping local work intact
- comparing implementations side by side
- running tests on one branch while developing on another

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
- **Preserves uncommitted changes, stashes, remotes, and repo-local config** during migration

## Dependencies

- `git` (`2.48.0+` for relative worktree support)

## Installation

### Homebrew

```bash
brew install ahmedelgabri/tap/git-wt
```

Shell completions are installed automatically for bash, zsh, and fish.

### Nix Flakes

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

### Manual installation

Download the latest release archive for your platform from the
[releases page](https://github.com/ahmedelgabri/git-wt/releases/latest):

```bash
curl -sL https://github.com/ahmedelgabri/git-wt/releases/latest/download/git-wt-VERSION-OS-ARCH.tar.gz | tar xz
cp git-wt-VERSION-OS-ARCH/git-wt ~/.local/bin/
```

Replace `VERSION` with the current release version and choose the correct
platform archive.

### Shell completions

For manual installs, the release archives include a `completions/` directory:

```bash
# Bash
cp completions/git-wt.bash ~/.local/share/bash-completion/completions/git-wt

# Zsh
cp completions/_git-wt ~/.local/share/zsh/site-functions/_git-wt
cp completions/_git_wt ~/.local/share/zsh/site-functions/_git_wt # enables `git wt` completion

# Fish
cp completions/git-wt.fish ~/.config/fish/completions/git-wt.fish
```

For zsh, both completion files are needed:

- `_git-wt` completes the standalone `git-wt` command
- `_git_wt` bridges `git wt ...` when git/oh-my-zsh-style completion wrappers
  dispatch to the underscore form

### Agent skill

Install an [Agent Skills](https://agentskills.io/)-compatible skill so coding
agents can discover and use `git-wt` workflows:

```bash
git wt agent-skill
```

By default this writes `~/.agents/skills/git-wt/SKILL.md`. Use
`git wt agent-skill --dir ~/.claude/skills` for a different skill root,
`--print` to review the skill, or `--force` to overwrite an existing copy.

## Usage

### Clone with the bare worktree layout

```bash
git wt clone https://github.com/user/repo.git
```

This creates:

```text
repo/
├── .bare/         # Git data (bare repository)
├── .git           # gitdir pointer to .bare
└── main/          # Worktree for the default branch
```

### Migrate an existing repository

```bash
cd existing-repo
git wt migrate
```

This converts a standard Git repository into the bare worktree layout while
preserving tracked changes, untracked files, stashes, remotes, and selected
repo-local config.

### Create a worktree

```bash
# Interactive mode

git wt add

# From a remote branch
git wt add feature origin/feature

# Create a new branch
git wt add -b new-feature new-feature

# Detached, locked, or quiet modes
git wt add --detach hotfix HEAD~5
git wt add --lock -b wip wip-branch
git wt add --quiet -b feature feature
```

### Switch worktrees

```bash
cd "$(git wt switch)"
```

### Remove a worktree and local branch

```bash
git wt remove feature-branch
git wt remove --dry-run feature-branch
```

### Remove a worktree and local + remote branch

```bash
git wt remove feature-branch --delete-remote
```

### Sweep safe cleanup candidates

```bash
git wt remove --sweep

git wt remove --sweep --dry-run
```

### Inspect repository health

```bash
git wt doctor
```

### Show worktree status

```bash
git wt status
```

### List worktrees

```bash
git wt list
git wt list --json
git wt list --porcelain
```

### Update the default branch

```bash
git wt update # or: git wt u
```

## Commands

| Command             | Description                                                |
| ------------------- | ---------------------------------------------------------- |
| `clone <url>`       | Clone a repo with the bare worktree structure              |
| `migrate`           | Convert an existing repo to the bare worktree structure    |
| `add [options] ...` | Create a new worktree                                      |
| `remove` / `rm`     | Remove worktrees directly or by safe cleanup filters       |
| `doctor`            | Run repository diagnostics                                 |
| `agent-skill`       | Install the git-wt agent skill                             |
| `status`            | Show a compact dashboard for linked worktrees              |
| `list`              | List worktrees with table, JSON, or passthrough Git output |
| `switch`            | Interactively select a worktree                            |
| `update` / `u`      | Fetch remotes and update the default branch                |

Native `git worktree` commands (`lock`, `unlock`, `move`, `prune`, `repair`)
are also supported as pass-through commands.

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

## Development

```bash
# Enter development shell
nix develop

# Format code
nix fmt

# Run all checks
nix flake check
```

## License

MIT

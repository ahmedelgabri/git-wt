# Homebrew Support

git-wt is available via a Homebrew tap with automatic shell completion support
for bash, zsh, and fish.

## Installation

```bash
brew tap ahmedelgabri/git-wt
brew install git-wt
```

## Updating

```bash
brew update
brew upgrade git-wt
```

## How It Works

The formula:

1. Downloads a prebuilt binary from the GitHub release for your platform
2. Installs `git-wt` to `$(brew --prefix)/bin/`
3. Installs shell completions (bundled in the release archive) to:
   - Bash: `$(brew --prefix)/etc/bash_completion.d/git-wt`
   - Zsh: `$(brew --prefix)/share/zsh/site-functions/_git-wt`
   - Fish: `$(brew --prefix)/share/fish/vendor_completions.d/git-wt.fish`

## Dependencies

- `git` - typically already installed on systems using Homebrew

## Tap Repository

The Homebrew formula is maintained in a separate repository:
[ahmedelgabri/homebrew-git-wt](https://github.com/ahmedelgabri/homebrew-git-wt)

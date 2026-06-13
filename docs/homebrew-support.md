# Homebrew Support

git-wt is available via a Homebrew tap with automatic shell completion support
for bash, zsh, and fish.

## Installation

```bash
brew install ahmedelgabri/tap/git-wt
```

## Updating

```bash
brew update
brew upgrade ahmedelgabri/tap/git-wt
```

## How It Works

The formula:

1. Downloads a prebuilt binary from the GitHub release for your platform
2. Installs `git-wt` to `$(brew --prefix)/bin/`
3. Installs shell completions (bundled in the release archive) to:
   - Bash: `$(brew --prefix)/etc/bash_completion.d/git-wt`
   - Zsh: `$(brew --prefix)/share/zsh/site-functions/_git-wt`
   - Zsh git-subcommand bridge: `$(brew --prefix)/share/zsh/site-functions/_git_wt`
   - Fish: `$(brew --prefix)/share/fish/vendor_completions.d/git-wt.fish`

The two zsh files serve different entry points:

- `_git-wt` completes the standalone `git-wt` binary
- `_git_wt` is a compatibility bridge for `git wt ...` in environments that
  dispatch git subcommand completions using the underscore form (for example,
  some git/oh-my-zsh completion wrappers)

## Dependencies

- `git` - typically already installed on systems using Homebrew

## Tap Repository

The Homebrew formula is maintained in the shared tap repository:
[ahmedelgabri/homebrew-tap](https://github.com/ahmedelgabri/homebrew-tap)

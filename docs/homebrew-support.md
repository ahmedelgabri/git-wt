# Homebrew Support

This document tracks the addition of Homebrew installation support for git-wt.

## Summary

Added Homebrew formula that installs git-wt with automatic shell completion
support for bash, zsh, and fish.

## Changes

### New Files

- `Formula/git-wt.rb` - Homebrew formula

### Modified Files

- `README.md` - Added Homebrew installation instructions

## Installation

Users can install via:

```bash
brew tap ahmedelgabri/git-wt https://github.com/ahmedelgabri/git-wt
brew install git-wt
```

## How It Works

The formula:

1. Clones the repository from the main branch
2. Installs `git-wt` script to `$(brew --prefix)/bin/`
3. Installs shell completions to:
   - Bash: `$(brew --prefix)/etc/bash_completion.d/git-wt`
   - Zsh: `$(brew --prefix)/share/zsh/site-functions/_git-wt`
   - Fish: `$(brew --prefix)/share/fish/vendor_completions.d/git-wt.fish`

## Notes

- The formula uses `head` installation (builds from main branch)
- No tagged releases are required
- Dependencies:
  - `git` - typically already installed on systems using Homebrew
  - `fzf` - required for interactive worktree selection

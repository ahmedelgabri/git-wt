# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- GitHub Actions CI workflow with shellcheck, shfmt, and nix build verification
- Automated release workflow that creates releases on merge to main
- lefthook pre-commit hooks for shellcheck and shfmt
- CHANGELOG.md for tracking changes

### Changed

- `add` command now supports all `git worktree add` flags (`--detach`, `--lock`,
  `--reason`, `--quiet`, `--force`, `--no-checkout`, `--track`, `--guess-remote`,
  `--orphan`, `--relative-paths`, and their negated forms)

## [0.1.0] - 2024-01-01

### Added

- Initial release
- Core worktree management commands: clone, migrate, add, remove, destroy,
  update, switch, list
- Shell completions for Bash, Zsh, and Fish
- Nix flake for reproducible builds
- Interactive mode with fzf integration

# CI/CD and Development Workflow Setup

This document describes the CI/CD pipeline, release process, and pre-commit
hooks added to git-wt.

## Overview

The project now includes:

1. **GitHub Actions CI** - Runs on every push and PR
2. **Automated Releases** - Creates releases when version changes on main
3. **Pre-commit Hooks** - Local linting via lefthook

## GitHub Actions Workflows

### CI Workflow (`.github/workflows/ci.yml`)

Runs on:

- Every push to `main`
- Every pull request targeting `main`

Jobs:

1. **Lint** (ubuntu-latest)
   - Runs `shellcheck` on `git-wt` and bash completions
   - Runs `shfmt` to verify formatting (tabs, `-ci` for case indentation)

1. **Build** (matrix: ubuntu-latest, macos-latest)
   - Installs Nix using DeterminateSystems/nix-installer-action
   - Uses magic-nix-cache for faster builds
   - Runs `nix flake check`
   - Runs `nix build`
   - Verifies the built executable runs (`git-wt --help`)

### Release Workflow (`.github/workflows/release.yml`)

Runs on:

- Every push to `main`

Behavior:

- Extracts version from `flake.nix`
- Checks if a tag for that version already exists
- If no tag exists, creates a GitHub release with:
  - Auto-generated changelog from commits since last tag
  - Installation instructions for Nix and manual methods
- To skip release creation, include `[skip release]` in commit message

## Pre-commit Hooks (lefthook)

Configuration in `lefthook.yml`.

### Setup

Hooks are automatically installed when entering `nix develop`:

```bash
nix develop
# lefthook install runs automatically via shellHook
```

### Hooks

**pre-commit** (parallel execution):

- `shellcheck` - Lints staged bash files
- `shfmt` - Verifies formatting of staged bash files

**pre-push**:

- `nix-build` - Verifies the flake builds before pushing

### Manual Execution

```bash
# Run pre-commit hooks manually
lefthook run pre-commit

# Run pre-push hooks manually
lefthook run pre-push
```

## Release Process

1. Update the version in `flake.nix`:

   ```nix
   version = "0.2.0";  # Update this
   ```

1. Update `CHANGELOG.md` with changes under `[Unreleased]`

1. Commit and push to main:

   ```bash
   git add flake.nix CHANGELOG.md
   git commit -m "chore: bump version to 0.2.0"
   git push
   ```

1. The release workflow will automatically:
   - Detect the new version
   - Create a git tag `v0.2.0`
   - Create a GitHub release with changelog

## Code Style

- **Indentation**: Tabs (not spaces)
- **Case statement indentation**: Use `-ci` flag with shfmt
- **Shellcheck**: Enabled with specific exceptions documented in file headers

### Shellcheck Directives

The main script (`git-wt`) uses these directives:

- `SC2034` - Color variables appear unused in else branch but are used
- `SC2086` - `$CMD` intentionally word-splits for DEBUG mode ("echo git")
- `SC2016` - Single quotes intentional for fzf preview strings

The bash completion script uses:

- `SC2207` - Standard completion array pattern

## Files Added/Modified

### New Files

- `.github/workflows/ci.yml` - CI workflow
- `.github/workflows/release.yml` - Release workflow
- `lefthook.yml` - Pre-commit hook configuration
- `CHANGELOG.md` - Project changelog
- `docs/ci-cd-setup.md` - This documentation

### Modified Files

- `flake.nix` - Added lefthook to devShell, added shellHook for auto-install
- `git-wt` - Added shellcheck directives, reformatted with shfmt
- `completions/git-wt.bash` - Added shellcheck directive, reformatted with shfmt

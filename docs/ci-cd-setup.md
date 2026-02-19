# CI/CD and Development Workflow Setup

This document describes the CI/CD pipeline, release process, and pre-commit
hooks added to git-wt.

## Overview

The project now includes:

1. **GitHub Actions CI** - Runs on every push and PR
2. **Automated Releases** - Creates releases when version changes on main
3. **Pre-commit Hooks** - Local linting via lefthook
4. **treefmt-nix** - Unified formatting via `nix fmt`

## Formatting with treefmt-nix

The project uses [treefmt-nix](https://github.com/numtide/treefmt-nix) for
unified code formatting. This provides:

- Single command formatting: `nix fmt`
- Automatic CI check via `nix flake check`
- Consistent formatting across all contributors

### Configured Formatters

| Formatter | Files                     |
| --------- | ------------------------- |
| gofumpt   | `*.go`                    |
| prettier  | `*.md`, `*.yml`, `*.yaml` |
| alejandra | `*.nix`                   |

### Usage

```bash
# Format all files
nix fmt

# Check formatting without modifying files
nix fmt -- --fail-on-change

# Format specific files
nix fmt -- path/to/file
```

## GitHub Actions Workflows

### CI Workflow (`.github/workflows/ci.yml`)

Runs on:

- Every push to `main`
- Every pull request targeting `main`

Jobs:

1. **Lint** (ubuntu-latest)
   - Runs `nix fmt -- --fail-on-change` to verify formatting
   - Runs `go vet ./...` via nix develop

2. **Test** (matrix: ubuntu-latest, macos-latest)
   - Sets up Go from `go.mod`
   - Installs bats
   - Runs `go test ./...` (unit tests)
   - Runs `bats tests/` (E2E tests)

3. **Build** (matrix: ubuntu-latest, macos-latest)
   - Installs Nix with cache
   - Runs `nix flake check` (includes treefmt formatting check)
   - Runs `nix build`
   - Verifies the built executable runs (`git-wt --help`)

### Release Workflow (`.github/workflows/release.yml`)

Runs on:

- Every push to `main`

Jobs:

1. **Build** (ubuntu-latest)
   - Extracts version from `flake.nix`
   - Checks if a tag for that version already exists
   - Sets up Go from `go.mod`
   - Builds a native binary to generate shell completions and man pages
   - Cross-compiles for 4 targets: `darwin/amd64`, `darwin/arm64`,
     `linux/amd64`, `linux/arm64`
   - Passes ldflags to inject the version string
   - Packages each target into `git-wt-VERSION-OS-ARCH.tar.gz` containing
     the binary, `completions/`, and `man/`
   - Generates `git-wt-VERSION-checksums.txt` (sha256)
   - Uploads all archives and checksums as artifacts

2. **Release** (needs build)
   - Downloads build artifacts
   - Generates changelog from commits since the previous tag
   - Creates a GitHub release with all archives and checksums attached

To skip release creation, include `[skip release]` in the commit message.

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

- `go vet` - Checks Go code for common issues
- `treefmt` - Verifies formatting of all files

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
   version = "2.0.0";  # Update this
   ```

2. Commit and push to main:

   ```bash
   git add flake.nix
   git commit -m "chore: bump version to 2.0.0"
   git push
   ```

3. The release workflow will automatically:
   - Detect the new version
   - Cross-compile binaries for macOS and Linux (amd64 + arm64)
   - Create a git tag `v2.0.0`
   - Create a GitHub release with changelog and binary archives

## Code Style

- **Go**: Formatted with gofumpt
- **Markdown/YAML**: Formatted with prettier
- **Nix files**: Formatted with alejandra

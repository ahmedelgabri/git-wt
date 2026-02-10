# What is git-wt

A bash script (`git-wt`) that implements the [bare repository worktree pattern](https://gabri.me/blog/git-worktrees-done-right). Instead of a traditional `.git/` directory, the git database lives in `.bare/` and a `.git` **file** (not directory) points to it with `gitdir: ./.bare`. Each branch gets its own sibling worktree directory, so `ls` shows your branches:

```
my-project/
├── .bare/       # git database (bare clone)
├── .git         # file containing "gitdir: ./.bare"
├── main/        # worktree for main branch
└── feature/     # worktree for feature branch
```

## Development Environment

Requires Nix. Enter the dev shell first:

```bash
nix develop
```

## Common Commands

```bash
# Format all files (shfmt for bash, prettier for md/yml/json/svg, alejandra for nix)
nix fmt

# Lint
shellcheck git-wt completions/*.bash

# Run all tests
bats tests/

# Run a single test file
bats tests/add.bats

# Run a specific test by name
bats tests/add.bats -f "test name pattern"

# Build
nix build

# Run all checks (build + format verification)
nix flake check

# Debug mode - echoes git mutation commands instead of executing
DEBUG=1 ./git-wt add
```

## Architecture

Single bash script (`git-wt`, ~1650 lines) with command dispatch via case statement. Bash 3.x compatible (indexed arrays, no associative arrays) for macOS support.

**Key design patterns:**

- **`$CMD` vs direct `git`**: Mutation operations use `$CMD` (which becomes `echo git` in DEBUG mode). Read-only operations call `git` directly. This is how the DEBUG/dry-run system works.
- **Worktree cache**: `load_worktree_cache()` parses `git worktree list --porcelain` once per invocation into parallel indexed arrays (`WORKTREE_CACHE_PATHS`, `WORKTREE_CACHE_BRANCHES`, `WORKTREE_CACHE_HEADS`).
- **`resolve_worktree_path()`**: Central resolver that takes user input (name, relative path, absolute path) and resolves it to a full worktree path.
- **NO_COLOR**: Color output respects the `NO_COLOR` environment variable.

## Formatting

- Bash: tabs (shfmt with `indent_size=0`)
- Markdown/YAML/JSON/SVG: prettier
- Nix: alejandra

## Testing

Tests use [bats](https://github.com/bats-core/bats-core) (Bash Automated Testing System). Test helpers are in `tests/test_helper.bash`.

Key test fixtures:

- `init_bare_repo [dirname]` - Creates a bare repo with `.bare/` structure (standard for most tests)
- `init_bare_repo_with_remote [dirname]` - Bare repo with a fake origin for testing remote operations
- `init_repo [dirname]` - Standard git repo (for migration tests)
- `init_repo_with_remote [dirname]` - Standard repo with fake origin

Each test gets an isolated temp directory via `setup_test_env`/`teardown_test_env`.

## Git Hooks (lefthook)

- **pre-commit**: shellcheck on staged files + `nix fmt -- --fail-on-change`
- **pre-push**: `nix build --no-link`

## Distribution

- **Nix Flakes**: `flake.nix` in this repo
- **Homebrew**: separate tap repo at [ahmedelgabri/homebrew-git-wt](https://github.com/ahmedelgabri/homebrew-git-wt)

## Shell Completions

Completion scripts for bash, zsh, and fish live in `completions/`. These are installed automatically by the Nix package.

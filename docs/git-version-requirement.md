# Git version requirement

git-wt 3.1.0 requires Git 2.48.0 or newer. This Git version introduced relative worktree paths, which let repository directories move without breaking their worktrees.

## Changes

- Commands check the installed Git version before running repository operations or hooks.
- Commands report the required version when Git is older, missing, or returns unreadable version output.
- Help and version output remain available without supported Git.
- Migration always verifies the relative-worktree settings instead of accepting absolute-path fallback behavior.
- Remote deletion no longer has a separate Git 2.46 check.
- The README, documentation site, safety guide, command help, and agent skill use the same minimum version.
- CI uses Git from the project's Nix dependencies and builds real Git 2.38.0 for rejection tests.
- The package version is 3.1.0. Future release notes state the Git requirement.

## Testing

Unit tests cover version boundaries, vendor suffixes, invalid output, missing Git, cancellation, and help behavior. Integration tests query real Git. End-to-end tests verify rejection before changes, aliases, native commands, DEBUG mode, and moving relative-worktree repositories.

To run the end-to-end checks with an installed older Git binary:

```bash
GIT_WT_OLD_GIT=/absolute/path/to/older/git bats tests/remote_compat.bats
```

Without `GIT_WT_OLD_GIT`, the tests that require older Git skip. CI always supplies it.

## Validation

- Go tests passed with the race detector.
- All 301 end-to-end tests passed with Git 2.48.1 and real Git 2.38.0 for rejection checks.
- Rejection checks also passed with real Git 2.47.0.
- Go vet, staticcheck, workflow lint, and tracked-source Nix formatting checks passed.
- The Nix package build returned exit 0 and its executable ran successfully.
- The 3.1.0 candidate passed version, completion, and man-page checks.
- All four release target builds passed locally with `CGO_ENABLED=0`.
- Nix's packaging audit helper hit a Bash/macOS locale crash. The packaging audit remains incomplete.

## Relative worktrees

Relative worktrees remain enabled. Clone and migration set `worktree.useRelativePaths=true`. Migration always verifies `core.repositoryFormatVersion=1` and `extensions.relativeWorktrees=true`.

The removed `migratePlan.relativeWorktrees` field selected verification rules for older Git. The Git 2.48.0 minimum makes that field unnecessary. The relocation test passed again with real Git 2.48.1.

## Nix audit investigation

Bash 5.3p15 crashes while restoring the locale after Nix's temporary `LANG=C` file-header read. The crash occurs in a nested pipeline child. Its stack enters gettext's `libintl_setlocale`, CoreFoundation preferences, and macOS logging.

A standalone reproduction uses `/bin/ls`, without git-wt or Git. Five of twenty runs returned exit 139. Each run attempted 200 classifier iterations. Setting `LANG=C` or `LC_ALL=C` before starting Bash prevented crashes in three runs each. The interactive Bash package also passed three noninteractive runs. Expected crash output was captured and checked.

The stack matches [NixOS/nixpkgs#570561](https://github.com/NixOS/nixpkgs/issues/570561). Earlier crash reports on this machine show the same stack before these changes.

Nix's audit helper checks its consumer processes but does not check the classifier process's exit status. This explains the successful build exit despite the crash. No audit checks were disabled. No package configuration changed.

The reproduction scripts and logs are in `/tmp/git-wt-nix-audit/` on the investigation machine.

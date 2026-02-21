#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "add: creates worktree with new branch using -b flag" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add -b feature-x feature-x
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/feature-x"
	assert_branch_exists "feature-x"
}

@test "add: creates worktree from remote branch" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	# Create a branch on origin
	command git checkout -b develop --quiet
	create_commit "develop.txt"
	command git push --quiet -u origin develop
	command git checkout main --quiet 2>/dev/null || command git checkout master --quiet

	# Path is relative to current directory
	run "$GIT_WT" add develop origin/develop
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/develop"
}

@test "add: interactive mode creates local branch from remote (not detached HEAD)" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	# Create a branch on origin that only exists remotely
	command git checkout -b remote-only --quiet
	create_commit "remote-only.txt"
	command git push --quiet -u origin remote-only
	command git checkout main --quiet 2>/dev/null || command git checkout master --quiet
	command git branch -D remote-only --quiet

	# Use GIT_WT_SELECT to bypass fzf TUI and select the branch directly
	GIT_WT_SELECT="remote-only" run "$GIT_WT" add
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/remote-only"
	assert_branch_exists "remote-only"
	# Verify it's not detached
	local branch
	branch=$(command git -C "$TEST_DIR/myrepo/remote-only" symbolic-ref --short HEAD)
	[ "$branch" = "remote-only" ]
}

@test "add: interactive mode excludes already checked-out branches" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	# Create two remote branches
	command git checkout -b feature-a --quiet
	create_commit "feature-a.txt"
	command git push --quiet -u origin feature-a
	command git checkout -b feature-b --quiet
	create_commit "feature-b.txt"
	command git push --quiet -u origin feature-b
	command git checkout main --quiet 2>/dev/null || command git checkout master --quiet

	# Delete local branches so they only exist on origin
	command git branch -D feature-a --quiet
	command git branch -D feature-b --quiet

	# Check out feature-a as a worktree (simulates it being already in use)
	command git worktree add -b feature-a feature-a origin/feature-a --quiet

	# feature-a is checked out, so selecting it should cancel (filtered from picker)
	GIT_WT_SELECT="feature-a" run "$GIT_WT" add
	[ "$status" -eq 0 ]
	# Worktree should NOT be re-created (selection was canceled)
	[ ! -d "$TEST_DIR/myrepo/feature-a-copy" ]

	# feature-b is available, so selecting it should succeed
	GIT_WT_SELECT="feature-b" run "$GIT_WT" add
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/feature-b"
	assert_branch_exists "feature-b"
}

@test "add: interactive mode creates new branch" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	# Use GIT_WT_SELECT to bypass fzf TUI and select "Create new branch"
	# Provide branch name and accept default path via stdin (non-TTY falls back to simple IO)
	printf 'my-new-branch\n\n' | GIT_WT_SELECT="__create_new__" "$GIT_WT" add

	assert_branch_exists "my-new-branch"
	assert_worktree_exists "$TEST_DIR/myrepo/my-new-branch"
}

@test "add: creates worktree from remote branch at flat path with -b" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	# Create branch only on origin (not locally)
	(cd "$TEST_DIR/myrepo-origin" && command git branch develop main)
	command git fetch origin --quiet

	run "$GIT_WT" add -b develop develop origin/develop
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/develop"
	# Must not create nested origin/ directory
	[ ! -d "$TEST_DIR/myrepo/origin" ]
}

@test "add: fails when branch already exists" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git branch existing-branch --quiet

	run "$GIT_WT" add -b existing-branch existing
	[ "$status" -ne 0 ]
}

@test "add: succeeds when worktree path is existing empty dir" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	mkdir "$TEST_DIR/myrepo/existing-dir"

	# git worktree add succeeds with an existing empty directory
	run "$GIT_WT" add -b new-branch "$TEST_DIR/myrepo/existing-dir"
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/existing-dir"
}

@test "add: --help shows usage" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" add --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
}

@test "add: succeeds without remote configured" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" add feature-test
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/feature-test"
}

@test "add: handles branch names with slashes" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add -b feature/nested/branch feature-nested
	[ "$status" -eq 0 ]
	assert_branch_exists "feature/nested/branch"
}

@test "add: supports --detach flag" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add --detach detached-wt HEAD
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/detached-wt"
	# Verify it's actually detached
	local head_status
	head_status=$(command git -C "$TEST_DIR/myrepo/detached-wt" symbolic-ref HEAD 2>&1 || true)
	[[ "$head_status" == *"not a symbolic ref"* ]]
}

@test "add: supports -d short flag for detach" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add -d detached-wt HEAD
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/detached-wt"
}

@test "add: supports --quiet flag" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add --quiet -b quiet-branch quiet-wt
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/quiet-wt"
	assert_branch_exists "quiet-branch"
}

@test "add: supports -q short flag for quiet" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add -q -b quiet-branch quiet-wt
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/quiet-wt"
}

@test "add: supports --lock flag" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add --lock -b locked-branch locked-wt
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/locked-wt"
	# Verify it's locked (trying to remove should fail)
	run command git worktree remove "$TEST_DIR/myrepo/locked-wt"
	[ "$status" -ne 0 ]
	[[ "$output" == *"locked"* ]]
}

@test "add: supports --lock with --reason flag" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add --lock --reason "work in progress" -b locked-reason locked-reason-wt
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/locked-reason-wt"
}

@test "add: supports --reason=value syntax" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add --lock --reason="WIP feature" -b locked-eq locked-eq-wt
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/locked-eq-wt"
}

@test "add: supports --force flag" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	# Create a branch that's already checked out
	command git checkout -b force-test --quiet
	command git checkout - --quiet

	# Without --force this would fail if branch is dirty, but with --force it proceeds
	run "$GIT_WT" add --force force-wt force-test
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/force-wt"
}

@test "add: supports -f short flag for force" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	command git checkout -b force-test-short --quiet
	command git checkout - --quiet

	run "$GIT_WT" add -f force-wt-short force-test-short
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/force-wt-short"
}

@test "add: supports --no-checkout flag" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add --no-checkout -b no-checkout-branch no-checkout-wt
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/no-checkout-wt"
	# Directory should exist but be mostly empty (no working tree files)
	[ -d "$TEST_DIR/myrepo/no-checkout-wt" ]
}

@test "add: supports multiple flags combined" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add --quiet --lock -b multi-flag multi-flag-wt
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/multi-flag-wt"
	assert_branch_exists "multi-flag"
}

@test "add: creates worktree at bare root when run from worktree subdirectory" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	# Create a worktree for main, then cd into a subdirectory of it
	command git worktree add main HEAD --quiet 2>/dev/null
	mkdir -p main/src/deep
	cd main/src/deep

	run "$GIT_WT" add -b feature-sub feature-sub
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/feature-sub"
}

@test "add: absolute path works from worktree subdirectory" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	# Create a worktree for main, then cd into a subdirectory of it
	command git worktree add main HEAD --quiet 2>/dev/null
	mkdir -p main/src
	cd main/src

	run "$GIT_WT" add -b feature-abs "$TEST_DIR/myrepo/abs-worktree"
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/abs-worktree"
}

@test "add: works with non-origin remote name" {
	init_bare_repo_with_custom_remote upstream myrepo
	cd myrepo
	# Create a branch only on the remote (not locally)
	(cd "$TEST_DIR/myrepo-upstream" && command git branch feature-up main)
	command git fetch upstream --quiet

	run "$GIT_WT" add feature-up upstream/feature-up
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/feature-up"
}

@test "add: interactive mode works with non-origin remote" {
	init_bare_repo_with_custom_remote upstream myrepo
	cd myrepo
	# Create a branch only on the remote
	command git checkout -b remote-feat --quiet
	create_commit "remote-feat.txt"
	command git push --quiet -u upstream remote-feat
	command git checkout main --quiet 2>/dev/null || command git checkout master --quiet
	command git branch -D remote-feat --quiet

	GIT_WT_SELECT="remote-feat" run "$GIT_WT" add
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/remote-feat"
	assert_branch_exists "remote-feat"
}

@test "add: supports -B flag to reset branch" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	# Create branch first
	command git branch reset-branch --quiet

	# -B should reset it
	run "$GIT_WT" add -B reset-branch reset-wt
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/reset-wt"
	assert_branch_exists "reset-branch"
}

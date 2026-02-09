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

	run "$GIT_WT" add -b feature-x ../feature-x
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/feature-x"
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
	run "$GIT_WT" add ../develop origin/develop
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/develop"
}

@test "add: fails when branch already exists" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git branch existing-branch --quiet

	run "$GIT_WT" add -b existing-branch ../existing
	[ "$status" -ne 0 ]
}

@test "add: succeeds when worktree path is existing empty dir" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	mkdir "$TEST_DIR/existing-dir"

	# git worktree add succeeds with an existing empty directory
	run "$GIT_WT" add -b new-branch "$TEST_DIR/existing-dir"
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/existing-dir"
}

@test "add: --help shows usage" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" add --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
}

@test "add: fails without remote configured" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" add feature-test
	[ "$status" -ne 0 ]
	[[ "$output" == *"origin"* ]]
}

@test "add: handles branch names with slashes" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add -b feature/nested/branch ../feature-nested
	[ "$status" -eq 0 ]
	assert_branch_exists "feature/nested/branch"
}

@test "add: supports --detach flag" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add --detach ../detached-wt HEAD
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/detached-wt"
	# Verify it's actually detached
	local head_status
	head_status=$(command git -C "$TEST_DIR/detached-wt" symbolic-ref HEAD 2>&1 || true)
	[[ "$head_status" == *"not a symbolic ref"* ]]
}

@test "add: supports -d short flag for detach" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add -d ../detached-wt HEAD
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/detached-wt"
}

@test "add: supports --quiet flag" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add --quiet -b quiet-branch ../quiet-wt
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/quiet-wt"
	assert_branch_exists "quiet-branch"
}

@test "add: supports -q short flag for quiet" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add -q -b quiet-branch ../quiet-wt
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/quiet-wt"
}

@test "add: supports --lock flag" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add --lock -b locked-branch ../locked-wt
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/locked-wt"
	# Verify it's locked (trying to remove should fail)
	run command git worktree remove "$TEST_DIR/locked-wt"
	[ "$status" -ne 0 ]
	[[ "$output" == *"locked"* ]]
}

@test "add: supports --lock with --reason flag" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add --lock --reason "work in progress" -b locked-reason ../locked-reason-wt
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/locked-reason-wt"
}

@test "add: supports --reason=value syntax" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add --lock --reason="WIP feature" -b locked-eq ../locked-eq-wt
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/locked-eq-wt"
}

@test "add: supports --force flag" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	# Create a branch that's already checked out
	command git checkout -b force-test --quiet
	command git checkout - --quiet

	# Without --force this would fail if branch is dirty, but with --force it proceeds
	run "$GIT_WT" add --force ../force-wt force-test
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/force-wt"
}

@test "add: supports -f short flag for force" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	command git checkout -b force-test-short --quiet
	command git checkout - --quiet

	run "$GIT_WT" add -f ../force-wt-short force-test-short
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/force-wt-short"
}

@test "add: supports --no-checkout flag" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add --no-checkout -b no-checkout-branch ../no-checkout-wt
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/no-checkout-wt"
	# Directory should exist but be mostly empty (no working tree files)
	[ -d "$TEST_DIR/no-checkout-wt" ]
}

@test "add: supports multiple flags combined" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add --quiet --lock -b multi-flag ../multi-flag-wt
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/multi-flag-wt"
	assert_branch_exists "multi-flag"
}

@test "add: supports -B flag to reset branch" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	# Create branch first
	command git branch reset-branch --quiet

	# -B should reset it
	run "$GIT_WT" add -B reset-branch ../reset-wt
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/reset-wt"
	assert_branch_exists "reset-branch"
}

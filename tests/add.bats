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

@test "add: copies untracked files from default branch worktree" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	# Detach HEAD so we can create a worktree for main
	command git checkout --detach --quiet
	command git worktree add main main --quiet

	# Place untracked config files in the default branch worktree
	echo "SECRET=foo" >"main/.env"
	echo "LOCAL=bar" >"main/.env.local"
	echo "use nix" >"main/.envrc"
	echo "nodejs 20" >"main/.tool-versions"

	run "$GIT_WT" add -b feature feature
	[ "$status" -eq 0 ]
	[ -f "feature/.env" ]
	[ "$(cat feature/.env)" = "SECRET=foo" ]
	[ -f "feature/.env.local" ]
	[ -f "feature/.envrc" ]
	[ -f "feature/.tool-versions" ]
}

@test "add: copies untracked files recursively preserving directory structure" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	command git checkout --detach --quiet
	command git worktree add main main --quiet
	mkdir -p "main/packages/api"
	echo "DB_URL=localhost" >"main/packages/api/.env"
	echo "local_override" >"main/packages/api/.env.local"

	run "$GIT_WT" add -b feature feature
	[ "$status" -eq 0 ]
	[ -f "feature/packages/api/.env" ]
	[ "$(cat feature/packages/api/.env)" = "DB_URL=localhost" ]
	[ -f "feature/packages/api/.env.local" ]
}

@test "add: skips copy when default branch worktree does not exist" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	# No default branch worktree exists — should still succeed
	run "$GIT_WT" add -b feature feature
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/feature"
}

@test "add: --no-untracked-files skips copying" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	command git checkout --detach --quiet
	command git worktree add main main --quiet
	echo "SECRET=foo" >"main/.env"

	run "$GIT_WT" add --no-untracked-files -b feature feature
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/feature"
	[ ! -f "feature/.env" ]
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

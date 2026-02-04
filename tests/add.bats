#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "add: creates worktree with new branch using -b flag" {
	init_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add -b feature-x ../feature-x
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/feature-x"
	assert_branch_exists "feature-x"
}

@test "add: creates worktree from remote branch" {
	init_repo_with_remote myrepo
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
	init_repo_with_remote myrepo
	cd myrepo
	command git branch existing-branch --quiet

	run "$GIT_WT" add -b existing-branch ../existing
	[ "$status" -ne 0 ]
}

@test "add: succeeds when worktree path is existing empty dir" {
	init_repo_with_remote myrepo
	cd myrepo
	mkdir "$TEST_DIR/existing-dir"

	# git worktree add succeeds with an existing empty directory
	run "$GIT_WT" add -b new-branch "$TEST_DIR/existing-dir"
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/existing-dir"
}

@test "add: --help shows usage" {
	init_repo myrepo
	cd myrepo

	run "$GIT_WT" add --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
}

@test "add: fails without remote configured" {
	init_repo myrepo
	cd myrepo

	run "$GIT_WT" add feature-test
	[ "$status" -ne 0 ]
	[[ "$output" == *"origin"* ]]
}

@test "add: handles branch names with slashes" {
	init_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add -b feature/nested/branch ../feature-nested
	[ "$status" -eq 0 ]
	assert_branch_exists "feature/nested/branch"
}

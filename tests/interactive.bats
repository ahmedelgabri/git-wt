#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

# --- switch ---

@test "interactive switch: selects worktree and prints path" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree feature feature

	run env GIT_WT_SELECT="$TEST_DIR/myrepo/feature" "$GIT_WT" switch
	[ "$status" -eq 0 ]
	[[ "$output" == *"$TEST_DIR/myrepo/feature"* ]]
}

# --- add ---

@test "interactive add: selects remote branch and creates worktree" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	# Create branch only on origin (not locally)
	(cd "$TEST_DIR/myrepo-origin" && command git branch develop main)

	run env GIT_WT_SELECT=develop "$GIT_WT" add
	[ "$status" -eq 0 ]
	assert_branch_exists develop
}

@test "interactive add: creates new branch via prompts" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	# Pipe branch name; empty second line uses default worktree path
	echo "new-feature" | env GIT_WT_SELECT="Create new branch" "$GIT_WT" add

	assert_branch_exists new-feature
	assert_worktree_exists "$TEST_DIR/myrepo/new-feature"
}

# --- remove ---

@test "interactive remove: selects and removes worktree" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree to-remove to-remove

	echo "y" | env GIT_WT_SELECT="$TEST_DIR/myrepo/to-remove" "$GIT_WT" remove

	assert_worktree_not_exists "$TEST_DIR/myrepo/to-remove"
	assert_branch_not_exists to-remove
}

@test "interactive remove: dry-run preserves worktree" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree dry-run-int dry-run-int

	run env GIT_WT_SELECT="$TEST_DIR/myrepo/dry-run-int" "$GIT_WT" remove --dry-run
	[ "$status" -eq 0 ]
	[[ "$output" == *"DRY RUN"* ]]
	assert_worktree_exists "$TEST_DIR/myrepo/dry-run-int"
	assert_branch_exists dry-run-int
}

# --- destroy ---

@test "interactive destroy: selects and destroys worktree" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree to-destroy to-destroy

	# Confirm by typing the branch name
	echo "to-destroy" | env GIT_WT_SELECT="$TEST_DIR/myrepo/to-destroy" "$GIT_WT" destroy

	assert_worktree_not_exists "$TEST_DIR/myrepo/to-destroy"
	assert_branch_not_exists to-destroy
}

@test "interactive destroy: dry-run preserves worktree" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree dry-run-dest dry-run-dest

	run env GIT_WT_SELECT="$TEST_DIR/myrepo/dry-run-dest" "$GIT_WT" destroy --dry-run
	[ "$status" -eq 0 ]
	[[ "$output" == *"DRY RUN"* ]]
	assert_worktree_exists "$TEST_DIR/myrepo/dry-run-dest"
	assert_branch_exists dry-run-dest
}

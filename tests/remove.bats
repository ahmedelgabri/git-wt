#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "remove: removes worktree by path and deletes branch" {
	init_repo myrepo
	cd myrepo
	create_worktree ../feature-to-remove feature-to-remove

	run "$GIT_WT" remove "$TEST_DIR/feature-to-remove"
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/feature-to-remove"
	assert_branch_not_exists "feature-to-remove"
}

@test "remove: handles multiple worktrees" {
	init_repo myrepo
	cd myrepo
	create_worktree ../wt-one wt-one
	create_worktree ../wt-two wt-two

	run "$GIT_WT" remove "$TEST_DIR/wt-one" "$TEST_DIR/wt-two"
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/wt-one"
	assert_worktree_not_exists "$TEST_DIR/wt-two"
}

@test "remove: fails for invalid worktree path" {
	init_repo myrepo
	cd myrepo

	run "$GIT_WT" remove "$TEST_DIR/nonexistent"
	[ "$status" -ne 0 ]
}

@test "remove: fails when trying to remove current worktree" {
	init_repo myrepo
	cd myrepo

	run "$GIT_WT" remove "$TEST_DIR/myrepo"
	[ "$status" -ne 0 ]
}

@test "remove: --help shows usage" {
	init_repo myrepo
	cd myrepo

	run "$GIT_WT" remove --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
}

@test "remove: alias 'rm' works" {
	init_repo myrepo
	cd myrepo
	create_worktree ../to-rm to-rm

	run "$GIT_WT" rm "$TEST_DIR/to-rm"
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/to-rm"
}

@test "remove: --dry-run shows what would be removed" {
	init_repo myrepo
	cd myrepo
	create_worktree ../dry-run-test dry-run-test

	run "$GIT_WT" remove --dry-run "$TEST_DIR/dry-run-test"
	[ "$status" -eq 0 ]
	[[ "$output" == *"DRY RUN"* ]] || [[ "$output" == *"dry"* ]]
	# Worktree should still exist
	assert_worktree_exists "$TEST_DIR/dry-run-test"
}

@test "remove: removes worktree with uncommitted changes (force)" {
	init_repo myrepo
	cd myrepo
	create_worktree ../dirty-wt dirty-wt
	echo "uncommitted change" > "$TEST_DIR/dirty-wt/dirty.txt"

	run "$GIT_WT" remove "$TEST_DIR/dirty-wt"
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/dirty-wt"
}

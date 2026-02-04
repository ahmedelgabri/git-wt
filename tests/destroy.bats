#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "destroy: removes worktree and deletes local branch with confirmation" {
	init_repo myrepo
	cd myrepo
	create_worktree ../feature-destroy feature-destroy

	# Destroy requires 'y' confirmation
	echo "y" | "$GIT_WT" destroy "$TEST_DIR/feature-destroy"

	assert_worktree_not_exists "$TEST_DIR/feature-destroy"
	assert_branch_not_exists "feature-destroy"
}

@test "destroy: --dry-run shows what would be destroyed" {
	init_repo myrepo
	cd myrepo
	create_worktree ../dry-run-destroy dry-run-destroy

	run "$GIT_WT" destroy --dry-run "$TEST_DIR/dry-run-destroy"
	[ "$status" -eq 0 ]
	[[ "$output" == *"DRY RUN"* ]]
	# Worktree should still exist
	assert_worktree_exists "$TEST_DIR/dry-run-destroy"
	assert_branch_exists "dry-run-destroy"
}

@test "destroy: -n is alias for --dry-run" {
	init_repo myrepo
	cd myrepo
	create_worktree ../dry-run-n dry-run-n

	run "$GIT_WT" destroy -n "$TEST_DIR/dry-run-n"
	[ "$status" -eq 0 ]
	[[ "$output" == *"DRY RUN"* ]]
	assert_worktree_exists "$TEST_DIR/dry-run-n"
}

@test "destroy: fails for invalid worktree path" {
	init_repo myrepo
	cd myrepo

	run "$GIT_WT" destroy "$TEST_DIR/nonexistent"
	[ "$status" -ne 0 ]
}

@test "destroy: can be cancelled" {
	init_repo myrepo
	cd myrepo
	create_worktree ../cancel-destroy cancel-destroy

	# Answer 'n' to cancel - exits with non-zero but that's expected
	echo "n" | "$GIT_WT" destroy "$TEST_DIR/cancel-destroy" || true

	# Worktree should still exist
	assert_worktree_exists "$TEST_DIR/cancel-destroy"
	assert_branch_exists "cancel-destroy"
}

@test "destroy: --help shows usage" {
	init_repo myrepo
	cd myrepo

	run "$GIT_WT" destroy --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
	[[ "$output" == *"REMOTE"* ]]
}

@test "destroy: attempts to delete remote branch" {
	init_repo_with_remote myrepo
	cd myrepo
	create_worktree ../feature-remote feature-remote
	# Push the branch to origin
	command git push -u origin feature-remote --quiet 2>/dev/null || true

	# Destroy with confirmation
	echo "y" | "$GIT_WT" destroy "$TEST_DIR/feature-remote"

	assert_worktree_not_exists "$TEST_DIR/feature-remote"
	assert_branch_not_exists "feature-remote"
}

@test "destroy: handles multiple worktrees" {
	init_repo myrepo
	cd myrepo
	create_worktree ../destroy-one destroy-one
	create_worktree ../destroy-two destroy-two

	# Destroy multiple requires 'y' confirmation
	echo "y" | "$GIT_WT" destroy "$TEST_DIR/destroy-one" "$TEST_DIR/destroy-two"

	assert_worktree_not_exists "$TEST_DIR/destroy-one"
	assert_worktree_not_exists "$TEST_DIR/destroy-two"
}

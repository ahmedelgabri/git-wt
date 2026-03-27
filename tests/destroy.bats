#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "destroy: compatibility alias removes worktree and deletes local branch" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree feature-destroy feature-destroy

	printf 'feature-destroy\n' | "$GIT_WT" destroy "$TEST_DIR/myrepo/feature-destroy"

	assert_worktree_not_exists "$TEST_DIR/myrepo/feature-destroy"
	assert_branch_not_exists "feature-destroy"
}

@test "destroy: --dry-run shows what would be removed" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree dry-run-destroy dry-run-destroy

	run "$GIT_WT" destroy --dry-run "$TEST_DIR/myrepo/dry-run-destroy"
	[ "$status" -eq 0 ]
	[[ "$output" == *"DRY RUN"* ]]
	[[ "$output" == *"remote branch"* ]]
	assert_worktree_exists "$TEST_DIR/myrepo/dry-run-destroy"
	assert_branch_exists "dry-run-destroy"
}

@test "destroy: -n is alias for --dry-run" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree dry-run-n dry-run-n

	run "$GIT_WT" destroy -n "$TEST_DIR/myrepo/dry-run-n"
	[ "$status" -eq 0 ]
	[[ "$output" == *"DRY RUN"* ]]
	assert_worktree_exists "$TEST_DIR/myrepo/dry-run-n"
}

@test "destroy: fails for invalid worktree path" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" destroy "$TEST_DIR/nonexistent"
	[ "$status" -ne 0 ]
}

@test "destroy: can be cancelled" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree cancel-destroy cancel-destroy

	printf 'nope\n' | "$GIT_WT" destroy "$TEST_DIR/myrepo/cancel-destroy" || true

	assert_worktree_exists "$TEST_DIR/myrepo/cancel-destroy"
	assert_branch_exists "cancel-destroy"
}

@test "destroy: --help forwards to remove help" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" destroy --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"remove [<worktree>...]"* ]]
	[[ "$output" == *"--delete-remote"* ]]
	[[ "$output" != *"Compatibility alias"* ]]
}

@test "destroy: compatibility alias deletes remote branch" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	create_worktree feature-remote feature-remote
	command git push -u origin feature-remote --quiet 2>/dev/null || true

	printf 'feature-remote\n' | "$GIT_WT" destroy "$TEST_DIR/myrepo/feature-remote"

	assert_worktree_not_exists "$TEST_DIR/myrepo/feature-remote"
	assert_branch_not_exists "feature-remote"
}

@test "destroy: compatibility alias handles multiple worktrees" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree destroy-one destroy-one
	create_worktree destroy-two destroy-two

	printf 'remove\n' | "$GIT_WT" destroy "$TEST_DIR/myrepo/destroy-one" "$TEST_DIR/myrepo/destroy-two"

	assert_worktree_not_exists "$TEST_DIR/myrepo/destroy-one"
	assert_worktree_not_exists "$TEST_DIR/myrepo/destroy-two"
}

@test "destroy: compatibility alias deletes remote branch with non-origin remote" {
	init_bare_repo_with_custom_remote upstream myrepo
	cd myrepo
	create_worktree feature-up feature-up
	command git push -u upstream feature-up --quiet 2>/dev/null || true

	printf 'feature-up\n' | "$GIT_WT" destroy "$TEST_DIR/myrepo/feature-up"

	assert_worktree_not_exists "$TEST_DIR/myrepo/feature-up"
	assert_branch_not_exists "feature-up"
}

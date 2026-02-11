#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "remove: removes worktree by path and deletes branch" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree feature-to-remove feature-to-remove

	run "$GIT_WT" remove "$TEST_DIR/myrepo/feature-to-remove"
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/myrepo/feature-to-remove"
	assert_branch_not_exists "feature-to-remove"
}

@test "remove: handles multiple worktrees" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree wt-one wt-one
	create_worktree wt-two wt-two

	run "$GIT_WT" remove "$TEST_DIR/myrepo/wt-one" "$TEST_DIR/myrepo/wt-two"
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/myrepo/wt-one"
	assert_worktree_not_exists "$TEST_DIR/myrepo/wt-two"
}

@test "remove: fails for invalid worktree path" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" remove "$TEST_DIR/nonexistent"
	[ "$status" -ne 0 ]
}

@test "remove: fails when trying to remove current worktree" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" remove "$TEST_DIR/myrepo"
	[ "$status" -ne 0 ]
}

@test "remove: --help shows usage" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" remove --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
}

@test "remove: alias 'rm' works" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree to-rm to-rm

	run "$GIT_WT" rm "$TEST_DIR/myrepo/to-rm"
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/myrepo/to-rm"
}

@test "remove: --dry-run shows what would be removed" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree dry-run-test dry-run-test

	run "$GIT_WT" remove --dry-run "$TEST_DIR/myrepo/dry-run-test"
	[ "$status" -eq 0 ]
	[[ "$output" == *"DRY RUN"* ]] || [[ "$output" == *"dry"* ]]
	# Worktree should still exist
	assert_worktree_exists "$TEST_DIR/myrepo/dry-run-test"
}

@test "remove: removes worktree with uncommitted changes (force)" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree dirty-wt dirty-wt
	echo "uncommitted change" > "$TEST_DIR/myrepo/dirty-wt/dirty.txt"

	run "$GIT_WT" remove "$TEST_DIR/myrepo/dirty-wt"
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/myrepo/dirty-wt"
}

@test "remove: resolves worktree by workspace name" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree bex-1697 bex-1697

	run "$GIT_WT" remove bex-1697
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/myrepo/bex-1697"
	assert_branch_not_exists "bex-1697"
}

@test "remove: resolves worktree by relative path" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree rel-path-test rel-path-test

	run "$GIT_WT" remove ./rel-path-test
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/myrepo/rel-path-test"
	assert_branch_not_exists "rel-path-test"
}

@test "remove: resolves multiple worktrees by name" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree name-one name-one
	create_worktree name-two name-two

	run "$GIT_WT" remove name-one name-two
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/myrepo/name-one"
	assert_worktree_not_exists "$TEST_DIR/myrepo/name-two"
}

@test "remove: invalid name lists available worktrees" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree some-wt some-wt

	run "$GIT_WT" remove no-such-wt
	[ "$status" -ne 0 ]
	[[ "$output" == *"Available worktrees:"* ]]
	[[ "$output" == *"some-wt"* ]]
}

@test "remove: works from worktree subdirectory" {
	init_bare_repo myrepo
	cd myrepo
	command git worktree add main HEAD --quiet 2>/dev/null
	create_worktree to-remove to-remove
	mkdir -p main/src
	cd main/src

	run "$GIT_WT" remove to-remove
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/myrepo/to-remove"
	assert_branch_not_exists "to-remove"
}

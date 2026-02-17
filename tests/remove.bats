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

@test "remove: invalid name lists available nested worktrees with full relative path" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree feature/nested feature/nested

	run "$GIT_WT" remove no-such-wt
	[ "$status" -ne 0 ]
	[[ "$output" == *"Available worktrees:"* ]]
	[[ "$output" == *"feature/nested"* ]]
}

@test "remove: resolves worktree with slash-containing path by name" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree feature/my-thing feature/my-thing

	run "$GIT_WT" remove feature/my-thing
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/myrepo/feature/my-thing"
	assert_branch_not_exists "feature/my-thing"
}

@test "remove: resolves worktree with slash-containing path by full path" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree feature/another feature/another

	run "$GIT_WT" remove "$TEST_DIR/myrepo/feature/another"
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/myrepo/feature/another"
	assert_branch_not_exists "feature/another"
}

@test "remove: resolves worktree with slash-containing path from another worktree" {
	init_bare_repo myrepo
	cd myrepo
	command git worktree add main HEAD --quiet 2>/dev/null
	create_worktree feature/nested feature/nested
	cd main

	run "$GIT_WT" remove feature/nested
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/myrepo/feature/nested"
	assert_branch_not_exists "feature/nested"
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

@test "remove: interactive mode removes selected worktree" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree int-remove int-remove

	# Create a fake fzf that outputs in the display format
	local wt_path="$TEST_DIR/myrepo/int-remove"
	mkdir -p "$TEST_DIR/bin"
	printf '#!/usr/bin/env bash\nprintf "int-remove [int-remove]\\t%s\\n" "%s"\n' "$wt_path" >"$TEST_DIR/bin/fzf"
	chmod +x "$TEST_DIR/bin/fzf"

	# Confirm removal via stdin
	echo "y" | PATH="$TEST_DIR/bin:$PATH" "$GIT_WT" remove

	assert_worktree_not_exists "$TEST_DIR/myrepo/int-remove"
	assert_branch_not_exists "int-remove"
}

@test "remove: interactive mode dry-run preserves worktree" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree dry-int dry-int

	# Create a fake fzf that outputs in the display format
	local wt_path="$TEST_DIR/myrepo/dry-int"
	mkdir -p "$TEST_DIR/bin"
	printf '#!/usr/bin/env bash\nprintf "dry-int [dry-int]\\t%s\\n" "%s"\n' "$wt_path" >"$TEST_DIR/bin/fzf"
	chmod +x "$TEST_DIR/bin/fzf"

	PATH="$TEST_DIR/bin:$PATH" run "$GIT_WT" remove --dry-run
	[ "$status" -eq 0 ]
	[[ "$output" == *"DRY RUN"* ]]
	assert_worktree_exists "$TEST_DIR/myrepo/dry-int"
}

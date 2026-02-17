#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "destroy: removes worktree and deletes local branch with confirmation" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree feature-destroy feature-destroy

	# Destroy requires 'y' confirmation
	echo "y" | "$GIT_WT" destroy "$TEST_DIR/myrepo/feature-destroy"

	assert_worktree_not_exists "$TEST_DIR/myrepo/feature-destroy"
	assert_branch_not_exists "feature-destroy"
}

@test "destroy: --dry-run shows what would be destroyed" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree dry-run-destroy dry-run-destroy

	run "$GIT_WT" destroy --dry-run "$TEST_DIR/myrepo/dry-run-destroy"
	[ "$status" -eq 0 ]
	[[ "$output" == *"DRY RUN"* ]]
	# Worktree should still exist
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

	# Answer 'n' to cancel - exits with non-zero but that's expected
	echo "n" | "$GIT_WT" destroy "$TEST_DIR/myrepo/cancel-destroy" || true

	# Worktree should still exist
	assert_worktree_exists "$TEST_DIR/myrepo/cancel-destroy"
	assert_branch_exists "cancel-destroy"
}

@test "destroy: --help shows usage" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" destroy --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
	[[ "$output" == *"REMOTE"* ]]
}

@test "destroy: attempts to delete remote branch" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	create_worktree feature-remote feature-remote
	# Push the branch to origin
	command git push -u origin feature-remote --quiet 2>/dev/null || true

	# Destroy with confirmation
	echo "y" | "$GIT_WT" destroy "$TEST_DIR/myrepo/feature-remote"

	assert_worktree_not_exists "$TEST_DIR/myrepo/feature-remote"
	assert_branch_not_exists "feature-remote"
}

@test "destroy: handles multiple worktrees" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree destroy-one destroy-one
	create_worktree destroy-two destroy-two

	# Destroy multiple requires 'y' confirmation
	echo "y" | "$GIT_WT" destroy "$TEST_DIR/myrepo/destroy-one" "$TEST_DIR/myrepo/destroy-two"

	assert_worktree_not_exists "$TEST_DIR/myrepo/destroy-one"
	assert_worktree_not_exists "$TEST_DIR/myrepo/destroy-two"
}

@test "destroy: resolves worktree by workspace name" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree destroy-by-name destroy-by-name

	echo "y" | "$GIT_WT" destroy destroy-by-name

	assert_worktree_not_exists "$TEST_DIR/myrepo/destroy-by-name"
	assert_branch_not_exists "destroy-by-name"
}

@test "destroy: resolves worktree by relative path" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree destroy-rel destroy-rel

	echo "y" | "$GIT_WT" destroy ./destroy-rel

	assert_worktree_not_exists "$TEST_DIR/myrepo/destroy-rel"
	assert_branch_not_exists "destroy-rel"
}

@test "destroy: resolves multiple worktrees by name" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree dest-name-one dest-name-one
	create_worktree dest-name-two dest-name-two

	echo "y" | "$GIT_WT" destroy dest-name-one dest-name-two

	assert_worktree_not_exists "$TEST_DIR/myrepo/dest-name-one"
	assert_worktree_not_exists "$TEST_DIR/myrepo/dest-name-two"
}

@test "destroy: invalid name lists available worktrees" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree some-dest some-dest

	run "$GIT_WT" destroy no-such-wt
	[ "$status" -ne 0 ]
	[[ "$output" == *"Available worktrees:"* ]]
	[[ "$output" == *"some-dest"* ]]
}

@test "destroy: resolves worktree with slash-containing path from another worktree" {
	init_bare_repo myrepo
	cd myrepo
	command git worktree add main HEAD --quiet 2>/dev/null
	create_worktree feature/to-destroy feature/to-destroy
	cd main

	echo "y" | "$GIT_WT" destroy feature/to-destroy

	assert_worktree_not_exists "$TEST_DIR/myrepo/feature/to-destroy"
	assert_branch_not_exists "feature/to-destroy"
}

@test "destroy: works from worktree subdirectory" {
	init_bare_repo myrepo
	cd myrepo
	command git worktree add main HEAD --quiet 2>/dev/null
	create_worktree to-destroy to-destroy
	mkdir -p main/src
	cd main/src

	echo "y" | "$GIT_WT" destroy to-destroy

	assert_worktree_not_exists "$TEST_DIR/myrepo/to-destroy"
	assert_branch_not_exists "to-destroy"
}

@test "destroy: interactive mode destroys selected worktree" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree int-destroy int-destroy

	# Create a fake fzf that outputs in the display format
	local wt_path="$TEST_DIR/myrepo/int-destroy"
	mkdir -p "$TEST_DIR/bin"
	printf '#!/usr/bin/env bash\nprintf "int-destroy [int-destroy]\\t%s\\n" "%s"\n' "$wt_path" >"$TEST_DIR/bin/fzf"
	chmod +x "$TEST_DIR/bin/fzf"

	# Single worktree destroy requires typing the branch name to confirm
	echo "int-destroy" | PATH="$TEST_DIR/bin:$PATH" "$GIT_WT" destroy

	assert_worktree_not_exists "$TEST_DIR/myrepo/int-destroy"
	assert_branch_not_exists "int-destroy"
}

@test "destroy: interactive mode dry-run preserves worktree" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree dry-dest dry-dest

	# Create a fake fzf that outputs in the display format
	local wt_path="$TEST_DIR/myrepo/dry-dest"
	mkdir -p "$TEST_DIR/bin"
	printf '#!/usr/bin/env bash\nprintf "dry-dest [dry-dest]\\t%s\\n" "%s"\n' "$wt_path" >"$TEST_DIR/bin/fzf"
	chmod +x "$TEST_DIR/bin/fzf"

	PATH="$TEST_DIR/bin:$PATH" run "$GIT_WT" destroy --dry-run
	[ "$status" -eq 0 ]
	[[ "$output" == *"DRY RUN"* ]]
	assert_worktree_exists "$TEST_DIR/myrepo/dry-dest"
}

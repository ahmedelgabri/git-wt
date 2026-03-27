#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "status: shows worktrees with clean and dirty states" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	create_worktree feature-status feature-status
	echo "dirty" > "$TEST_DIR/myrepo/feature-status/dirty.txt"

	run "$GIT_WT" status
	[ "$status" -eq 0 ]
	[[ "$output" != *"Worktree status"* ]]
	[[ "$output" == *"WORKTREE"* ]]
	[[ "$output" == *"feature-status"* ]]
	[[ "$output" == *"dirty"* ]]
}

@test "status: shows detached head worktrees" {
	init_bare_repo myrepo
	cd myrepo
	local sha
	sha=$(command git rev-parse HEAD)
	command git worktree add --detach detached-status "$sha" --quiet

	run "$GIT_WT" status
	[ "$status" -eq 0 ]
	[[ "$output" == *"detached HEAD"* ]]
	[[ "$output" == *"detached-status"* ]]
}

@test "status: counts worktree errors separately from clean worktrees" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree stale-status stale-status
	rm -rf "$TEST_DIR/myrepo/stale-status"

	run "$GIT_WT" status
	[ "$status" -eq 0 ]
	[[ "$output" == *"● error"* ]]
	[[ "$output" == *"0 clean"* ]]
	[[ "$output" == *"1 error"* ]]
}

@test "status: fails outside git repo" {
	run "$GIT_WT" status
	[ "$status" -ne 0 ]
}

@test "status: does not truncate long values" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	long_branch="feature-this-is-a-very-long-branch-name-for-status-output"
	create_worktree "$long_branch" "$long_branch"

	run "$GIT_WT" status
	[ "$status" -eq 0 ]
	[[ "$output" == *"$long_branch"* ]]
	[[ "$output" == *"./$long_branch"* ]]
}

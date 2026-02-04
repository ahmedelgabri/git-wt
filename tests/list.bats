#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "list: shows single worktree in standard repo" {
	init_repo myrepo
	cd myrepo

	run "$GIT_WT" list
	[ "$status" -eq 0 ]
	[[ "$output" == *"myrepo"* ]]
	[[ "$output" == *"[main]"* ]] || [[ "$output" == *"[master]"* ]]
}

@test "list: shows multiple worktrees" {
	init_repo myrepo
	cd myrepo
	create_worktree ../feature-a feature-a
	create_worktree ../feature-b feature-b

	run "$GIT_WT" list
	[ "$status" -eq 0 ]
	[[ "$output" == *"myrepo"* ]]
	[[ "$output" == *"feature-a"* ]]
	[[ "$output" == *"feature-b"* ]]
}

@test "list: shows bare directory in bare repo setup" {
	# Use git-wt clone to create a proper bare repo structure
	init_repo source-repo
	cd source-repo
	create_commit "file.txt"
	cd "$TEST_DIR"

	run "$GIT_WT" clone "$TEST_DIR/source-repo" bare-test
	[ "$status" -eq 0 ]

	cd bare-test
	run "$GIT_WT" list
	[ "$status" -eq 0 ]
	# list is a passthrough to git worktree list, so .bare shows up
	[[ "$output" == *".bare"* ]]
	[[ "$output" == *"main"* ]]
}

@test "list: shows detached HEAD worktrees" {
	init_repo myrepo
	cd myrepo
	local sha
	sha=$(command git rev-parse HEAD)
	command git worktree add --detach ../detached "$sha" --quiet

	run "$GIT_WT" list
	[ "$status" -eq 0 ]
	[[ "$output" == *"detached"* ]]
}

@test "list: fails outside git repo" {
	run "$GIT_WT" list
	[ "$status" -ne 0 ]
}

@test "list: --help shows usage" {
	init_repo myrepo
	cd myrepo

	run "$GIT_WT" list --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
}

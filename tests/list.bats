#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "list: passes through native output" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree feature-a feature-a
	create_worktree feature-b feature-b

	run command git worktree list
	[ "$status" -eq 0 ]
	expected="$output"

	run "$GIT_WT" list
	[ "$status" -eq 0 ]
	[ "$output" = "$expected" ]
}

@test "list: passes through detached worktrees" {
	init_bare_repo myrepo
	cd myrepo
	local sha
	sha=$(command git rev-parse HEAD)
	command git worktree add --detach detached "$sha" --quiet

	run command git worktree list
	[ "$status" -eq 0 ]
	expected="$output"

	run "$GIT_WT" list
	[ "$status" -eq 0 ]
	[ "$output" = "$expected" ]
}

@test "list: fails outside git repo" {
	run "$GIT_WT" list
	[ "$status" -ne 0 ]
}

@test "list: --help shows passthrough usage" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" list --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Pass-through to git worktree list"* ]]
}

@test "list: works from worktree subdirectory" {
	init_bare_repo myrepo
	cd myrepo
	command git worktree add main HEAD --quiet 2>/dev/null
	create_worktree feature-list feature-list
	mkdir -p main/src
	cd main/src

	run command git worktree list
	[ "$status" -eq 0 ]
	expected="$output"

	run "$GIT_WT" list
	[ "$status" -eq 0 ]
	[ "$output" = "$expected" ]
}

@test "list: still lists worktrees in DEBUG mode" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree feature-debug feature-debug

	run command git worktree list
	[ "$status" -eq 0 ]
	expected="$output"

	run env DEBUG=1 "$GIT_WT" list
	[ "$status" -eq 0 ]
	[ "$output" = "$expected" ]
}

@test "list: passes through native flags" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree feature-porcelain feature-porcelain

	run command git worktree list --porcelain
	[ "$status" -eq 0 ]
	expected="$output"

	run "$GIT_WT" list --porcelain
	[ "$status" -eq 0 ]
	[ "$output" = "$expected" ]
}

@test "list: old --json flag is no longer supported" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" list --json
	[ "$status" -ne 0 ]
	[[ "$output" == *"unknown option"* ]]
	[[ "$output" == *"json"* ]]
	[[ "$output" == *"usage: git worktree list"* ]]
}

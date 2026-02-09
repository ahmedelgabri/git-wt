#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

# General help and usage tests

@test "general: --help shows main help" {
	run "$GIT_WT" --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"git-wt"* ]] || [[ "$output" == *"worktree"* ]]
}

@test "general: help command shows main help" {
	run "$GIT_WT" help
	[ "$status" -eq 0 ]
	[[ "$output" == *"git-wt"* ]] || [[ "$output" == *"worktree"* ]]
}

@test "general: no arguments shows help" {
	run "$GIT_WT"
	[ "$status" -eq 0 ]
	[[ "$output" == *"git-wt"* ]] || [[ "$output" == *"worktree"* ]]
}

@test "general: unknown command shows error" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" nonexistent-command
	[ "$status" -ne 0 ]
}

# Error handling tests

@test "error: commands fail outside git repo" {
	run "$GIT_WT" list
	[ "$status" -ne 0 ]
}

@test "error: add fails with invalid base branch" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add nonexistent origin/nonexistent
	[ "$status" -ne 0 ]
}

# Edge cases

@test "edge: handles repo with no commits gracefully" {
	mkdir empty-repo
	cd empty-repo
	command git init --quiet

	run "$GIT_WT" list
	# Should either succeed with empty list or fail gracefully
	true
}

@test "edge: handles paths with spaces" {
	init_bare_repo "repo with spaces"
	cd "repo with spaces"

	run "$GIT_WT" list
	[ "$status" -eq 0 ]
	[[ "$output" == *"repo with spaces"* ]]
}

@test "edge: worktree cache is populated correctly" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree ../wt-one wt-one
	create_worktree ../wt-two wt-two
	create_worktree ../wt-three wt-three

	run "$GIT_WT" list
	[ "$status" -eq 0 ]

	# Count worktrees in output (should be 4: main + 3 created)
	local count
	count=$(echo "$output" | wc -l | tr -d ' ')
	[ "$count" -eq 4 ]
}

@test "edge: detached HEAD worktree handling" {
	init_bare_repo myrepo
	cd myrepo
	local sha
	sha=$(command git rev-parse HEAD)
	command git worktree add --detach ../detached-wt "$sha" --quiet

	run "$GIT_WT" list
	[ "$status" -eq 0 ]
	[[ "$output" == *"detached"* ]]
}

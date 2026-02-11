#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "update: --help shows usage" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" update --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
	[[ "$output" == *"Fetch"* ]]
}

@test "update: alias 'u' works with --help" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" u --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
}

@test "update: fails gracefully without remote" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" update
	# Should fail or warn when no remote configured
	# The exact behavior depends on implementation
	[[ "$status" -ne 0 ]] || [[ "$output" == *"remote"* ]] || [[ "$output" == *"error"* ]] || [[ "$output" == *"Error"* ]]
}

@test "update: works with remote configured" {
	# Create a "remote" repo
	init_repo remote-repo
	cd "$TEST_DIR"

	# Clone it
	command git clone --quiet remote-repo local-repo
	cd local-repo

	run "$GIT_WT" update
	[ "$status" -eq 0 ]
}

@test "update: works from worktree subdirectory" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	# Use absolute path for remote (relative paths break from subdirs)
	command git remote set-url origin "$TEST_DIR/myrepo-origin"
	# Detach .bare HEAD so main branch can be checked out in a worktree
	command git checkout --detach --quiet
	command git worktree add main main --quiet 2>/dev/null
	mkdir -p main/src
	cd main/src

	run "$GIT_WT" update
	[ "$status" -eq 0 ]
}

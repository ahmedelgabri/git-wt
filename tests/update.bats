#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "update: --help shows usage" {
	init_repo myrepo
	cd myrepo

	run "$GIT_WT" update --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
	[[ "$output" == *"Fetch"* ]]
}

@test "update: alias 'u' works with --help" {
	init_repo myrepo
	cd myrepo

	run "$GIT_WT" u --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
}

@test "update: fails gracefully without remote" {
	init_repo myrepo
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

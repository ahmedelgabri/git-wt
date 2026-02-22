#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "switch: --help shows usage" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" switch --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
}

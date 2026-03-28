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

@test "switch: keeps stdout empty when no worktrees are available" {
	init_bare_repo myrepo
	cd myrepo

	local stdout_file stderr_file stdout_text stderr_text
	stdout_file="$TEST_DIR/stdout.txt"
	stderr_file="$TEST_DIR/stderr.txt"

	"$GIT_WT" switch >"$stdout_file" 2>"$stderr_file"

	stdout_text=$(cat "$stdout_file")
	stderr_text=$(cat "$stderr_file")
	[ -z "$stdout_text" ]
	[[ "$stderr_text" == *"No worktrees available"* ]]
}

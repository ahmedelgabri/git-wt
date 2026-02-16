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

@test "switch: interactive mode outputs selected worktree path" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree switch-target switch-target

	# Create a fake fzf that outputs in the display format
	local wt_path="$TEST_DIR/myrepo/switch-target"
	mkdir -p "$TEST_DIR/bin"
	printf '#!/usr/bin/env bash\nprintf "switch-target [switch-target]\\t%s\\n" "%s"\n' "$wt_path" >"$TEST_DIR/bin/fzf"
	chmod +x "$TEST_DIR/bin/fzf"

	PATH="$TEST_DIR/bin:$PATH" run "$GIT_WT" switch
	[ "$status" -eq 0 ]
	[[ "$output" == *"$TEST_DIR/myrepo/switch-target"* ]]
}

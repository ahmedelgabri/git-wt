#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
	init_bare_repo_with_remote repo
	cd repo
	command git --git-dir="$TEST_DIR/repo-origin" branch feature main
}

teardown() { teardown_test_env; }

@test "TTY: Ctrl-C cancels path input without creating a worktree" {
	run python3 "$BATS_TEST_DIRNAME/tty_cancel.py" "$GIT_WT" path ctrl-c
	[ "$status" -eq 0 ]
}

@test "TTY: Escape cancels path input without accepting its default" {
	run python3 "$BATS_TEST_DIRNAME/tty_cancel.py" "$GIT_WT" path escape
	[ "$status" -eq 0 ]
}

@test "TTY: Ctrl-C cancels the real fzf picker" {
	run python3 "$BATS_TEST_DIRNAME/tty_cancel.py" "$GIT_WT" picker ctrl-c
	[ "$status" -eq 0 ]
}

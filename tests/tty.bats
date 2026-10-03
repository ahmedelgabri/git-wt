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

tty_remove_fixture() {
	create_worktree clean clean
	create_worktree dirty dirty
	echo local >dirty/untracked.txt
}

@test "TTY: typing the name discards a dirty worktree in a multi-target removal" {
	tty_remove_fixture
	run python3 "$BATS_TEST_DIRNAME/tty_answer.py" "$GIT_WT" remove clean dirty -- "Type dirty to discard" 'dirty\r' "[y/N]" y
	[ "$status" -eq 0 ]
	[[ "$output" == *"dirty has work that removal would discard"* ]]
	[[ "$output" == *"contains local files or changes"* ]]
	[ ! -e clean ]
	[ ! -e dirty ]
	assert_branch_not_exists dirty
}

@test "TTY: Enter skips a dirty worktree and removes the rest" {
	tty_remove_fixture
	run python3 "$BATS_TEST_DIRNAME/tty_answer.py" "$GIT_WT" remove clean dirty -- "Type dirty to discard" '\r' "[y/N]" y
	[ "$status" -eq 0 ]
	[[ "$output" == *"Skipped ./dirty"* ]]
	[ ! -e clean ]
	[ -f dirty/untracked.txt ]
	assert_branch_exists dirty
}

tty_migration_fixture() {
	cd "$TEST_DIR"
	init_repo standard
	cd standard
	mkdir "$TEST_DIR/bin"
	# Slow down one step so the interruption arrives mid-migration.
	cat >"$TEST_DIR/bin/git" <<SH
#!/bin/sh
case " \$* " in *" worktree add "*) sleep 2 ;; esac
exec $(command -v git) "\$@"
SH
	chmod +x "$TEST_DIR/bin/git"
}

assert_migration_rolled_back() {
	[ "$status" -ne 0 ]
	[[ "$output" == *"original repository restored"* ]]
	[ -d .git ]
	[ ! -e .bare ]
	[ ! -e .git-wt-migrate ]
	[ "$(command git status --porcelain)" = "" ]
}

@test "TTY: Ctrl-C during migration rolls back" {
	tty_migration_fixture
	run env PATH="$TEST_DIR/bin:$PATH" python3 "$BATS_TEST_DIRNAME/tty_answer.py" "$GIT_WT" migrate -- "[y/N]" y "Migrating repository in place" '\x03'
	assert_migration_rolled_back
}

@test "TTY: SIGTERM and SIGINT during migration roll back" {
	for sig in SIGTERM SIGINT; do
		rm -rf "$TEST_DIR/standard" "$TEST_DIR/bin"
		tty_migration_fixture
		run env PATH="$TEST_DIR/bin:$PATH" python3 "$BATS_TEST_DIRNAME/tty_answer.py" "$GIT_WT" migrate -- "[y/N]" y "Migrating repository in place" "signal:$sig"
		assert_migration_rolled_back
	done
}

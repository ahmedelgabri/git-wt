#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

# --- Post-creation hooks (wt.hook) ---

@test "hooks: wt.hook runs after creating worktree" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git config --add wt.hook 'touch .hook-ran'

	run "$GIT_WT" add -b hook-test hook-test
	[ "$status" -eq 0 ]
	[ -f "$TEST_DIR/myrepo/hook-test/.hook-ran" ]
}

@test "hooks: wt.hook runs in the new worktree directory" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git config --add wt.hook 'pwd > .hook-pwd'

	"$GIT_WT" add -b hook-dir hook-dir
	local hook_pwd
	hook_pwd=$(cat "$TEST_DIR/myrepo/hook-dir/.hook-pwd")
	[ "$hook_pwd" = "$TEST_DIR/myrepo/hook-dir" ]
}

@test "hooks: multiple wt.hook values run sequentially" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git config --add wt.hook 'echo first >> .hook-order'
	command git config --add wt.hook 'echo second >> .hook-order'

	"$GIT_WT" add -b multi-hook multi-hook

	local contents
	contents=$(cat "$TEST_DIR/myrepo/multi-hook/.hook-order")
	[[ "$contents" == *"first"* ]]
	[[ "$contents" == *"second"* ]]
	# Verify order
	[ "$(head -1 "$TEST_DIR/myrepo/multi-hook/.hook-order")" = "first" ]
}

@test "hooks: wt.hook failure stops execution and returns error" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git config --add wt.hook 'exit 1'
	command git config --add wt.hook 'touch .second-ran'

	run "$GIT_WT" add -b fail-hook fail-hook
	[ "$status" -ne 0 ]
	# Second hook should not have run
	[ ! -f "$TEST_DIR/myrepo/fail-hook/.second-ran" ]
}

@test "hooks: wt.hook does not run when no hooks configured" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add -b no-hook no-hook
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/no-hook"
}

@test "hooks: wt.hook output goes to stderr" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git config --add wt.hook 'echo hook-output'

	local stdout_file stderr_file
	stdout_file="$TEST_DIR/stdout.txt"
	stderr_file="$TEST_DIR/stderr.txt"

	"$GIT_WT" add -b stderr-hook stderr-hook >"$stdout_file" 2>"$stderr_file"

	# stdout should only have the path
	[[ "$(cat "$stdout_file")" == *"stderr-hook"* ]]
	# stderr should contain hook output
	[[ "$(cat "$stderr_file")" == *"hook-output"* ]]
}

@test "hooks: wt.hook runs in interactive mode" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git config --add wt.hook 'touch .hook-ran'
	command git checkout -b interactive-hook --quiet
	create_commit "interactive-hook.txt"
	command git push --quiet -u origin interactive-hook
	command git checkout main --quiet 2>/dev/null || command git checkout master --quiet
	command git branch -D interactive-hook --quiet

	run env GIT_WT_SELECT="origin/interactive-hook" "$GIT_WT" add
	[ "$status" -eq 0 ]
	[ -f "$TEST_DIR/myrepo/interactive-hook/.hook-ran" ]
}

# --- Delete hooks (wt.deletehook) ---

@test "hooks: wt.deletehook runs before removing worktree" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree del-hook del-hook
	command git config --add wt.deletehook 'touch "$TEST_DIR/delete-hook-ran"'

	run bash -c 'printf "y\n" | "$1" remove "$2"' _ "$GIT_WT" "$TEST_DIR/myrepo/del-hook"
	[ "$status" -eq 0 ]
	[ -f "$TEST_DIR/delete-hook-ran" ]
	assert_worktree_not_exists "$TEST_DIR/myrepo/del-hook"
}

@test "hooks: wt.deletehook runs in the worktree directory" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree del-dir del-dir
	command git config --add wt.deletehook 'pwd > "$TEST_DIR/delete-hook-pwd"'

	run bash -c 'printf "y\n" | "$1" remove "$2"' _ "$GIT_WT" "$TEST_DIR/myrepo/del-dir"
	[ "$status" -eq 0 ]
	local hook_pwd
	hook_pwd=$(cat "$TEST_DIR/delete-hook-pwd")
	[ "$hook_pwd" = "$TEST_DIR/myrepo/del-dir" ]
}

@test "hooks: wt.deletehook failure prevents worktree removal" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree del-fail del-fail
	command git config --add wt.deletehook 'exit 1'

	run bash -c 'printf "y\n" | "$1" remove "$2"' _ "$GIT_WT" "$TEST_DIR/myrepo/del-fail"
	[ "$status" -ne 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/del-fail"
}

@test "hooks: multiple wt.deletehook values run sequentially" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree del-multi del-multi
	command git config --add wt.deletehook 'echo first >> "$TEST_DIR/del-hook-order"'
	command git config --add wt.deletehook 'echo second >> "$TEST_DIR/del-hook-order"'

	run bash -c 'printf "y\n" | "$1" remove "$2"' _ "$GIT_WT" "$TEST_DIR/myrepo/del-multi"
	[ "$status" -eq 0 ]
	[ "$(head -1 "$TEST_DIR/del-hook-order")" = "first" ]
	[ "$(tail -1 "$TEST_DIR/del-hook-order")" = "second" ]
}

@test "hooks: wt.deletehook does not run for stale prune" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree stale-prune stale-prune
	command git config --add wt.deletehook 'touch "$TEST_DIR/prune-hook-ran"'

	# Manually remove the worktree directory to make it stale
	rm -rf "$TEST_DIR/myrepo/stale-prune"

	run "$GIT_WT" remove --stale
	# Hook should NOT have run (prune path doesn't call removeSingleWorktree)
	[ ! -f "$TEST_DIR/prune-hook-ran" ]
}

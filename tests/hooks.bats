#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

# --- Add lifecycle hooks ---

@test "hooks: wt.beforeadd runs from bare root before creating worktree" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git config --add wt.beforeadd '[ ! -e "$GIT_WT_PATH" ] && printf "%s\\n%s\\n%s\\n%s\\n" "$PWD" "$GIT_WT_EVENT" "$GIT_WT_BRANCH" "$GIT_WT_PATH" > .beforeadd-context'

	run "$GIT_WT" add -b before-add before-add
	[ "$status" -eq 0 ]
	local context
	context=$(cat "$TEST_DIR/myrepo/.beforeadd-context")
	[[ "$context" == "$TEST_DIR/myrepo"$'\n'"beforeadd"$'\n'"before-add"$'\n'"$TEST_DIR/myrepo/before-add" ]]
}

@test "hooks: wt.beforeadd failure prevents worktree creation and later hooks" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git config --add wt.beforeadd 'exit 1'
	command git config --add wt.beforeadd 'touch .second-ran'
	command git config --add wt.afteradd 'touch .after-ran'

	run "$GIT_WT" add -b before-fail before-fail
	[ "$status" -ne 0 ]
	[ ! -e "$TEST_DIR/myrepo/before-fail" ]
	[ ! -e "$TEST_DIR/myrepo/.second-ran" ]
	[ ! -e "$TEST_DIR/myrepo/before-fail/.after-ran" ]
}

@test "hooks: wt.afteradd runs after creating worktree" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git config --add wt.afteradd 'touch .hook-ran'

	run "$GIT_WT" add -b hook-test hook-test
	[ "$status" -eq 0 ]
	[ -f "$TEST_DIR/myrepo/hook-test/.hook-ran" ]
}

@test "hooks: wt.afteradd runs in the new worktree with lifecycle environment" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git config --add wt.afteradd 'printf "%s\\n%s\\n%s\\n%s\\n%s\\n" "$PWD" "$GIT_WT_EVENT" "$GIT_WT_PATH" "$GIT_WT_BRANCH" "$GIT_WT_BARE_ROOT" > .hook-context'

	"$GIT_WT" add -b hook-dir hook-dir
	local context
	context=$(cat "$TEST_DIR/myrepo/hook-dir/.hook-context")
	[[ "$context" == "$TEST_DIR/myrepo/hook-dir"$'\n'"afteradd"$'\n'"$TEST_DIR/myrepo/hook-dir"$'\n'"hook-dir"$'\n'"$TEST_DIR/myrepo" ]]
}

@test "hooks: multiple wt.afteradd values run sequentially" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git config --add wt.afteradd 'echo first >> .hook-order'
	command git config --add wt.afteradd 'echo second >> .hook-order'

	"$GIT_WT" add -b multi-hook multi-hook

	local contents
	contents=$(cat "$TEST_DIR/myrepo/multi-hook/.hook-order")
	[[ "$contents" == *"first"* ]]
	[[ "$contents" == *"second"* ]]
	# Verify order
	[ "$(head -1 "$TEST_DIR/myrepo/multi-hook/.hook-order")" = "first" ]
}

@test "hooks: multiline wt.afteradd remains a single command" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git config --add wt.afteradd $'if true; then\n\ttouch .multiline-ran\nfi'

	run "$GIT_WT" add -b multiline-hook multiline-hook
	[ "$status" -eq 0 ]
	[ -f "$TEST_DIR/myrepo/multiline-hook/.multiline-ran" ]
}

@test "hooks: wt.afteradd failure stops execution and returns error" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git config --add wt.afteradd 'exit 1'
	command git config --add wt.afteradd 'touch .second-ran'

	run "$GIT_WT" add -b fail-hook fail-hook
	[ "$status" -ne 0 ]
	# Second hook should not have run
	[ ! -f "$TEST_DIR/myrepo/fail-hook/.second-ran" ]
}

@test "hooks: wt.afteradd failure does not print success path to stdout" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git config --add wt.afteradd 'exit 1'

	local stdout_file stderr_file
	stdout_file="$TEST_DIR/stdout.txt"
	stderr_file="$TEST_DIR/stderr.txt"

	run bash -c '"$1" add -b fail-stdout fail-stdout >"$2" 2>"$3"' _ \
		"$GIT_WT" "$stdout_file" "$stderr_file"

	[ "$status" -ne 0 ]
	# stdout must stay empty on failure (no machine-readable path).
	[ ! -s "$stdout_file" ]
	[[ "$(cat "$stdout_file")" != *"fail-stdout"* ]]
	# The recovery hint with the path goes to stderr instead.
	[[ "$(cat "$stderr_file")" == *"$TEST_DIR/myrepo/fail-stdout"* ]]
}

@test "hooks: wt.afteradd does not run when no hooks configured" {
	init_bare_repo_with_remote myrepo
	cd myrepo

	run "$GIT_WT" add -b no-hook no-hook
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/no-hook"
}

@test "hooks: wt.afteradd output goes to stderr" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git config --add wt.afteradd 'echo hook-output'

	local stdout_file stderr_file
	stdout_file="$TEST_DIR/stdout.txt"
	stderr_file="$TEST_DIR/stderr.txt"

	"$GIT_WT" add -b stderr-hook stderr-hook >"$stdout_file" 2>"$stderr_file"

	# stdout should only have the path
	[[ "$(cat "$stdout_file")" == *"stderr-hook"* ]]
	# stderr should contain hook output
	[[ "$(cat "$stderr_file")" == *"hook-output"* ]]
}

@test "hooks: wt.afteradd is echoed, not executed, under DEBUG=1" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git config --add wt.afteradd 'touch .should-not-run'

	run env DEBUG=1 "$GIT_WT" add -b dbg-hook dbg-hook
	[ "$status" -eq 0 ]
	# The hook is echoed rather than run.
	[[ "$output" == *"sh -c touch .should-not-run"* ]]
	# DEBUG echoes git mutations too, so nothing should have actually run.
	[ ! -f "$TEST_DIR/myrepo/dbg-hook/.should-not-run" ]
}

@test "hooks: wt.afteradd runs in interactive mode" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git config --add wt.afteradd 'touch .hook-ran'
	command git checkout -b interactive-hook --quiet
	create_commit "interactive-hook.txt"
	command git push --quiet -u origin interactive-hook
	command git checkout main --quiet 2>/dev/null || command git checkout master --quiet
	command git branch -D interactive-hook --quiet

	run env GIT_WT_SELECT="origin/interactive-hook" "$GIT_WT" add
	[ "$status" -eq 0 ]
	[ -f "$TEST_DIR/myrepo/interactive-hook/.hook-ran" ]
}

# --- Remove lifecycle hooks ---

@test "hooks: wt.removehook runs before removing worktree" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree del-hook del-hook
	command git config --add wt.removehook 'touch "$TEST_DIR/delete-hook-ran"'

	run bash -c 'printf "y\n" | "$1" remove "$2"' _ "$GIT_WT" "$TEST_DIR/myrepo/del-hook"
	[ "$status" -eq 0 ]
	[ -f "$TEST_DIR/delete-hook-ran" ]
	assert_worktree_not_exists "$TEST_DIR/myrepo/del-hook"
}

@test "hooks: wt.removehook runs in the worktree directory" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree del-dir del-dir
	command git config --add wt.removehook 'pwd > "$TEST_DIR/delete-hook-pwd"'

	run bash -c 'printf "y\n" | "$1" remove "$2"' _ "$GIT_WT" "$TEST_DIR/myrepo/del-dir"
	[ "$status" -eq 0 ]
	local hook_pwd
	hook_pwd=$(cat "$TEST_DIR/delete-hook-pwd")
	[ "$hook_pwd" = "$TEST_DIR/myrepo/del-dir" ]
}

@test "hooks: wt.removehook failure prevents worktree removal" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree del-fail del-fail
	command git config --add wt.removehook 'exit 1'

	run bash -c 'printf "y\n" | "$1" remove "$2"' _ "$GIT_WT" "$TEST_DIR/myrepo/del-fail"
	[ "$status" -ne 0 ]
	assert_worktree_exists "$TEST_DIR/myrepo/del-fail"
}

@test "hooks: multiple wt.removehook values run sequentially" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree del-multi del-multi
	command git config --add wt.removehook 'echo first >> "$TEST_DIR/del-hook-order"'
	command git config --add wt.removehook 'echo second >> "$TEST_DIR/del-hook-order"'

	run bash -c 'printf "y\n" | "$1" remove "$2"' _ "$GIT_WT" "$TEST_DIR/myrepo/del-multi"
	[ "$status" -eq 0 ]
	[ "$(head -1 "$TEST_DIR/del-hook-order")" = "first" ]
	[ "$(tail -1 "$TEST_DIR/del-hook-order")" = "second" ]
}

@test "hooks: wt.removehook does not run for stale prune" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree stale-prune stale-prune
	command git config --add wt.removehook 'touch "$TEST_DIR/prune-hook-ran"'

	# Manually remove the worktree directory to make it stale
	rm -rf "$TEST_DIR/myrepo/stale-prune"

	run "$GIT_WT" remove --stale
	# Hook should NOT have run (prune path doesn't call removeSingleWorktree)
	[ ! -f "$TEST_DIR/prune-hook-ran" ]
}

@test "hooks: wt.removehook does not run when removing current worktree is rejected" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree del-current del-current
	command git config --add wt.removehook 'touch "$TEST_DIR/remove-hook-should-not-run"'

	# Removing the worktree we are standing in is rejected before hooks run.
	cd "$TEST_DIR/myrepo/del-current"
	run bash -c 'printf "y\n" | "$1" remove "$2"' _ "$GIT_WT" "$TEST_DIR/myrepo/del-current"

	[ "$status" -ne 0 ]
	[ ! -f "$TEST_DIR/remove-hook-should-not-run" ]
	assert_worktree_exists "$TEST_DIR/myrepo/del-current"
}

#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "init: prints bash integration" {
	run "$GIT_WT" init bash
	[ "$status" -eq 0 ]
	[[ $output == *"git-wt()"* ]]
	[[ $output == *"git()"* ]]
	[[ $output == *"cd "* ]]
}

@test "init: prints zsh integration" {
	run "$GIT_WT" init zsh
	[ "$status" -eq 0 ]
	[[ $output == *"git-wt()"* ]]
	[[ $output == *"git()"* ]]
}

@test "init: prints fish integration" {
	run "$GIT_WT" init fish
	[ "$status" -eq 0 ]
	[[ $output == *"function git-wt"* ]]
	[[ $output == *"function git "* ]]
}

@test "init: --no-git-wrapper omits the git() wrapper" {
	run "$GIT_WT" init bash --no-git-wrapper
	[ "$status" -eq 0 ]
	[[ $output == *"git-wt()"* ]]
	[[ $output != *$'\ngit() {'* ]]
}

@test "init: rejects unsupported shell" {
	run "$GIT_WT" init powershell
	[ "$status" -ne 0 ]
	[[ $output == *"unsupported shell"* ]]
}

@test "init: requires a shell argument" {
	run "$GIT_WT" init
	[ "$status" -ne 0 ]
}

@test "init: sourced bash integration cds on add" {
	init_bare_repo myrepo
	cd myrepo

	# Make the binary under test reachable as `git-wt` for the wrapper
	mkdir -p "$TEST_DIR/bin"
	ln -s "$GIT_WT" "$TEST_DIR/bin/git-wt"
	PATH="$TEST_DIR/bin:$PATH"

	eval "$("$GIT_WT" init bash)"

	git-wt add --quiet -b feature-x feature-x </dev/null
	[ "$PWD" = "$TEST_DIR/myrepo/feature-x" ]
}

@test "init: sourced git() wrapper routes git wt add and cds" {
	init_bare_repo myrepo
	cd myrepo

	mkdir -p "$TEST_DIR/bin"
	ln -s "$GIT_WT" "$TEST_DIR/bin/git-wt"
	PATH="$TEST_DIR/bin:$PATH"

	eval "$("$GIT_WT" init bash)"

	git wt add --quiet -b feature-y feature-y </dev/null
	[ "$PWD" = "$TEST_DIR/myrepo/feature-y" ]
}

@test "init: sourced wrapper passes other commands through" {
	init_bare_repo myrepo
	cd myrepo

	mkdir -p "$TEST_DIR/bin"
	ln -s "$GIT_WT" "$TEST_DIR/bin/git-wt"
	PATH="$TEST_DIR/bin:$PATH"

	eval "$("$GIT_WT" init bash)"

	run git-wt list
	[ "$status" -eq 0 ]
	[[ $output == *"main"* ]]

	run git status
	[ "$status" -eq 0 ]
}

#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "list: shows single worktree in bare repo" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" list
	[ "$status" -eq 0 ]
	[[ "$output" != *"Worktree list"* ]]
	[[ "$output" == *"WORKTREE"* ]]
	[[ "$output" == *".bare"* ]]
	[[ "$output" == *"git database"* ]]
}

@test "list: shows multiple worktrees" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree feature-a feature-a
	create_worktree feature-b feature-b

	run "$GIT_WT" list
	[ "$status" -eq 0 ]
	[[ "$output" == *".bare"* ]]
	[[ "$output" == *"feature-a"* ]]
	[[ "$output" == *"feature-b"* ]]
}

@test "list: shows bare directory in bare repo setup" {
	# Use git-wt clone to create a proper bare repo structure
	init_repo source-repo
	cd source-repo
	create_commit "file.txt"
	cd "$TEST_DIR"

	run "$GIT_WT" clone "$TEST_DIR/source-repo" bare-test
	[ "$status" -eq 0 ]

	cd bare-test
	run "$GIT_WT" list
	[ "$status" -eq 0 ]
	# list is a passthrough to git worktree list, so .bare shows up
	[[ "$output" == *".bare"* ]]
	# The default branch could be main or master depending on git version
	[[ "$output" == *"main"* ]] || [[ "$output" == *"master"* ]]
}

@test "list: shows detached HEAD worktrees" {
	init_bare_repo myrepo
	cd myrepo
	local sha
	sha=$(command git rev-parse HEAD)
	command git worktree add --detach detached "$sha" --quiet

	run "$GIT_WT" list
	[ "$status" -eq 0 ]
	[[ "$output" == *"detached"* ]]
}

@test "list: fails outside git repo" {
	run "$GIT_WT" list
	[ "$status" -ne 0 ]
}

@test "list: --help shows usage" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" list --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
}

@test "list: works from worktree subdirectory" {
	init_bare_repo myrepo
	cd myrepo
	command git worktree add main HEAD --quiet 2>/dev/null
	create_worktree feature-list feature-list
	mkdir -p main/src
	cd main/src

	run "$GIT_WT" list
	[ "$status" -eq 0 ]
	[[ "$output" == *"main"* ]]
	[[ "$output" == *"feature-list"* ]]
}

@test "list: still lists worktrees in DEBUG mode" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree feature-debug feature-debug

	run env DEBUG=1 "$GIT_WT" list
	[ "$status" -eq 0 ]
	[[ "$output" == *"feature-debug"* ]]
	[[ "$output" != *"git worktree list"* ]]
}

@test "list: supports --json output" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree feature-json feature-json

	run "$GIT_WT" list --json
	[ "$status" -eq 0 ]
	[[ "$output" == *'"Path"'* ]]
	[[ "$output" == *'"feature-json"'* ]]
}

@test "list: passes through native git flags" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree feature-porcelain feature-porcelain

	run "$GIT_WT" list --porcelain
	[ "$status" -eq 0 ]
	[[ "$output" == *"worktree "* ]]
	[[ "$output" == *"branch refs/heads/feature-porcelain"* ]]
}

@test "list: preserves long nested relative paths" {
	init_bare_repo myrepo
	cd myrepo
	long_path="feature/this-is-a-very-long-worktree-path-for-list-rendering"
	create_worktree "$long_path" "$long_path"

	run env NO_COLOR=1 "$GIT_WT" list
	[ "$status" -eq 0 ]
	[[ "$output" == *"./$long_path"* ]]
}

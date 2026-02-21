#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "clone: clones repo with bare structure" {
	# Create a source repo to clone from
	init_repo source-repo
	cd source-repo
	create_commit "file.txt"
	cd "$TEST_DIR"

	run "$GIT_WT" clone "$TEST_DIR/source-repo" cloned-repo
	[ "$status" -eq 0 ]
	[ -d "$TEST_DIR/cloned-repo" ]
	[ -d "$TEST_DIR/cloned-repo/.bare" ]
	[ -f "$TEST_DIR/cloned-repo/.git" ]
}

@test "clone: creates main worktree" {
	init_repo source-repo
	cd source-repo
	create_commit "file.txt"
	cd "$TEST_DIR"

	run "$GIT_WT" clone "$TEST_DIR/source-repo" cloned-with-main
	[ "$status" -eq 0 ]

	cd "$TEST_DIR/cloned-with-main"
	run command git worktree list
	[ "$status" -eq 0 ]
	# Should have a worktree for the default branch
	[[ "$output" == *"main"* ]] || [[ "$output" == *"master"* ]]
}

@test "clone: --help shows usage" {
	run "$GIT_WT" clone --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
}

@test "clone: fails without repository argument" {
	run "$GIT_WT" clone
	[ "$status" -ne 0 ]
}

@test "clone: .git file contains correct gitdir path" {
	init_repo source-gitdir
	cd source-gitdir
	create_commit "file.txt"
	cd "$TEST_DIR"

	"$GIT_WT" clone "$TEST_DIR/source-gitdir" gitdir-test
	[ -f "$TEST_DIR/gitdir-test/.git" ]

	local gitdir_content
	gitdir_content=$(cat "$TEST_DIR/gitdir-test/.git")
	[[ "$gitdir_content" == "gitdir: ./.bare" ]]
}

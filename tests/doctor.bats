#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "doctor: reports healthy bare layout" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	create_worktree feature-doc feature-doc

	run "$GIT_WT" doctor
	[ "$status" -eq 0 ]
	[[ "$output" == *"bare worktree layout"* ]]
	[[ "$output" == *"Default remote"* ]]
}

@test "doctor: works from bare repo root and linked worktree" {
	init_repo_with_remote source
	cd "$TEST_DIR"

	run "$GIT_WT" clone "$TEST_DIR/source-origin" cloned
	[ "$status" -eq 0 ]

	cd cloned
	repo_root=$(pwd -P)

	run "$GIT_WT" doctor
	[ "$status" -eq 0 ]
	[[ "$output" == *"Repository"* ]]
	[[ "$output" == *"$repo_root"* ]]
	[[ "$output" == *".bare directory"* ]]

	cd main
	run "$GIT_WT" doctor
	[ "$status" -eq 0 ]
	[[ "$output" == *"Repository"* ]]
	[[ "$output" == *"$repo_root"* ]]
	[[ "$output" == *".bare directory"* ]]
}

@test "doctor: reports migration readiness for standard repos" {
	init_repo myrepo
	cd myrepo
	create_commit "file.txt"

	run "$GIT_WT" doctor
	[ "$status" -eq 0 ]
	[[ "$output" == *"standard git layout"* ]]
	[[ "$output" == *"Migration readiness"* ]]
}

@test "doctor: fails when migration blockers are present" {
	init_repo myrepo
	cd myrepo
	create_commit "file.txt"
	command git worktree add ../myrepo-feature -b feature --quiet

	run "$GIT_WT" doctor
	[ "$status" -ne 0 ]
	[[ "$output" == *"linked worktrees"* ]]
}

#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

cleanup_base_fixture() {
	init_bare_repo_with_remote repo
	cd repo
	create_worktree_existing main main
	create_worktree current current
	create_worktree feature feature
	command git push --quiet -u origin feature
	command git config branch.current.remote .
	command git config branch.current.merge refs/heads/main
	cd current
}

@test "cleanup: local upstream fails closed with an explicit base remedy" {
	cleanup_base_fixture
	for count in one two; do
		if [[ "$count" = two ]]; then
			command git remote add other "$TEST_DIR/repo-origin"
		fi
		for filter in --merged --gone --sweep; do
			run "$GIT_WT" remove "$filter" --dry-run
			[ "$status" -ne 0 ]
			[[ "$output" == *'branch "current"'* ]]
			[[ "$output" == *"branch.current.remote is ."* ]]
			[[ "$output" == *"git config wt.cleanupBase refs/heads/main"* ]]
		done
	done
	assert_branch_exists main
	assert_branch_exists feature
	[ -d ../main ]
	[ -d ../feature ]
}

@test "cleanup: explicit one-off base overrides config and preserves the default worktree" {
	cleanup_base_fixture
	command git config wt.cleanupBase refs/heads/missing
	run bash -c 'printf "cleanup\n" | env GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=wt.cleanupBase GIT_CONFIG_VALUE_0=refs/heads/main GIT_WT_SELECT="$2" "$1" remove --merged' _ "$GIT_WT" "$TEST_DIR/repo/feature"
	[ "$status" -eq 0 ]
	[ ! -d ../feature ]
	[ -d ../main ]
	assert_branch_exists main
	assert_branch_not_exists feature
	[ "$(command git config --local wt.cleanupBase)" = refs/heads/missing ]
	[ "$(command git config branch.current.remote)" = . ]
}

@test "cleanup: invalid explicit bases do not fall back to remote discovery" {
	cleanup_base_fixture
	for base in refs/heads/missing refs/tags/main 'main~1'; do
		command git config wt.cleanupBase "$base"
		run "$GIT_WT" remove --merged --dry-run
		[ "$status" -ne 0 ]
		[[ "$output" == *"local branch"* ]]
		[[ "$output" == *"wt.cleanupBase"* ]]
	done
	[ -d ../feature ]
}

@test "cleanup: rechecks and protects a base changed by a before-remove hook" {
	cleanup_base_fixture
	command git config wt.cleanupBase main
	command git config wt.beforeremove 'git config wt.cleanupBase refs/heads/feature'
	run bash -c 'printf "cleanup\n" | env GIT_WT_SELECT="$2" "$1" remove --merged' _ "$GIT_WT" "$TEST_DIR/repo/feature"
	[ "$status" -ne 0 ]
	[[ "$output" == *"cannot remove cleanup base refs/heads/feature"* ]]
	[ -d ../main ]
	[ -d ../feature ]
	assert_branch_exists feature
}

@test "cleanup: raw URL discovery honors remoteTimeout and the remote default" {
	cleanup_base_fixture
	command git config branch.current.remote "$TEST_DIR/repo-origin"
	command git config wt.remoteTimeout 1ns
	run "$GIT_WT" remove --merged --dry-run
	[ "$status" -ne 0 ]
	[[ "$output" == *"cannot determine cleanup base"* ]]
	command git config wt.remoteTimeout 0
	run "$GIT_WT" remove --merged --dry-run
	[ "$status" -eq 0 ]
	[[ "$output" == *"fully merged into main"* ]]
	[[ "$output" != *"./main"* ]]
}

@test "cleanup: stale-only selection needs no cleanup base" {
	cleanup_base_fixture
	run "$GIT_WT" remove --stale --dry-run
	[ "$status" -eq 0 ]
	[[ "$output" == *"No matching cleanup candidates"* ]]
}

@test "cleanup: a tag cannot shadow an explicit local base branch" {
	cleanup_base_fixture
	command git config wt.cleanupBase main
	new=$(command git commit-tree "$(command git rev-parse 'refs/heads/main^{tree}')" -p refs/heads/main -m feature)
	command git update-ref refs/heads/feature "$new"
	command git tag main refs/heads/feature
	run "$GIT_WT" remove --merged --dry-run
	[ "$status" -eq 0 ]
	[[ "$output" == *"No matching cleanup candidates"* ]]
	[ -d ../feature ]
}

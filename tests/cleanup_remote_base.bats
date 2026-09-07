#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

remote_base_fixture() {
	init_bare_repo_with_remote repo
	cd repo
	create_worktree feature feature
	initial=$(command git rev-parse refs/heads/main)
	new=$(command git commit-tree "$(command git rev-parse 'refs/heads/main^{tree}')" -p refs/heads/main -m feature)
	command git update-ref refs/heads/feature "$new"
	command git push --quiet -u origin feature
	command git push --quiet origin feature:main
	command git config wt.cleanupBase refs/remotes/origin/main
	[ "$(command git rev-parse refs/remotes/origin/main)" = "$new" ]
}

@test "cleanup: remote-tracking base works without a local default branch" {
	remote_base_fixture
	command git update-ref -d refs/heads/main
	run bash -c 'printf "cleanup\n" | "$1" remove --merged --delete-remote' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[[ "$output" == *"fully merged into refs/remotes/origin/main"* ]]
	[ ! -d feature ]
	assert_branch_not_exists feature
	assert_branch_not_exists main
	! command git -C "$TEST_DIR/repo-origin" show-ref --verify refs/heads/feature
	[ "$(command git -C "$TEST_DIR/repo-origin" rev-parse main)" = "$new" ]
}

@test "cleanup: remote-tracking base works offline and protects lagging local main" {
	remote_base_fixture
	create_worktree_existing main main
	command git remote set-url origin "$TEST_DIR/unreachable"
	run bash -c 'printf "cleanup\n" | "$1" remove --merged' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[[ "$output" == *"fully merged into refs/remotes/origin/main"* ]]
	[ ! -d feature ]
	[ -d main ]
	[ "$(command git -C main rev-parse HEAD)" = "$initial" ]
	[ "$(command git rev-parse refs/remotes/origin/main)" = "$new" ]
}

@test "cleanup: remote HEAD alias protects the local default during gone cleanup" {
	remote_base_fixture
	create_worktree_existing main main
	command git symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/main
	command git config wt.cleanupBase refs/remotes/origin/HEAD
	command git push --quiet origin --delete feature
	run bash -c 'printf "cleanup\n" | "$1" remove --gone' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[[ "$output" == *"fully merged into refs/remotes/origin/main"* ]]
	[ ! -d feature ]
	[ -d main ]
	[ "$(command git -C main rev-parse HEAD)" = "$initial" ]
}

@test "cleanup: remote-tracking base is rechecked after hooks" {
	remote_base_fixture
	command git config wt.beforeremove 'git update-ref refs/remotes/origin/main refs/heads/main'
	run bash -c 'printf "cleanup\n" | "$1" remove --merged' _ "$GIT_WT"
	[ "$status" -ne 0 ]
	[[ "$output" == *"no longer merged into cleanup base refs/remotes/origin/main"* ]]
	[ -d feature ]
	[ "$(command git rev-parse refs/heads/feature)" = "$new" ]
}

@test "cleanup: remote-tracking base cannot be deleted as a target upstream" {
	remote_base_fixture
	command git config branch.feature.merge refs/heads/main
	run bash -c 'printf "cleanup\n" | "$1" remove --merged --delete-remote' _ "$GIT_WT"
	[ "$status" -ne 0 ]
	[[ "$output" == *"cannot delete cleanup base refs/remotes/origin/main as an upstream"* ]]
	[ -d feature ]
	[ "$(command git rev-parse refs/heads/feature)" = "$new" ]
	[ "$(command git -C "$TEST_DIR/repo-origin" rev-parse main)" = "$new" ]
}

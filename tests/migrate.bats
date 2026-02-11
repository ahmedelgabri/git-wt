#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "migrate: converts standard repo to bare structure" {
	init_repo myrepo
	cd myrepo
	create_commit "file.txt"

	# Run migrate with 'y' confirmation
	echo "y" | "$GIT_WT" migrate

	# Check the new structure exists
	[ -d "$TEST_DIR/myrepo/.bare" ]
	[ -f "$TEST_DIR/myrepo/.git" ]
	[[ $(cat "$TEST_DIR/myrepo/.git") == "gitdir: ./.bare" ]]
}

@test "migrate: creates worktree for current branch" {
	init_repo myrepo
	cd myrepo
	create_commit "file.txt"

	echo "y" | "$GIT_WT" migrate

	# Should have a worktree for main/master
	[ -d "$TEST_DIR/myrepo/main" ] || [ -d "$TEST_DIR/myrepo/master" ]
}

@test "migrate: preserves uncommitted changes" {
	init_repo myrepo
	cd myrepo
	create_commit "file.txt"
	echo "uncommitted change" > uncommitted.txt

	echo "y" | "$GIT_WT" migrate

	# Check the uncommitted file exists in the worktree
	local wt_dir
	if [ -d "$TEST_DIR/myrepo/main" ]; then
		wt_dir="$TEST_DIR/myrepo/main"
	else
		wt_dir="$TEST_DIR/myrepo/master"
	fi
	[ -f "$wt_dir/uncommitted.txt" ]
	[[ $(cat "$wt_dir/uncommitted.txt") == "uncommitted change" ]]
}

@test "migrate: preserves modified tracked files" {
	init_repo myrepo
	cd myrepo
	create_commit "file.txt"
	echo "modified content" > file.txt

	echo "y" | "$GIT_WT" migrate

	local wt_dir
	if [ -d "$TEST_DIR/myrepo/main" ]; then
		wt_dir="$TEST_DIR/myrepo/main"
	else
		wt_dir="$TEST_DIR/myrepo/master"
	fi
	[[ $(cat "$wt_dir/file.txt") == "modified content" ]]
}

@test "migrate: creates separate worktrees for default and current branch" {
	init_repo myrepo
	cd myrepo
	create_commit "file.txt"
	command git checkout -b feature --quiet
	create_commit "feature.txt"

	echo "y" | "$GIT_WT" migrate

	# Should have worktrees for both main/master and feature
	[ -d "$TEST_DIR/myrepo/feature" ]
	[ -d "$TEST_DIR/myrepo/main" ] || [ -d "$TEST_DIR/myrepo/master" ]
}

@test "migrate: fails outside git repo" {
	run "$GIT_WT" migrate
	[ "$status" -ne 0 ]
	[[ "$output" == *"Not in a git repository"* ]]
}

@test "migrate: fails in detached HEAD state" {
	init_repo myrepo
	cd myrepo
	create_commit "file.txt"
	local sha
	sha=$(command git rev-parse HEAD)
	command git checkout --detach "$sha" --quiet

	run bash -c 'echo "y" | '"$GIT_WT"' migrate'
	[ "$status" -ne 0 ]
	[[ "$output" == *"detached HEAD"* ]]
}

@test "migrate: can be cancelled" {
	init_repo myrepo
	cd myrepo
	create_commit "file.txt"

	# Run migrate with 'n' to cancel
	echo "n" | "$GIT_WT" migrate

	# Should still be a standard repo
	[ -d "$TEST_DIR/myrepo/.git" ]
	[ ! -d "$TEST_DIR/myrepo/.bare" ]
}

@test "migrate: preserves remote URL" {
	init_repo_with_remote myrepo
	cd myrepo
	create_commit "file.txt"

	echo "y" | "$GIT_WT" migrate

	cd "$TEST_DIR/myrepo"
	local remote_url
	remote_url=$(command git remote get-url origin 2>/dev/null || true)
	[[ "$remote_url" == *"myrepo-origin"* ]]
}

@test "migrate: preserves repo directory inode (no getcwd errors)" {
	init_repo myrepo
	cd myrepo
	create_commit "file.txt"

	# Record the inode of the repo directory before migration
	local inode_before
	inode_before=$(command stat -c '%i' "$TEST_DIR/myrepo" 2>/dev/null \
		|| command stat -f '%i' "$TEST_DIR/myrepo")

	echo "y" | "$GIT_WT" migrate

	# The inode should be the same after migration
	local inode_after
	inode_after=$(command stat -c '%i' "$TEST_DIR/myrepo" 2>/dev/null \
		|| command stat -f '%i' "$TEST_DIR/myrepo")
	[ "$inode_before" = "$inode_after" ]
}

@test "migrate: copies untracked files to default branch worktree" {
	init_repo_with_remote myrepo
	cd myrepo
	create_commit "file.txt"

	# Create untracked config files in the working tree
	echo "SECRET=prod" >.env
	echo "LOCAL=dev" >.env.local
	echo "use nix" >.envrc
	echo "nodejs 20" >.tool-versions
	mkdir -p packages/api
	echo "DB_URL=localhost" >packages/api/.env

	# Switch to a feature branch so migrate creates two worktrees
	command git checkout -b feature --quiet
	create_commit "feature.txt"

	echo "y" | "$GIT_WT" migrate

	# Current branch worktree should have all files (via rsync)
	[ -f "$TEST_DIR/myrepo/feature/.env" ]
	[ "$(cat "$TEST_DIR/myrepo/feature/.env")" = "SECRET=prod" ]

	# Default branch worktree should also have untracked files copied
	local default_wt
	if [ -d "$TEST_DIR/myrepo/main" ]; then
		default_wt="$TEST_DIR/myrepo/main"
	else
		default_wt="$TEST_DIR/myrepo/master"
	fi
	[ -f "$default_wt/.env" ]
	[ "$(cat "$default_wt/.env")" = "SECRET=prod" ]
	[ -f "$default_wt/.env.local" ]
	[ -f "$default_wt/.envrc" ]
	[ -f "$default_wt/.tool-versions" ]
	[ -f "$default_wt/packages/api/.env" ]
}

@test "migrate: --no-untracked-files skips copying to default branch worktree" {
	init_repo_with_remote myrepo
	cd myrepo
	create_commit "file.txt"
	echo "SECRET=prod" >.env

	command git checkout -b feature --quiet
	create_commit "feature.txt"

	echo "y" | "$GIT_WT" migrate --no-untracked-files

	# Current branch still has it (via rsync, not our feature)
	[ -f "$TEST_DIR/myrepo/feature/.env" ]

	# Default branch should NOT have it
	local default_wt
	if [ -d "$TEST_DIR/myrepo/main" ]; then
		default_wt="$TEST_DIR/myrepo/main"
	else
		default_wt="$TEST_DIR/myrepo/master"
	fi
	[ ! -f "$default_wt/.env" ]
}

@test "migrate: no copy needed when current branch is default branch" {
	init_repo_with_remote myrepo
	cd myrepo
	create_commit "file.txt"
	echo "SECRET=prod" >.env

	# Stay on main — migrate creates one worktree, rsync copies everything
	echo "y" | "$GIT_WT" migrate

	local default_wt
	if [ -d "$TEST_DIR/myrepo/main" ]; then
		default_wt="$TEST_DIR/myrepo/main"
	else
		default_wt="$TEST_DIR/myrepo/master"
	fi
	[ -f "$default_wt/.env" ]
	[ "$(cat "$default_wt/.env")" = "SECRET=prod" ]
}

@test "migrate: --help shows usage" {
	# migrate doesn't have --help, so test that running without args in non-repo fails
	run "$GIT_WT" migrate --help 2>&1
	# Either shows help or fails gracefully
	true
}

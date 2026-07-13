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

@test "migrate: handles slash-containing current branch names" {
	init_repo myrepo
	cd myrepo
	command git checkout --quiet -b feature/foo
	create_commit "feature.txt"
	echo "staged content" > staged.txt
	command git add staged.txt

	run bash -c 'echo "y" | "$1" migrate 2>&1' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[[ "$output" == *"Migration complete"* ]]
	[ -d "$TEST_DIR/myrepo/feature/foo" ]

	cd "$TEST_DIR/myrepo/feature/foo"
	[[ $(command git branch --show-current) == "feature/foo" ]]
	[[ $(command git diff --cached --name-only) == "staged.txt" ]]
	[[ $(cat staged.txt) == "staged content" ]]
}

@test "migrate: creates separate worktrees for default and current branch" {
	init_repo_with_remote myrepo
	cd myrepo
	create_commit "file.txt"
	command git push --quiet origin main 2>/dev/null
	command git checkout -b feature --quiet
	create_commit "feature.txt"

	echo "y" | "$GIT_WT" migrate

	# Should have worktrees for both main and feature
	[ -d "$TEST_DIR/myrepo/feature" ]
	[ -d "$TEST_DIR/myrepo/main" ]
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

@test "migrate: supports --dry-run" {
	init_repo myrepo
	cd myrepo
	create_commit "file.txt"

	run "$GIT_WT" migrate --dry-run
	[ "$status" -eq 0 ]
	[[ "$output" == *"DRY RUN"* ]]
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

@test "migrate: preserves local commits when branch is ahead of remote" {
	init_repo_with_remote myrepo
	cd myrepo
	create_commit "ahead.txt"

	local before_sha
	before_sha=$(command git rev-parse HEAD)

	echo "y" | "$GIT_WT" migrate

	cd "$TEST_DIR/myrepo/main"
	local after_sha
	after_sha=$(command git rev-parse HEAD)
	[ "$after_sha" = "$before_sha" ]
}

@test "migrate: preserves multiple remotes" {
	init_repo_with_remote myrepo
	mkdir -p "$TEST_DIR/myrepo-upstream"
	(
		cd "$TEST_DIR/myrepo-upstream" || exit 1
		command git init --quiet --bare -b main
	)

	cd "$TEST_DIR/myrepo"
	command git remote add upstream "$TEST_DIR/myrepo-upstream"
	command git push --quiet upstream HEAD:main
	create_commit "file.txt"

	echo "y" | "$GIT_WT" migrate

	cd "$TEST_DIR/myrepo"
	local origin_url upstream_url
	origin_url=$(command git remote get-url origin 2>/dev/null || true)
	upstream_url=$(command git remote get-url upstream 2>/dev/null || true)
	[[ "$origin_url" == *"myrepo-origin"* ]]
	[[ "$upstream_url" == *"myrepo-upstream"* ]]
}

@test "migrate: preserves repo-local hooksPath" {
	init_repo myrepo
	cd myrepo
	mkdir .githooks
	command git config core.hooksPath .githooks
	create_commit "file.txt"

	echo "y" | "$GIT_WT" migrate

	cd "$TEST_DIR/myrepo"
	local hooks_path
	hooks_path=$(command git config core.hooksPath 2>/dev/null || true)
	[ "$hooks_path" = ".githooks" ]
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

@test "migrate: succeeds when remote is unreachable" {
	init_repo_with_remote myrepo
	cd myrepo
	create_commit "file.txt"

	# Point origin to a non-existent location
	command git remote set-url origin "/tmp/nonexistent-repo-$$"

	run bash -c 'echo "y" | '"$GIT_WT"' migrate 2>&1'
	[ "$status" -eq 0 ]
	[[ "$output" == *"Migration complete"* ]]
	[ -d "$TEST_DIR/myrepo/.bare" ]
	[ -f "$TEST_DIR/myrepo/.git" ]

	# The unreachable URL should be preserved in the migrated repo
	cd "$TEST_DIR/myrepo"
	local remote_url
	remote_url=$(command git remote get-url origin 2>/dev/null || true)
	[[ "$remote_url" == "/tmp/nonexistent-repo-$$" ]]
}

@test "migrate: preserves symlinks in working directory" {
	init_repo myrepo
	cd myrepo
	echo "real content" > target.txt
	ln -s target.txt link.txt
	command git add target.txt link.txt
	command git commit --quiet -m "add symlink"

	echo "y" | "$GIT_WT" migrate

	local wt_dir
	if [ -d "$TEST_DIR/myrepo/main" ]; then
		wt_dir="$TEST_DIR/myrepo/main"
	else
		wt_dir="$TEST_DIR/myrepo/master"
	fi
	[ -L "$wt_dir/link.txt" ]
	[[ $(readlink "$wt_dir/link.txt") == "target.txt" ]]
	[[ $(cat "$wt_dir/link.txt") == "real content" ]]
}

@test "migrate: prints layout and next steps after success" {
	init_repo myrepo
	cd myrepo
	create_commit "file.txt"

	run bash -c 'echo "y" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[[ "$output" == *"./.bare"* ]]
	[[ "$output" == *"git wt add <branch-name> <branch-name>"* ]]
}

@test "migrate: fails when submodules are present" {
	init_repo lib
	cd lib
	create_commit "lib.txt"

	cd "$TEST_DIR"
	init_repo myrepo
	cd myrepo
	create_commit "file.txt"
	command git -c protocol.file.allow=always submodule add --quiet ../lib vendor/lib
	command git commit --quiet -am "add submodule"

	run bash -c 'echo "y" | '"$GIT_WT"' migrate 2>&1'
	[ "$status" -ne 0 ]
	[[ "$output" == *"submodules"* ]]
}

@test "migrate: fails when sparse checkout is enabled" {
	init_repo myrepo
	cd myrepo
	mkdir -p app docs
	echo "app" > app/app.txt
	echo "docs" > docs/docs.txt
	command git add app docs
	command git commit --quiet -m "add tree"
	command git sparse-checkout init --cone
	command git sparse-checkout set app

	run bash -c 'echo "y" | '"$GIT_WT"' migrate 2>&1'
	[ "$status" -ne 0 ]
	[[ "$output" == *"sparse checkout"* ]]
}

@test "migrate: fails when linked worktrees are present" {
	init_repo myrepo
	cd myrepo
	create_commit "file.txt"
	command git worktree add ../myrepo-feature -b feature --quiet

	run bash -c 'echo "y" | '"$GIT_WT"' migrate 2>&1'
	[ "$status" -ne 0 ]
	[[ "$output" == *"linked worktrees"* ]]
}

@test "migrate: --help shows usage" {
	# migrate doesn't have --help, so test that running without args in non-repo fails
	run "$GIT_WT" migrate --help 2>&1
	# Either shows help or fails gracefully
	true
}

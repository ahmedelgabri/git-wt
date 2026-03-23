#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "clean: removes merged worktrees with upstream" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	create_worktree feature-clean feature-clean

	# use absolute remote URL so push works from worktree subdirectory
	command git remote set-url origin "$TEST_DIR/myrepo-origin"
	# commit, push with upstream tracking, then merge into main
	echo "work" > "$TEST_DIR/myrepo/feature-clean/work.txt"
	command git -C "$TEST_DIR/myrepo/feature-clean" add work.txt
	command git -C "$TEST_DIR/myrepo/feature-clean" commit --quiet -m "feature work"
	command git -C "$TEST_DIR/myrepo/feature-clean" push --quiet -u origin feature-clean
	# HEAD is main at repo root, merge feature branch into it
	command git merge --quiet feature-clean

	echo "y" | "$GIT_WT" clean
	assert_worktree_not_exists "$TEST_DIR/myrepo/feature-clean"
	assert_branch_not_exists "feature-clean"
}

@test "clean: skips local-only branches without upstream" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	create_worktree feature-local feature-local

	run "$GIT_WT" clean --dry-run
	[ "$status" -eq 0 ]
	[[ "$output" == *"No cleanable worktrees found"* ]]
	assert_worktree_exists "$TEST_DIR/myrepo/feature-local"
	assert_branch_exists "feature-local"
}

@test "clean: removes worktrees whose upstream is gone" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	create_worktree feature-gone feature-gone
	command git remote set-url origin "$TEST_DIR/myrepo-origin"
	echo "gone" > "$TEST_DIR/myrepo/feature-gone/gone.txt"
	command git -C "$TEST_DIR/myrepo/feature-gone" add gone.txt
	command git -C "$TEST_DIR/myrepo/feature-gone" commit --quiet -m "gone"
	command git -C "$TEST_DIR/myrepo/feature-gone" push --quiet -u origin feature-gone
	command git push --quiet origin --delete feature-gone

	echo "y" | "$GIT_WT" clean
	assert_worktree_not_exists "$TEST_DIR/myrepo/feature-gone"
	assert_branch_not_exists "feature-gone"
}

@test "clean: prunes stale worktree metadata" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree stale-wt stale-wt
	rm -rf "$TEST_DIR/myrepo/stale-wt"

	echo "y" | "$GIT_WT" clean
	assert_worktree_not_exists "$TEST_DIR/myrepo/stale-wt"
	assert_branch_exists "stale-wt"
}

@test "clean: dry-run preserves worktrees" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	create_worktree dry-run-clean dry-run-clean

	# use absolute remote URL so push works from worktree subdirectory
	command git remote set-url origin "$TEST_DIR/myrepo-origin"
	# push with upstream tracking, then merge into main so it becomes a candidate
	echo "work" > "$TEST_DIR/myrepo/dry-run-clean/work.txt"
	command git -C "$TEST_DIR/myrepo/dry-run-clean" add work.txt
	command git -C "$TEST_DIR/myrepo/dry-run-clean" commit --quiet -m "work"
	command git -C "$TEST_DIR/myrepo/dry-run-clean" push --quiet -u origin dry-run-clean
	# HEAD is main at repo root, merge feature branch into it
	command git merge --quiet dry-run-clean

	run "$GIT_WT" clean --dry-run
	[ "$status" -eq 0 ]
	[[ "$output" == *"DRY RUN"* ]]
	assert_worktree_exists "$TEST_DIR/myrepo/dry-run-clean"
	assert_branch_exists "dry-run-clean"
}

@test "clean: skips dirty worktrees" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	create_worktree dirty-clean dirty-clean
	echo "dirty" > "$TEST_DIR/myrepo/dirty-clean/dirty.txt"

	run "$GIT_WT" clean --dry-run
	[ "$status" -eq 0 ]
	[[ "$output" == *"No cleanable worktrees found"* ]]
	assert_worktree_exists "$TEST_DIR/myrepo/dirty-clean"
	assert_branch_exists "dirty-clean"
}

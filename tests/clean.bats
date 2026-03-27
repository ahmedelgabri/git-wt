#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "remove: --sweep interactively removes selected merged worktrees with upstream" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	create_worktree feature-clean feature-clean

	command git remote set-url origin "$TEST_DIR/myrepo-origin"
	echo "work" > "$TEST_DIR/myrepo/feature-clean/work.txt"
	command git -C "$TEST_DIR/myrepo/feature-clean" add work.txt
	command git -C "$TEST_DIR/myrepo/feature-clean" commit --quiet -m "feature work"
	command git -C "$TEST_DIR/myrepo/feature-clean" push --quiet -u origin feature-clean
	command git merge --quiet feature-clean

	run bash -c 'printf "cleanup\n" | env GIT_WT_SELECT="$2" "$1" remove --sweep' _ "$GIT_WT" "$TEST_DIR/myrepo/feature-clean"
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/myrepo/feature-clean"
	assert_branch_not_exists "feature-clean"
}

@test "remove: --merged skips local-only branches without upstream" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	create_worktree feature-local feature-local

	run "$GIT_WT" remove --merged --dry-run
	[ "$status" -eq 0 ]
	[[ "$output" == *"No matching cleanup candidates found"* ]]
	assert_worktree_exists "$TEST_DIR/myrepo/feature-local"
	assert_branch_exists "feature-local"
}

@test "remove: --gone interactively removes worktrees whose upstream is gone" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	create_worktree feature-gone feature-gone
	command git remote set-url origin "$TEST_DIR/myrepo-origin"
	echo "gone" > "$TEST_DIR/myrepo/feature-gone/gone.txt"
	command git -C "$TEST_DIR/myrepo/feature-gone" add gone.txt
	command git -C "$TEST_DIR/myrepo/feature-gone" commit --quiet -m "gone"
	command git -C "$TEST_DIR/myrepo/feature-gone" push --quiet -u origin feature-gone
	command git push --quiet origin --delete feature-gone

	run bash -c 'printf "cleanup\n" | env GIT_WT_SELECT="$2" "$1" remove --gone' _ "$GIT_WT" "$TEST_DIR/myrepo/feature-gone"
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/myrepo/feature-gone"
	assert_branch_not_exists "feature-gone"
}

@test "remove: --stale prunes stale worktree metadata" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree stale-wt stale-wt
	rm -rf "$TEST_DIR/myrepo/stale-wt"

	run bash -c 'printf "cleanup\n" | env GIT_WT_SELECT="$2" "$1" remove --stale' _ "$GIT_WT" "$TEST_DIR/myrepo/stale-wt"
	[ "$status" -eq 0 ]
	assert_worktree_not_exists "$TEST_DIR/myrepo/stale-wt"
	assert_branch_exists "stale-wt"
}

@test "remove: --sweep --dry-run preserves worktrees" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	create_worktree dry-run-clean dry-run-clean

	command git remote set-url origin "$TEST_DIR/myrepo-origin"
	echo "work" > "$TEST_DIR/myrepo/dry-run-clean/work.txt"
	command git -C "$TEST_DIR/myrepo/dry-run-clean" add work.txt
	command git -C "$TEST_DIR/myrepo/dry-run-clean" commit --quiet -m "work"
	command git -C "$TEST_DIR/myrepo/dry-run-clean" push --quiet -u origin dry-run-clean
	command git merge --quiet dry-run-clean

	run env GIT_WT_SELECT="$TEST_DIR/myrepo/dry-run-clean" "$GIT_WT" remove --sweep --dry-run
	[ "$status" -eq 0 ]
	[[ "$output" == *"DRY RUN"* ]]
	assert_worktree_exists "$TEST_DIR/myrepo/dry-run-clean"
	assert_branch_exists "dry-run-clean"
}

@test "remove: --merged skips dirty worktrees" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	create_worktree dirty-clean dirty-clean
	echo "dirty" > "$TEST_DIR/myrepo/dirty-clean/dirty.txt"

	run "$GIT_WT" remove --merged --dry-run
	[ "$status" -eq 0 ]
	[[ "$output" == *"No matching cleanup candidates found"* ]]
	assert_worktree_exists "$TEST_DIR/myrepo/dirty-clean"
	assert_branch_exists "dirty-clean"
}

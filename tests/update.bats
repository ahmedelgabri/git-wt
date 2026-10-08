#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

init_update_repo() {
	init_bare_repo_with_remote myrepo
	cd myrepo
	command git worktree add main main --quiet
	mkdir -p "$TEST_DIR/bin"
	ln -s "$GIT_WT" "$TEST_DIR/bin/git-wt"
	export PATH="$TEST_DIR/bin:$PATH"
}

make_update_diverge() {
	command git worktree add -b remote-change "$TEST_DIR/remote-change" main --quiet
	(cd main && create_commit local.txt)
	(cd "$TEST_DIR/remote-change" && create_commit remote.txt && command git push --quiet origin HEAD:main)
}

assert_update_rebased() {
	[ "$(command git -C main rev-parse HEAD^)" = "$(command git rev-parse origin/main)" ]
	[ -f main/local.txt ]
	[ -f main/remote.txt ]
}

@test "update: tag pruning follows global and per-remote configuration" {
	init_update_repo
	export GIT_CONFIG_GLOBAL="$TEST_DIR/global-config"
	command git config --global fetch.pruneTags true
	command git tag global-pruned
	run "$GIT_WT" update
	[ "$status" -eq 0 ]
	! command git show-ref --verify refs/tags/global-pruned

	command git config remote.origin.pruneTags false
	command git tag remote-preserved
	run "$GIT_WT" update
	[ "$status" -eq 0 ]
	command git show-ref --verify refs/tags/remote-preserved

	command git config --global fetch.pruneTags false
	command git config remote.origin.pruneTags true
	run "$GIT_WT" update
	[ "$status" -eq 0 ]
	! command git show-ref --verify refs/tags/remote-preserved

	command git tag one-off-remote-preserved
	run command git -c remote.origin.pruneTags=false wt update
	[ "$status" -eq 0 ]
	command git show-ref --verify refs/tags/one-off-remote-preserved
}

@test "update: one-off tag pruning overrides repository configuration" {
	init_update_repo
	command git config fetch.pruneTags false
	command git tag one-off-pruned
	run command git -c fetch.pruneTags=true wt update
	[ "$status" -eq 0 ]
	! command git show-ref --verify refs/tags/one-off-pruned

	command git config fetch.pruneTags true
	command git tag one-off-preserved
	run command git -c fetch.pruneTags=false wt update
	[ "$status" -eq 0 ]
	command git show-ref --verify refs/tags/one-off-preserved
}

@test "update: pull respects global rebase configuration" {
	init_update_repo
	make_update_diverge
	export GIT_CONFIG_GLOBAL="$TEST_DIR/global-config"
	command git config --global pull.rebase true
	run "$GIT_WT" update
	[ "$status" -eq 0 ]
	assert_update_rebased
}

@test "update: pull respects branch-specific rebase configuration" {
	init_update_repo
	make_update_diverge
	command git config pull.rebase false
	command git config branch.main.rebase true
	run "$GIT_WT" update
	[ "$status" -eq 0 ]
	assert_update_rebased
}

@test "update: one-off pull strategy overrides repository configuration" {
	init_update_repo
	make_update_diverge
	command git config pull.rebase false
	run command git -c pull.rebase=true wt update
	[ "$status" -eq 0 ]
	assert_update_rebased
}

@test "update: pull respects a configured merge strategy" {
	init_update_repo
	make_update_diverge
	local_head=$(command git rev-parse main)
	remote_head=$(command git rev-parse origin/main)
	command git config pull.rebase false
	command git config pull.ff false
	run env GIT_MERGE_AUTOEDIT=no "$GIT_WT" update
	[ "$status" -eq 0 ]
	[ "$(command git -C main rev-parse HEAD^1)" = "$local_head" ]
	[ "$(command git -C main rev-parse HEAD^2)" = "$remote_head" ]
}

@test "update: configured fast-forward-only still rejects divergence" {
	init_update_repo
	make_update_diverge
	before=$(command git rev-parse main)
	command git config pull.ff only
	run "$GIT_WT" update
	[ "$status" -ne 0 ]
	[[ "$output" == *"Not possible to fast-forward"* ]]
	[ "$(command git rev-parse main)" = "$before" ]
}

@test "update: --help shows usage" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" update --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
	[[ "$output" == *"Fetch"* ]]
}

@test "update: alias 'u' works with --help" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" u --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
}

@test "update: fails gracefully without remote" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" update
	# Should fail or warn when no remote configured
	# The exact behavior depends on implementation
	[[ "$status" -ne 0 ]] || [[ "$output" == *"remote"* ]] || [[ "$output" == *"error"* ]] || [[ "$output" == *"Error"* ]]
}

@test "update: works with remote configured" {
	# Create a "remote" repo
	init_repo remote-repo
	cd "$TEST_DIR"

	# Clone it
	command git clone --quiet remote-repo local-repo
	cd local-repo

	run "$GIT_WT" update
	[ "$status" -eq 0 ]
}

@test "update: works from worktree subdirectory" {
	init_bare_repo_with_remote myrepo
	cd myrepo
	# Use absolute path for remote (relative paths break from subdirs)
	command git remote set-url origin "$TEST_DIR/myrepo-origin"
	command git worktree add main main --quiet 2>/dev/null
	mkdir -p main/src
	cd main/src

	run "$GIT_WT" update
	[ "$status" -eq 0 ]
}

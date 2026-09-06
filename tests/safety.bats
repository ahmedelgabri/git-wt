#!/usr/bin/env bats

load test_helper

setup() { setup_test_env; }
teardown() { teardown_test_env; }

@test "clone: failed nested destination never deletes unrelated data" {
	mkdir -p parent/parent/new
	echo precious >parent/parent/new/precious.txt
	run "$GIT_WT" clone "$TEST_DIR/missing" parent/new
	[ "$status" -ne 0 ]
	[ -f parent/parent/new/precious.txt ]
	[ ! -e parent/new ]
}

@test "clone: refuses a dangling destination symlink" {
	ln -s absent destination
	run "$GIT_WT" clone "$TEST_DIR/missing" destination
	[ "$status" -ne 0 ]
	[ -L destination ]
}

@test "migrate: preserves deleted files and executable changes" {
	init_repo repo
	cd repo
	create_commit unstaged.txt
	create_commit staged.txt
	create_commit script.sh
	rm unstaged.txt
	command git rm --quiet staged.txt
	chmod +x script.sh
	before=$(command git status --porcelain)
	printf 'y\n' | "$GIT_WT" migrate
	[ ! -e main/unstaged.txt ]
	[ ! -e main/staged.txt ]
	[ -x main/script.sh ]
	[ "$(command git -C main status --porcelain)" = "$before" ]
	[ -d "$TEST_DIR"/repo-backup-*/.git ]
}

@test "migrate: replacing a symlink never overwrites its external target" {
	echo external >external.txt
	init_repo repo
	cd repo
	ln -s "$TEST_DIR/external.txt" link
	command git add link
	command git commit --quiet -m link
	rm link
	echo replacement >link
	printf 'y\n' | "$GIT_WT" migrate
	[ "$(cat "$TEST_DIR/external.txt")" = external ]
	[ ! -L main/link ]
	[ "$(cat main/link)" = replacement ]
}

@test "migrate: packed multiple stashes and split index remain usable" {
	init_repo repo
	cd repo
	create_commit tracked.txt
	for n in first second; do
		echo "$n" >tracked.txt
		command git stash push --quiet -m "$n"
	done
	command git pack-refs --all
	before=$(command git stash list --format='%H %gs')
	echo staged >tracked.txt
	command git add tracked.txt
	command git update-index --split-index
	printf 'y\n' | "$GIT_WT" migrate
	[ "$(command git -C main stash list --format='%H %gs')" = "$before" ]
	[ "$(command git -C main show :tracked.txt)" = staged ]
	command git -C main status --porcelain
	command git -C main stash show -p 'stash@{1}'
}

@test "migrate: preserves complete local config hooks and nested repositories" {
	init_repo repo
	cd repo
	command git config wt.afteradd 'echo hook'
	command git config alias.multi $'!echo first\necho second'
	command git config --add remote.origin.url "$TEST_DIR/not-reachable"
	command git config --add remote.origin.pushurl "$TEST_DIR/push-only"
	mkdir -p .git/hooks
	printf '#!/bin/sh\nexit 0\n' >.git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit
	init_repo nested
	printf 'y\n' | "$GIT_WT" migrate
	[ "$(command git config wt.afteradd)" = 'echo hook' ]
	[ "$(command git config alias.multi)" = $'!echo first\necho second' ]
	[ "$(command git config remote.origin.pushurl)" = "$TEST_DIR/push-only" ]
	[ -x .bare/hooks/pre-commit ]
	command git -C main/nested rev-parse --verify HEAD
}

@test "migrate: refuses a repository during a Git operation" {
	init_repo repo
	cd repo
	touch .git/index.lock
	run "$GIT_WT" migrate --dry-run
	[ "$status" -ne 0 ]
	[ -d .git ]
	[ ! -e .bare ]
}

@test "migrate: DEBUG does not write filesystem state" {
	init_repo repo
	cd repo
	run env DEBUG=1 "$GIT_WT" migrate
	[ "$status" -eq 0 ]
	[ -d .git ]
	[ ! -e .bare ]
}

@test "remove: gone upstream with unique local commits is not a cleanup candidate" {
	init_bare_repo_with_remote repo
	cd repo
	create_remote_branch feature
	command git worktree add feature feature --quiet
	(cd feature && create_commit unpushed.txt)
	command git push --quiet origin --delete feature
	run bash -c 'printf "cleanup\n" | "$1" remove --sweep' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	assert_branch_exists feature
	[ -f feature/unpushed.txt ]
}

@test "remove: explicit unique commits require force" {
	init_bare_repo repo
	cd repo
	create_worktree feature feature
	(cd feature && create_commit unique.txt)
	run bash -c 'printf "y\n" | "$1" remove feature' _ "$GIT_WT"
	[ "$status" -ne 0 ]
	[[ "$output" == *"retained"* ]]
	assert_branch_exists feature
	[ -f feature/unique.txt ]
}

@test "remove: checks for changes made by before-remove hooks" {
	init_bare_repo repo
	cd repo
	create_worktree feature feature
	command git config wt.beforeremove 'echo valuable >hook-output.txt'
	run bash -c 'printf "y\n" | "$1" remove feature' _ "$GIT_WT"
	[ "$status" -ne 0 ]
	[ -f feature/hook-output.txt ]
	assert_branch_exists feature
}

@test "remove: remote deletion follows target upstream including renamed branches" {
	init_bare_repo_with_remote repo
	init_repo upstream
	cd repo
	command git remote add upstream "$TEST_DIR/upstream"
	create_worktree local local
	command git push --quiet origin local
	command git push --quiet -u upstream local:remote-name
	run bash -c 'printf "local\n" | "$1" remove local --delete-remote' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	command git --git-dir="$TEST_DIR/repo-origin" show-ref --verify refs/heads/local
	! command git -C "$TEST_DIR/upstream" show-ref --verify refs/heads/remote-name
}

@test "remove: remote query failure is not reported as success" {
	init_bare_repo_with_remote repo
	cd repo
	create_worktree feature feature
	command git push --quiet -u origin feature
	command git remote set-url origin "$TEST_DIR/unreachable"
	run bash -c 'printf "feature\n" | "$1" remove feature --delete-remote' _ "$GIT_WT"
	[ "$status" -ne 0 ]
	[[ "$output" == *"remote branch"* ]]
}

@test "remove: force is rejected for cleanup filters" {
	init_bare_repo repo
	cd repo
	run "$GIT_WT" remove --sweep --force
	[ "$status" -ne 0 ]
}

@test "remove: newline paths round-trip through porcelain" {
	init_bare_repo repo
	cd repo
	path=$'feature\nline'
	command git worktree add -b feature "$path" --quiet
	run "$GIT_WT" remove "$path" --dry-run
	[ "$status" -eq 0 ]
	[ -d "$path" ]
}

@test "update: preserves local-only tags by default" {
	init_bare_repo_with_remote repo
	cd repo
	command git worktree add main main --quiet
	command git tag local-only
	run "$GIT_WT" update
	[ "$status" -eq 0 ]
	command git show-ref --verify refs/tags/local-only
}

@test "add: rejects a standard repository without touching its database" {
	init_repo repo
	cd repo
	run "$GIT_WT" add -b feature feature
	[ "$status" -ne 0 ]
	[[ "$output" == *"migrate"* ]]
	[ ! -e .git/feature ]
}

@test "add: no-track is respected" {
	init_bare_repo_with_remote repo
	cd repo
	command git --git-dir="$TEST_DIR/repo-origin" branch feature main
	"$GIT_WT" add --no-track -b feature feature origin/feature
	[ -z "$(command git for-each-ref --format='%(upstream)' refs/heads/feature)" ]
}

@test "add: interactive selection reuses an existing local branch without reset" {
	init_bare_repo_with_remote repo
	cd repo
	create_remote_branch feature
	before=$(command git rev-parse feature)
	run select_remote_branch origin/feature
	[ "$status" -eq 0 ]
	[ "$(command git -C feature rev-parse HEAD)" = "$before" ]
	[ "$(command git -C feature branch --show-current)" = feature ]
}

@test "add: EOF at the worktree path prompt cancels creation" {
	init_bare_repo_with_remote repo
	cd repo
	command git --git-dir="$TEST_DIR/repo-origin" branch feature main
	run bash -c 'GIT_WT_SELECT=origin/feature "$1" add </dev/null' _ "$GIT_WT"
	[ "$status" -ne 0 ]
	[ ! -e feature ]
}

@test "status: missing upstream is not displayed as synced" {
	init_bare_repo_with_remote repo
	cd repo
	create_worktree feature feature
	command git push --quiet -u origin feature
	command git push --quiet origin --delete feature
	run "$GIT_WT" status
	[ "$status" -eq 0 ]
	[[ "$output" == *"upstream unavailable"* ]]
	[[ "$output" != *"synced"* ]]
}

@test "remove: remote rejection returns failure after local removal" {
	init_bare_repo_with_remote repo
	cd repo
	create_worktree feature feature
	command git push --quiet -u origin feature
	printf '#!/bin/sh\nexit 1\n' >"$TEST_DIR/repo-origin/hooks/pre-receive"
	chmod +x "$TEST_DIR/repo-origin/hooks/pre-receive"
	run bash -c 'printf "feature\n" | "$1" remove feature --delete-remote' _ "$GIT_WT"
	[ "$status" -ne 0 ]
	[[ "$output" == *"local worktree removed"* ]]
	[ ! -d feature ]
	command git --git-dir="$TEST_DIR/repo-origin" show-ref --verify refs/heads/feature
}

@test "remove: ignored files are protected without force" {
	init_bare_repo repo
	cd repo
	create_worktree feature feature
	echo secret.txt >.bare/info/exclude
	echo valuable >feature/secret.txt
	run bash -c 'printf "y\n" | "$1" remove feature' _ "$GIT_WT"
	[ "$status" -ne 0 ]
	[ -f feature/secret.txt ]
}

@test "remove: missing worktree can be explicitly removed without force" {
	init_bare_repo repo
	cd repo
	create_worktree feature feature
	rm -rf feature
	run bash -c 'printf "y\n" | "$1" remove feature' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	assert_branch_not_exists feature
}

@test "remove: stale cleanup preserves detached worktree metadata" {
	init_bare_repo repo
	cd repo
	command git worktree add --detach detached HEAD --quiet
	rm -rf detached
	run bash -c 'printf "cleanup\n" | "$1" remove --stale' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	assert_worktree_exists "$TEST_DIR/repo/detached"
}

@test "migrate: rejects symlinked metadata before writing through it" {
	init_repo repo
	cd repo
	mv .git/config "$TEST_DIR/external-config"
	ln -s "$TEST_DIR/external-config" .git/config
	before=$(cat "$TEST_DIR/external-config")
	run bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -ne 0 ]
	[[ "$output" == *"symlinked Git metadata"* ]]
	[ "$(cat "$TEST_DIR/external-config")" = "$before" ]
	[ -d .git ]
}

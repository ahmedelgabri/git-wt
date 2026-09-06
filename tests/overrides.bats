#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "migrate: preserves fetch and push URL aliases" {
	init_repo_with_remote repo
	cd repo
	command git config "url.$TEST_DIR/.insteadOf" corp/
	command git remote set-url origin corp/repo-origin
	command git config "url.$TEST_DIR/.pushInsteadOf" publish/
	command git remote add publish publish/repo-origin
	command git config --add remote.origin.pushurl corp/repo-origin
	run bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[ "$(command git config remote.origin.url)" = corp/repo-origin ]
	[ "$(command git config remote.origin.pushurl)" = corp/repo-origin ]
	[ "$(command git config remote.publish.url)" = publish/repo-origin ]
	command git -C main fetch origin
	command git -C main push origin main
	command git -C main push publish main
}

@test "migrate: keeps native checkout hooks disabled only during preparation" {
	init_repo_with_remote repo
	cd repo
	command git checkout --quiet -b feature
	mkdir "$TEST_DIR/hooks"
	printf '#!/bin/sh\ntouch "%s"\n' "$TEST_DIR/checkout-ran" >"$TEST_DIR/hooks/post-checkout"
	chmod +x "$TEST_DIR/hooks/post-checkout"
	command git config core.hooksPath "$TEST_DIR/hooks"
	run bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[ ! -e "$TEST_DIR/checkout-ran" ]
	[ "$(command git config core.hooksPath)" = "$TEST_DIR/hooks" ]
	command git -C main checkout --quiet main
	[ -e "$TEST_DIR/checkout-ran" ]
}

@test "migrate: still resolves genuine relative remote URLs" {
	init_repo_with_remote repo
	cd repo
	run bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[ "$(command git remote get-url origin)" = "$TEST_DIR/repo-origin" ]
	command git -C main fetch origin
}

@test "remove: leases each push destination and retains remote transport settings" {
	init_bare_repo_with_remote repo
	cd repo
	initial=$(command git rev-parse main)
	new=$(command git commit-tree "$(command git rev-parse 'main^{tree}')" -p main -m next)
	command git update-ref refs/heads/main "$new"
	command git push --quiet origin main:feature
	for name in push-one push-two; do
		command git init --bare --quiet "$TEST_DIR/$name"
		command git config --add remote.origin.pushurl "$TEST_DIR/$name"
	done
	command git push --quiet "$TEST_DIR/push-one" "$initial:refs/heads/feature"
	command git push --quiet "$TEST_DIR/push-two" main:feature
	command git worktree add --quiet -b feature feature main
	command git config branch.feature.remote origin
	command git config branch.feature.merge refs/heads/feature
	command git config branch.feature.rebase true
	printf '#!/bin/sh\necho called >>"%s"\nexec git receive-pack "$@"\n' "$TEST_DIR/receive-pack-called" >"$TEST_DIR/receive-pack"
	chmod +x "$TEST_DIR/receive-pack"
	command git config remote.origin.receivepack "$TEST_DIR/receive-pack"
	before=$(command git config --get-all remote.origin.pushurl)
	run bash -c 'printf "feature\n" | "$1" remove feature --delete-remote' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[ ! -d feature ]
	! command git config --get-regexp '^branch\.feature\.'
	! command git -C "$TEST_DIR/push-one" show-ref --verify refs/heads/feature
	! command git -C "$TEST_DIR/push-two" show-ref --verify refs/heads/feature
	command git -C "$TEST_DIR/repo-origin" show-ref --verify refs/heads/feature
	[ "$(wc -l <"$TEST_DIR/receive-pack-called" | tr -d ' ')" = 2 ]
	[ "$(command git config --get-all remote.origin.pushurl)" = "$before" ]
}

@test "remove: honors pushInsteadOf without explicit push URLs" {
	init_bare_repo_with_remote repo
	cd repo
	command git worktree add --quiet -b feature feature main
	command git push --quiet -u origin feature
	command git init --bare --quiet "$TEST_DIR/push-only"
	command git push --quiet "$TEST_DIR/push-only" feature
	command git config "url.$TEST_DIR/push-only.pushInsteadOf" "$TEST_DIR/repo-origin"
	run bash -c 'printf "feature\n" | "$1" remove feature --delete-remote' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	! command git -C "$TEST_DIR/push-only" show-ref --verify refs/heads/feature
	command git -C "$TEST_DIR/repo-origin" show-ref --verify refs/heads/feature
}

@test "remove: reports when a remote branch is already absent" {
	init_bare_repo_with_remote repo
	cd repo
	create_worktree feature feature
	command git push --quiet -u origin feature
	command git push --quiet origin --delete feature
	run bash -c 'printf "feature\n" | "$1" remove feature --delete-remote' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[[ "$output" == *"No remote branch origin/feature"* ]]
	[[ "$output" == *"deletion skipped"* ]]
	[ ! -d feature ]
}

@test "remove: checks all push destinations before removing anything" {
	init_bare_repo_with_remote repo
	cd repo
	command git worktree add --quiet -b feature feature main
	command git push --quiet -u origin feature
	command git config --add remote.origin.pushurl "$TEST_DIR/repo-origin"
	command git config --add remote.origin.pushurl "$TEST_DIR/unreachable"
	run bash -c 'printf "feature\n" | "$1" remove feature --delete-remote' _ "$GIT_WT"
	[ "$status" -ne 0 ]
	[ -d feature ]
	command git show-ref --verify refs/heads/feature
	command git -C "$TEST_DIR/repo-origin" show-ref --verify refs/heads/feature
}

@test "remove: push destination lease rejects a concurrent branch update" {
	init_bare_repo_with_remote repo
	cd repo
	command git worktree add --quiet -b feature feature main
	command git push --quiet -u origin feature
	next=$(command git commit-tree "$(command git rev-parse 'main^{tree}')" -p main -m concurrent)
	command git push --quiet origin "$next:refs/heads/next"
	mkdir -p .bare/hooks
	printf '#!/bin/sh\n[ "$1" = committed ] || exit 0\nwhile read old new ref; do\n  if [ "$ref" = refs/heads/feature ]; then\n    git --git-dir="%s" update-ref refs/heads/feature "%s" || exit 1\n  fi\ndone\n' "$TEST_DIR/repo-origin" "$next" >.bare/hooks/reference-transaction
	chmod +x .bare/hooks/reference-transaction
	run bash -c 'printf "feature\n" | "$1" remove feature --delete-remote' _ "$GIT_WT"
	[ "$status" -ne 0 ]
	[[ "$output" == *"remote deletion failed"* ]]
	[ ! -d feature ]
	[ "$(command git -C "$TEST_DIR/repo-origin" rev-parse feature)" = "$next" ]
}

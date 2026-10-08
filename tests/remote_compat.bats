#!/usr/bin/env bats

load test_helper

setup() {
	bats_require_minimum_version 1.5.0
	setup_test_env
}
teardown() { teardown_test_env; }

unsupported_git_path() {
	if [[ -z ${GIT_WT_OLD_GIT:-} ]]; then
		skip "Set GIT_WT_OLD_GIT to a real Git binary older than 2.48.0"
	fi
	local version
	version=$("$GIT_WT_OLD_GIT" --version)
	[[ "$version" =~ ^git\ version\ ([0-9]+)\.([0-9]+)\.([0-9]+) ]]
	[[ ${BASH_REMATCH[1]} -lt 2 || ${BASH_REMATCH[1]} -eq 2 && ${BASH_REMATCH[2]} -lt 48 ]]
	mkdir -p "$TEST_DIR/old-bin"
	ln -s "$GIT_WT_OLD_GIT" "$TEST_DIR/old-bin/git"
	OLD_GIT_PATH="$TEST_DIR/old-bin:$PATH"
}

assert_unsupported_git() {
	run --separate-stderr env NO_COLOR=1 PATH="$OLD_GIT_PATH" "$GIT_WT" "$@"
	[ "$status" -eq 1 ]
	[ -z "$output" ]
	[[ "$stderr" == *"requires Git 2.48.0 or newer"* ]]
	[[ "$stderr" == *"Upgrade Git on PATH"* ]]
}

@test "version: rejects older Git before clone or migration changes files" {
	init_repo source
	unsupported_git_path
	assert_unsupported_git clone "$TEST_DIR/source" clone
	[ ! -e clone ]
	cd source
	cp .git/config "$TEST_DIR/config-before"
	assert_unsupported_git migrate
	[ -d .git ]
	[ ! -e .bare ]
	[ ! -e .git-wt-migrate ]
	cmp .git/config "$TEST_DIR/config-before"
}

@test "version: rejects older Git for commands aliases and native passthroughs" {
	init_bare_repo repo
	cd repo
	create_worktree feature feature
	command git config wt.beforeadd 'touch "$TEST_DIR/hook-ran"'
	command git config wt.beforeremove 'touch "$TEST_DIR/hook-ran"'
	cp .bare/config "$TEST_DIR/config-before"
	unsupported_git_path
	for subcommand in list ls status doctor switch update u lock unlock move prune repair not-a-command; do
		assert_unsupported_git "$subcommand"
	done
	assert_unsupported_git add -b new-feature new-feature
	assert_unsupported_git remove feature
	assert_unsupported_git rm feature --dry-run
	assert_unsupported_git destroy feature --force
	assert_unsupported_git _preview worktree "$TEST_DIR/repo/feature"
	assert_unsupported_git list -- --help
	[ -d feature ]
	[ ! -e new-feature ]
	[ ! -e "$TEST_DIR/hook-ran" ]
	assert_branch_exists feature
	assert_branch_not_exists new-feature
	cmp .bare/config "$TEST_DIR/config-before"
}

@test "version: DEBUG does not bypass the minimum Git version" {
	unsupported_git_path
	export DEBUG=1
	assert_unsupported_git clone "$TEST_DIR/source" clone
	[ ! -e clone ]
}

@test "version: help and version output remain available with older Git" {
	unsupported_git_path
	run env PATH="$OLD_GIT_PATH" "$GIT_WT"
	[ "$status" -eq 0 ]
	for flag in --help -h --version; do
		run env PATH="$OLD_GIT_PATH" "$GIT_WT" "$flag"
		[ "$status" -eq 0 ]
		[ -n "$output" ]
	done
	for subcommand in add list ls remove migrate lock; do
		for flag in --help -h; do
			run env PATH="$OLD_GIT_PATH" "$GIT_WT" "$subcommand" "$flag"
			[ "$status" -eq 0 ]
			[ -n "$output" ]
		done
	done
	run env PATH="$OLD_GIT_PATH" "$GIT_WT" help list
	[ "$status" -eq 0 ]
}

@test "version: reports missing Git without writing to stdout" {
	mkdir no-git
	run --separate-stderr env NO_COLOR=1 PATH="$TEST_DIR/no-git" "$GIT_WT" list --json
	[ "$status" -eq 1 ]
	[ -z "$output" ]
	[[ "$stderr" == *"requires Git 2.48.0 or newer; could not run git --version"* ]]
}

@test "version: clone add and migration create relocatable relative worktrees" {
	init_repo source
	run "$GIT_WT" clone "$TEST_DIR/source" repo
	[ "$status" -eq 0 ]
	cd repo
	run "$GIT_WT" add -b feature feature
	[ "$status" -eq 0 ]
	[ "$(command git config --get extensions.relativeworktrees)" = true ]
	[[ "$(<main/.git)" == "gitdir: ../.bare/worktrees/main" ]]
	[[ "$(<feature/.git)" == "gitdir: ../.bare/worktrees/feature" ]]
	cd "$TEST_DIR"
	mv repo moved-repo
	run command git -C moved-repo/main status --porcelain
	[ "$status" -eq 0 ]
	[ -z "$output" ]
	run command git -C moved-repo/feature status --porcelain
	[ "$status" -eq 0 ]
	[ -z "$output" ]
	cd source
	run bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[ "$(command git config --get extensions.relativeworktrees)" = true ]
	[[ "$(<main/.git)" == "gitdir: ../.bare/worktrees/main" ]]
	cd "$TEST_DIR"
	mv source moved-source
	run command git -C moved-source/main status --porcelain
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "remove: matching rewritten URLs retain named transport settings" {
	init_bare_repo_with_remote repo
	cd repo
	create_worktree feature feature
	command git push --quiet -u origin feature
	command git config wt.beforeremove 'touch "$TEST_DIR/hook-ran"'
	command git config "url.$TEST_DIR/repo-origin.insteadOf" alias:repo
	command git config remote.origin.url alias:repo
	command git config remote.origin.pushurl "$TEST_DIR/repo-origin"
	for operation in upload-pack receive-pack; do
		printf '#!/bin/sh\necho called >>"%s"\nexec git %s "$@"\n' "$TEST_DIR/$operation-called" "$operation" >"$TEST_DIR/$operation"
		chmod +x "$TEST_DIR/$operation"
	done
	command git config remote.origin.uploadpack "$TEST_DIR/upload-pack"
	command git config remote.origin.receivepack "$TEST_DIR/receive-pack"
	run bash -c 'printf "feature\n" | "$1" remove feature --delete-remote' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[ ! -d feature ]
	assert_branch_not_exists feature
	! command git -C "$TEST_DIR/repo-origin" show-ref --verify refs/heads/feature
	[ -f "$TEST_DIR/hook-ran" ]
	[ -f "$TEST_DIR/upload-pack-called" ]
	[ -f "$TEST_DIR/receive-pack-called" ]
}

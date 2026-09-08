#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "migrate: refuses working-tree config includes without moving files" {
	init_repo repo
	cd repo
	command git config --file remote.config remote.included.url "$TEST_DIR/remote"
	command git config include.path ../remote.config
	[ "$(command git remote)" = included ]
	cp .git/config "$TEST_DIR/original-config"
	run "$GIT_WT" migrate --dry-run
	[ "$status" -ne 0 ]
	[[ "$output" == *"unsupported config include"* ]]
	[[ "$output" == *"../remote.config"* ]]
	cmp .git/config "$TEST_DIR/original-config"
	[ -d .git ]
	[ -f remote.config ]
	[ ! -e .bare ]
	[ "$(command git remote)" = included ]
}

@test "migrate: refuses escaping nested includes and absolute paths into the old repository" {
	init_repo repo
	cd repo
	mkdir .git/settings
	command git config --file .git/settings/first.config include.path ../../remote.config
	command git config --file remote.config custom.keep value
	command git config include.path settings/first.config
	run "$GIT_WT" migrate --dry-run
	[ "$status" -ne 0 ]
	[[ "$output" == *"unsupported config include"* ]]
	command git config include.path "$(pwd -P)/.git/settings/first.config"
	run "$GIT_WT" migrate --dry-run
	[ "$status" -ne 0 ]
	[[ "$output" == *"unsupported config include"* ]]
	[ -d .git ]
	[ ! -e .bare ]
}

@test "migrate: preserves internal nested includes and stable external includes" {
	init_repo_with_remote repo
	cd repo
	mkdir .git/settings
	command git config --file .git/settings/first.config include.path remote.config
	command git config --file .git/settings/remote.config remote.included.url "$TEST_DIR/repo-origin"
	command git config --file .git/settings/remote.config custom.multiline $'first\nsecond'
	command git config include.path settings/first.config
	command git config --file "$TEST_DIR/shared.config" custom.external value
	command git config --add include.path "$TEST_DIR/shared.config"
	command git config --add custom.ordered first
	command git config --add custom.ordered second
	run bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	for location in . main; do
		[ "$(command git -C "$location" config custom.external)" = value ]
		[ "$(command git -C "$location" config custom.multiline)" = $'first\nsecond' ]
		[ "$(command git -C "$location" config --get-all custom.ordered)" = $'first\nsecond' ]
		[ "$(command git -C "$location" remote)" = $'included\norigin' ]
		[ "$(command git -C "$location" remote get-url included)" = "$TEST_DIR/repo-origin" ]
	done
	command git -C main fetch included
}

@test "migrate: honors a stable external include supplied as a one-off override" {
	init_repo repo
	cd repo
	command git config --file "$TEST_DIR/shared.config" custom.marker value
	run env GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=include.path GIT_CONFIG_VALUE_0="$TEST_DIR/shared.config" bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[ -d .bare ]
	run command git -C main config --get include.path
	[ "$status" -eq 1 ]
}

@test "migrate: refuses conditional settings lost during preparation" {
	init_repo repo
	cd repo
	command git config --file "$TEST_DIR/shared.config" custom.secret do-not-print-this
	command git config "includeIf.gitdir:$(pwd -P)/.git.path" "$TEST_DIR/shared.config"
	[ "$(command git config custom.secret)" = do-not-print-this ]
	run bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -ne 0 ]
	[[ "$output" == *"configuration verification failed"* ]]
	[[ "$output" != *"do-not-print-this"* ]]
	[ -d .git ]
	[ ! -e .bare ]
	[ "$(command git config custom.secret)" = do-not-print-this ]
}

@test "migrate: rolls back conditional remotes lost only after promotion" {
	init_repo_with_remote repo
	cd repo
	root=$(pwd -P)
	command git config --file "$TEST_DIR/shared.config" remote.included.url "$TEST_DIR/repo-origin"
	command git config "includeIf.gitdir:$root/.git.path" "$TEST_DIR/shared.config"
	command git config "includeIf.gitdir:$(dirname "$root")/repo-new-*/.path" "$TEST_DIR/shared.config"
	[ "$(command git remote)" = $'included\norigin' ]
	run bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -ne 0 ]
	[[ "$output" == *"effective remotes changed"* ]]
	[[ "$output" == *"original repository restored"* ]]
	[ -d .git ]
	[ ! -e .bare ]
	[ "$(command git remote)" = $'included\norigin' ]
}

#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "list: passes through native output" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree feature-a feature-a
	create_worktree feature-b feature-b

	run command git worktree list
	[ "$status" -eq 0 ]
	expected="$output"

	run "$GIT_WT" list
	[ "$status" -eq 0 ]
	[ "$output" = "$expected" ]
}

@test "list: passes through detached worktrees" {
	init_bare_repo myrepo
	cd myrepo
	local sha
	sha=$(command git rev-parse HEAD)
	command git worktree add --detach detached "$sha" --quiet

	run command git worktree list
	[ "$status" -eq 0 ]
	expected="$output"

	run "$GIT_WT" list
	[ "$status" -eq 0 ]
	[ "$output" = "$expected" ]
}

@test "list: fails outside git repo" {
	run "$GIT_WT" list
	[ "$status" -ne 0 ]
}

@test "list: --help describes native output and JSON" {
	init_bare_repo myrepo
	cd myrepo

	run "$GIT_WT" list --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"--json"* ]]
	[[ "$output" == *"ls"* ]]
}

@test "list: works from worktree subdirectory" {
	init_bare_repo myrepo
	cd myrepo
	command git worktree add main HEAD --quiet 2>/dev/null
	create_worktree feature-list feature-list
	mkdir -p main/src
	cd main/src

	run command git worktree list
	[ "$status" -eq 0 ]
	expected="$output"

	run "$GIT_WT" list
	[ "$status" -eq 0 ]
	[ "$output" = "$expected" ]
}

@test "list: still lists worktrees in DEBUG mode" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree feature-debug feature-debug

	run command git worktree list
	[ "$status" -eq 0 ]
	expected="$output"

	run env DEBUG=1 "$GIT_WT" list
	[ "$status" -eq 0 ]
	[ "$output" = "$expected" ]
}

@test "list: passes through native flags" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree feature-porcelain feature-porcelain

	run command git worktree list --porcelain
	[ "$status" -eq 0 ]
	expected="$output"

	run "$GIT_WT" list --porcelain
	[ "$status" -eq 0 ]
	[ "$output" = "$expected" ]
}

@test "list: JSON emits an empty array for a bare-only repository" {
	init_bare_repo myrepo
	cd myrepo
	for subcommand in list ls; do
		run "$GIT_WT" "$subcommand" --json
		[ "$status" -eq 0 ]
		[ "$output" = '[]' ]
	done
}

@test "ls: passes through native output and NUL-delimited flags" {
	init_bare_repo myrepo
	cd myrepo
	create_worktree $'feature\nwith\ttabs' feature
	run command git worktree list
	[ "$status" -eq 0 ]
	expected="$output"
	run "$GIT_WT" ls
	[ "$status" -eq 0 ]
	[ "$output" = "$expected" ]
	command git worktree list --porcelain -z >"$TEST_DIR/native"
	for subcommand in list ls; do
		"$GIT_WT" "$subcommand" --porcelain -z >"$TEST_DIR/wrapped"
		cmp "$TEST_DIR/native" "$TEST_DIR/wrapped"
	done
}

@test "list: JSON preserves full IDs paths and worktree metadata" {
	init_bare_repo myrepo
	cd myrepo
	command git worktree add main main --quiet
	path=$'space "quote"\tand\nnewline'
	create_worktree "$path" feature/nested
	command git worktree lock --reason $'maintenance\nsecond line' "$path"
	command git worktree add --detach detached HEAD --quiet
	create_worktree stale stale
	rm -rf stale
	"$GIT_WT" list --json >"$TEST_DIR/list.json"
	"$GIT_WT" ls --json >"$TEST_DIR/ls.json"
	cmp "$TEST_DIR/list.json" "$TEST_DIR/ls.json"
	python3 - "$TEST_DIR/list.json" "$TEST_DIR/myrepo" "$path" "$(command git rev-parse HEAD)" <<'PY'
import json
import pathlib
import sys

filename, root, unusual, head = sys.argv[1:]
rows = json.loads(pathlib.Path(filename).read_text())
assert len(rows) == 4, rows
by_path = {row['path']: row for row in rows}
assert root + '/.bare' not in by_path
for row in rows:
    assert set(row) == {'path', 'branch', 'head', 'detached', 'locked', 'locked_reason', 'prunable', 'prunable_reason'}
    assert row['head'] == head
    assert pathlib.Path(row['path']).is_absolute()
assert by_path[root + '/main']['branch'] == 'main'
assert by_path[root + '/main']['locked_reason'] == ''
assert by_path[root + '/' + unusual]['branch'] == 'feature/nested'
assert by_path[root + '/' + unusual]['locked'] is True
assert by_path[root + '/' + unusual]['locked_reason'] == 'maintenance\nsecond line'
assert by_path[root + '/detached']['branch'] == ''
assert by_path[root + '/detached']['detached'] is True
assert by_path[root + '/stale']['prunable'] is True
assert by_path[root + '/stale']['prunable_reason']
PY
	mkdir -p main/src
	cd main/src
	DEBUG=1 "$GIT_WT" ls --json >"$TEST_DIR/subdir.json"
	cmp "$TEST_DIR/list.json" "$TEST_DIR/subdir.json"
}

@test "list: JSON errors leave stdout empty outside a repository" {
	for subcommand in list ls; do
		run bash -c '"$1" "$2" --json >"$3" 2>"$4"' _ "$GIT_WT" "$subcommand" "$TEST_DIR/stdout" "$TEST_DIR/stderr"
		[ "$status" -ne 0 ]
		[ ! -s "$TEST_DIR/stdout" ]
		[ -s "$TEST_DIR/stderr" ]
	done
}

@test "list: JSON rejects conflicting native options" {
	init_bare_repo myrepo
	cd myrepo
	for subcommand in list ls; do
		for option in --porcelain -z --verbose --expire=now; do
			run "$GIT_WT" "$subcommand" --json "$option"
			[ "$status" -ne 0 ]
			[[ "$output" == *"cannot be combined"* ]]
		done
	done
}

@test "list: explicit false keeps native passthrough and unknown flags reach Git" {
	init_bare_repo myrepo
	cd myrepo
	command git worktree list --porcelain >"$TEST_DIR/native"
	"$GIT_WT" ls --json=false --porcelain >"$TEST_DIR/wrapped"
	cmp "$TEST_DIR/native" "$TEST_DIR/wrapped"
	run "$GIT_WT" ls --not-a-git-option
	[ "$status" -ne 0 ]
	[[ "$output" == *"usage: git worktree list"* ]]
}

@test "list: JSON includes standard worktrees even when named .bare" {
	init_repo .bare
	cd .bare
	run "$GIT_WT" ls --json
	[ "$status" -eq 0 ]
	printf '%s' "$output" | python3 -c 'import json,sys; rows=json.load(sys.stdin); assert len(rows)==1 and rows[0]["branch"]=="main"'
}

@test "list: JSON keeps full SHA-256 object IDs" {
	GIT_DEFAULT_HASH=sha256 init_bare_repo myrepo
	cd myrepo
	create_worktree feature feature
	head=$(command git rev-parse feature)
	[ "${#head}" -eq 64 ]
	"$GIT_WT" ls --json | python3 -c 'import json,sys; rows=json.load(sys.stdin); assert len(rows)==1 and rows[0]["head"]==sys.argv[1]' "$head"
}

@test "ls: works through git wt and completes the JSON flag" {
	init_bare_repo myrepo
	cd myrepo
	mkdir -p "$TEST_DIR/bin"
	ln -s "$GIT_WT" "$TEST_DIR/bin/git-wt"
	run env PATH="$TEST_DIR/bin:$PATH" git wt ls --json
	[ "$status" -eq 0 ]
	[ "$output" = '[]' ]
	run "$GIT_WT" ls --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"--json"* ]]
	run "$GIT_WT" __complete ls --j
	[ "$status" -eq 0 ]
	[[ "$output" == *"--json"* ]]
}

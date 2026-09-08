#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

assert_late_clone_failure() {
	bats_require_minimum_version 1.5.0
	local step="$1"
	init_repo source
	export REAL_GIT
	REAL_GIT=$(command -v git)
	mkdir "$TEST_DIR/bin"
	cat >"$TEST_DIR/bin/git" <<'SH'
#!/bin/sh
if [ "$1" = "$FAIL_CLONE_STEP" ]; then
	echo "injected $FAIL_CLONE_STEP failure" >&2
	exit 1
fi
exec "$REAL_GIT" "$@"
SH
	chmod +x "$TEST_DIR/bin/git"
	run --separate-stderr env PATH="$TEST_DIR/bin:$PATH" FAIL_CLONE_STEP="$step" NO_COLOR=1 "$GIT_WT" clone "$TEST_DIR/source" "retained clone"
	[ "$status" -ne 0 ]
	[[ "$stderr" == *"Warning: Repository downloaded and retained at $TEST_DIR/retained clone"* ]]
	[[ "$stderr" == *"Inspect the downloaded branches"* ]]
	[[ "$stderr" == *"wt add <path> <branch>"* ]]
	[ -f "$TEST_DIR/retained clone/.git" ]
	command git --git-dir="$TEST_DIR/retained clone/.bare" cat-file -e "$(command git -C source rev-parse HEAD)"
}

@test "clone: configuration failure retains the download and warns" {
	assert_late_clone_failure config
}

@test "clone: fetch failure retains the download and warns" {
	assert_late_clone_failure fetch
}

@test "clone: worktree failure retains the download and warns" {
	assert_late_clone_failure worktree
}

@test "clone: mistyped branch at the prompt retains the download" {
	init_repo source
	command git -C source symbolic-ref HEAD refs/heads/missing
	run bash -c 'printf "typo\n" | "$1" clone "$2" "$3"' _ "$GIT_WT" "$TEST_DIR/source" "$TEST_DIR/retained"
	[ "$status" -ne 0 ]
	[[ "$output" == *"Warning:"*"retained"* ]]
	[[ "$output" == *"wt add <path> <branch>"* ]]
	command git --git-dir="$TEST_DIR/retained/.bare" cat-file -e "$(command git -C source rev-parse main)"
}

@test "clone: clones repo with bare structure" {
	# Create a source repo to clone from
	init_repo source-repo
	cd source-repo
	create_commit "file.txt"
	cd "$TEST_DIR"

	run "$GIT_WT" clone "$TEST_DIR/source-repo" cloned-repo
	[ "$status" -eq 0 ]
	[ -d "$TEST_DIR/cloned-repo" ]
	[ -d "$TEST_DIR/cloned-repo/.bare" ]
	[ -f "$TEST_DIR/cloned-repo/.git" ]
}

@test "clone: creates main worktree" {
	init_repo source-repo
	cd source-repo
	create_commit "file.txt"
	cd "$TEST_DIR"

	run "$GIT_WT" clone "$TEST_DIR/source-repo" cloned-with-main
	[ "$status" -eq 0 ]

	cd "$TEST_DIR/cloned-with-main"
	run command git worktree list
	[ "$status" -eq 0 ]
	# Should have a worktree for the default branch
	[[ "$output" == *"main"* ]] || [[ "$output" == *"master"* ]]
}

@test "clone: --help shows usage" {
	run "$GIT_WT" clone --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Usage"* ]]
}

@test "clone: fails without repository argument" {
	run "$GIT_WT" clone
	[ "$status" -ne 0 ]
}

@test "clone: fails when target directory already exists" {
	mkdir -p "$TEST_DIR/existing-dir"
	echo "important" >"$TEST_DIR/existing-dir/data.txt"

	init_repo source-repo
	cd source-repo
	create_commit "file.txt"
	cd "$TEST_DIR"

	run "$GIT_WT" clone "$TEST_DIR/source-repo" existing-dir
	[ "$status" -ne 0 ]
	[[ "$output" == *"already exists"* ]]
	# Existing contents must not be touched
	[ -f "$TEST_DIR/existing-dir/data.txt" ]
	[[ "$(cat "$TEST_DIR/existing-dir/data.txt")" == "important" ]]
}

@test "clone: .git file contains correct gitdir path" {
	init_repo source-gitdir
	cd source-gitdir
	create_commit "file.txt"
	cd "$TEST_DIR"

	"$GIT_WT" clone "$TEST_DIR/source-gitdir" gitdir-test
	[ -f "$TEST_DIR/gitdir-test/.git" ]

	local gitdir_content
	gitdir_content=$(cat "$TEST_DIR/gitdir-test/.git")
	[[ "$gitdir_content" == "gitdir: ./.bare" ]]
}

@test "clone: prints layout and next steps" {
	init_repo source-ui
	cd source-ui
	create_commit "file.txt"
	cd "$TEST_DIR"

	run env NO_COLOR=1 "$GIT_WT" clone "$TEST_DIR/source-ui" clone-ui
	[ "$status" -eq 0 ]
	[[ "$output" == *"clone-ui/.bare"* ]]
	[[ "$output" == *"clone-ui/.git"* ]]
	[[ "$output" == *"cd clone-ui && git wt add"* ]]
}

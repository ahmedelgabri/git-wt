#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

# Simulate old Git and reject resets outright, so a fast-path test cannot pass
# merely because the underlying Git happens to support empty-value resets.
legacy_git() {
	export REAL_GIT="$(command -v git)"
	mkdir -p "$TEST_DIR/bin"
	cat >"$TEST_DIR/bin/git" <<'SCRIPT'
#!/usr/bin/env bash
if [[ "$1" = --version ]]; then
	printf 'git version 2.39.5 (Apple Git-154)\n'
	exit 0
fi
for arg in "$@"; do
	case "$arg" in
		remote.*.url=|remote.*.pushurl=)
			echo 'unsupported empty URL reset' >&2
			exit 99
			;;
	esac
done
exec "$REAL_GIT" "$@"
SCRIPT
	chmod +x "$TEST_DIR/bin/git"
	export PATH="$TEST_DIR/bin:$PATH"
}

remote_compat_fixture() {
	init_bare_repo_with_remote repo
	cd repo
	create_worktree feature feature
	command git push --quiet -u origin feature
	command git config wt.beforeremove 'touch "$TEST_DIR/hook-ran"'
}

@test "remove: old Git supports one matching rewritten URL with named transport settings" {
	remote_compat_fixture
	command git config "url.$TEST_DIR/repo-origin.insteadOf" alias:repo
	command git config remote.origin.url alias:repo
	command git config remote.origin.pushurl "$TEST_DIR/repo-origin"
	for operation in upload-pack receive-pack; do
		printf '#!/bin/sh\necho called >>"%s"\nexec git %s "$@"\n' "$TEST_DIR/$operation-called" "$operation" >"$TEST_DIR/$operation"
		chmod +x "$TEST_DIR/$operation"
	done
	command git config remote.origin.uploadpack "$TEST_DIR/upload-pack"
	command git config remote.origin.receivepack "$TEST_DIR/receive-pack"
	legacy_git
	run bash -c 'printf "feature\n" | "$1" remove feature --delete-remote' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[ ! -d feature ]
	assert_branch_not_exists feature
	! command git -C "$TEST_DIR/repo-origin" show-ref --verify refs/heads/feature
	[ -f "$TEST_DIR/hook-ran" ]
	[ -f "$TEST_DIR/upload-pack-called" ]
	[ -f "$TEST_DIR/receive-pack-called" ]
}

remote_compat_refusal() {
	local mode="$1"
	remote_compat_fixture
	command git init --bare --quiet -b main "$TEST_DIR/push-only"
	command git push --quiet "$TEST_DIR/push-only" feature
	if [[ "$mode" = multiple ]]; then
		command git config --add remote.origin.pushurl "$TEST_DIR/repo-origin"
	fi
	command git config --add remote.origin.pushurl "$TEST_DIR/push-only"
	local head_before
	head_before=$(command git rev-parse feature)
	cp .bare/config "$TEST_DIR/config-before"
	legacy_git
	run bash -c 'printf "feature\n" | "$1" remove feature --delete-remote' _ "$GIT_WT"
	[ "$status" -ne 0 ]
	[[ "$output" == *"remote origin has $mode"* ]]
	[[ "$output" == *"Git 2.46 or newer"* ]]
	[[ "$output" == *"without --delete-remote"* ]]
	[[ "$output" == *"native Git"* ]]
	assert_worktree_exists "$TEST_DIR/repo/feature"
	[ -d feature ]
	[ "$(command git rev-parse feature)" = "$head_before" ]
	[ "$(command git -C "$TEST_DIR/repo-origin" rev-parse feature)" = "$head_before" ]
	[ "$(command git -C "$TEST_DIR/push-only" rev-parse feature)" = "$head_before" ]
	cmp .bare/config "$TEST_DIR/config-before"
	[ ! -e "$TEST_DIR/hook-ran" ]
	# A non-mutating plan is still available on old Git.
	run "$GIT_WT" remove feature --delete-remote --dry-run
	[ "$status" -eq 0 ]
	[[ "$output" == *"No changes made"* ]]
}

@test "remove: old Git refuses a differing push URL before hooks or local changes" {
	remote_compat_refusal differing
}

@test "remove: old Git refuses multiple push URLs before hooks or local changes" {
	remote_compat_refusal multiple
}

@test "remove: checks compatibility for the entire selection before the first removal" {
	remote_compat_fixture
	create_worktree second second
	command git remote add other "$TEST_DIR/repo-origin"
	command git push --quiet -u other second
	command git config --add remote.other.pushurl "$TEST_DIR/repo-origin"
	command git config --add remote.other.pushurl "$TEST_DIR/another-destination"
	cp .bare/config "$TEST_DIR/config-before"
	legacy_git
	run bash -c 'printf "remove\n" | "$1" remove feature second --delete-remote' _ "$GIT_WT"
	[ "$status" -ne 0 ]
	[[ "$output" == *"remote other has multiple push URLs"* ]]
	[ -d feature ]
	[ -d second ]
	assert_branch_exists feature
	assert_branch_exists second
	cmp .bare/config "$TEST_DIR/config-before"
	[ ! -e "$TEST_DIR/hook-ran" ]
}

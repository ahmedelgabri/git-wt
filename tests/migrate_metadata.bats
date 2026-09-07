#!/usr/bin/env bats

load test_helper

setup() {
	setup_test_env
}

teardown() {
	teardown_test_env
}

@test "migrate: restores worktree pseudorefs, reflogs, and private ref namespaces" {
	init_repo repo
	cd repo
	first=$(command git rev-parse HEAD)
	create_commit tracked.txt
	command git update-ref --create-reflog -m saved ORIG_HEAD "$first"
	printf '%s\t\tfetched branch\n' "$first" >.git/FETCH_HEAD
	printf 'bisect history\n' >.git/BISECT_LOG
	mkdir -p .git/tool-state
	printf 'custom state\n' >.git/tool-state/saved
	for namespace in bisect worktree rewritten; do
		command git update-ref --create-reflog -m saved "refs/$namespace/saved" "$first"
		command git reflog show --format='%H %gs' "refs/$namespace/saved" >"$TEST_DIR/$namespace-log"
	done
	command git symbolic-ref refs/worktree/alias refs/worktree/saved
	command git reflog show --format='%H %gs' HEAD >"$TEST_DIR/head-log"
	command git reflog show --format='%H %gs' ORIG_HEAD >"$TEST_DIR/orig-log"
	chmod 700 .git .git/logs .git/refs
	run bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	gitdir=$(command git -C main rev-parse --absolute-git-dir)
	command git -C main reflog show --format='%H %gs' HEAD >"$TEST_DIR/actual-head-log"
	command git -C main reflog show --format='%H %gs' ORIG_HEAD >"$TEST_DIR/actual-orig-log"
	cmp "$TEST_DIR/head-log" "$TEST_DIR/actual-head-log"
	cmp "$TEST_DIR/orig-log" "$TEST_DIR/actual-orig-log"
	[ "$(command git -C main rev-parse ORIG_HEAD)" = "$first" ]
	[ "$(command git -C main rev-parse FETCH_HEAD)" = "$first" ]
	[ -f "$gitdir/BISECT_LOG" ]
	[ -f "$gitdir/tool-state/saved" ]
	for namespace in bisect worktree rewritten; do
		[ "$(command git -C main rev-parse "refs/$namespace/saved")" = "$first" ]
		command git -C main reflog show --format='%H %gs' "refs/$namespace/saved" >"$TEST_DIR/actual-$namespace-log"
		cmp "$TEST_DIR/$namespace-log" "$TEST_DIR/actual-$namespace-log"
	done
	[ "$(command git -C main symbolic-ref refs/worktree/alias)" = refs/worktree/saved ]
	python3 - "$gitdir" <<'PY'
import os, stat, sys
for suffix in ['', 'logs', 'refs']:
    assert stat.S_IMODE(os.stat(os.path.join(sys.argv[1], suffix)).st_mode) == 0o700
PY
}

@test "migrate: exposes originally packed per-worktree refs through the linked worktree" {
	init_repo repo
	cd repo
	head=$(command git rev-parse HEAD)
	command git update-ref --create-reflog -m saved refs/worktree/saved "$head"
	command git reflog show --format='%H %gs' refs/worktree/saved >"$TEST_DIR/saved-log"
	printf '# pack-refs with: sorted\n%s refs/heads/main\n%s refs/worktree/saved\n' "$head" "$head" >.git/packed-refs
	rm .git/refs/worktree/saved
	run bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[ "$(command git -C main rev-parse refs/worktree/saved)" = "$head" ]
	command git -C main reflog show --format='%H %gs' refs/worktree/saved >"$TEST_DIR/actual-log"
	cmp "$TEST_DIR/saved-log" "$TEST_DIR/actual-log"
}

@test "migrate: preserves extended attributes on working files and Git metadata" {
	init_repo repo
	cd repo
	create_commit tracked.txt
	printf 'local secret\n' >ignored.txt
	printf 'ignored.txt\n' >.git/info/exclude
	python3 - <<'PY'
import os, platform, subprocess
value = b'value\x00with\nbytes'
for path in ['.', 'tracked.txt', 'ignored.txt', '.git', '.git/config', '.git/index', '.git/objects', '.git/logs', '.git/logs/HEAD']:
    if platform.system() == 'Darwin':
        subprocess.run(['/usr/bin/xattr', '-wx', 'user.git-wt-test', value.hex(), path], check=True)
    else:
        os.setxattr(path, 'user.git-wt-test', value, follow_symlinks=False)
PY
	run bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	gitdir=$(command git -C main rev-parse --absolute-git-dir)
	python3 - "$gitdir" <<'PY'
import os, platform, subprocess, sys
paths = ['.', 'main', 'main/tracked.txt', 'main/ignored.txt', '.bare', '.bare/config', '.bare/objects']
paths += [os.path.join(sys.argv[1], suffix) for suffix in ['', 'index', 'logs', 'logs/HEAD']]
for path in paths:
    if platform.system() == 'Darwin':
        value = bytes.fromhex(subprocess.check_output(['/usr/bin/xattr', '-px', 'user.git-wt-test', path], text=True))
    else:
        value = os.getxattr(path, 'user.git-wt-test', follow_symlinks=False)
    assert value == b'value\x00with\nbytes', path
PY
}

migration_acl_refusal() {
	local target="$1"
	init_repo repo
	cd repo
	create_commit tracked.txt
	python3 - "$target" <<'PY'
import os, platform, struct, subprocess, sys
path = sys.argv[1]
if platform.system() == 'Darwin':
    subprocess.run(['/bin/chmod', '+a', 'everyone allow read', path], check=True)
else:
    entries = [(1, 6, 0xffffffff), (2, 4, 12345), (4, 4, 0xffffffff), (16, 4, 0xffffffff), (32, 0, 0xffffffff)]
    acl = struct.pack('<I', 2) + b''.join(struct.pack('<HHI', *entry) for entry in entries)
    name = 'system.posix_acl_default' if os.path.isdir(path) else 'system.posix_acl_access'
    os.setxattr(path, name, acl)
PY
	run "$GIT_WT" migrate --dry-run
	[ "$status" -ne 0 ]
	[[ "$output" == *"unsupported ACL metadata"* ]]
	[ -d .git ]
	[ ! -e .bare ]
	[ -f tracked.txt ]
}

@test "migrate: refuses file ACLs without restructuring" {
	migration_acl_refusal tracked.txt
}

@test "migrate: refuses root directory ACLs without restructuring" {
	migration_acl_refusal .
}

@test "migrate: refuses ACLs inside the Git database" {
	migration_acl_refusal .git/config
}

@test "migrate: metadata restoration does not execute native Git hooks" {
	init_repo repo
	cd repo
	command git update-ref refs/worktree/saved HEAD
	for event in reference-transaction post-index-change; do
		printf '#!/bin/sh\ntouch "%s"\n' "$TEST_DIR/hook-ran" >".git/hooks/$event"
		chmod +x ".git/hooks/$event"
	done
	printf '#!/bin/sh\ntouch "%s"\nprintf "token\\0/\\0"\n' "$TEST_DIR/hook-ran" >"$TEST_DIR/fsmonitor"
	chmod +x "$TEST_DIR/fsmonitor"
	command git config core.fsmonitor "$TEST_DIR/fsmonitor"
	command git config core.fsmonitorHookVersion 2
	run bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[ ! -e "$TEST_DIR/hook-ran" ]
	[ "$(command git -C main config core.fsmonitor)" = "$TEST_DIR/fsmonitor" ]
	[ -x .bare/hooks/reference-transaction ]
	[ -x .bare/hooks/post-index-change ]
}

@test "migrate: refuses reftable rather than losing private reflog state" {
	run command git init --quiet --ref-format=reftable -b main repo
	if [ "$status" -ne 0 ]; then
		skip "Git does not support reftable"
	fi
	cd repo
	command git -c user.name=Test -c user.email=test@test.com commit --quiet --allow-empty -m initial
	run "$GIT_WT" migrate --dry-run
	[ "$status" -ne 0 ]
	[[ "$output" == *"does not support the reftable ref storage format"* ]]
	[ -d .git ]
	[ ! -e .bare ]
}

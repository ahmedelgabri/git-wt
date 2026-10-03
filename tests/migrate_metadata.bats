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
	[ ! -e .bare/BISECT_LOG ]
	# Unknown directories stay shared, where tools such as git-lfs look for them.
	[ -f .bare/tool-state/saved ]
	[ ! -e "$gitdir/tool-state" ]
	for namespace in bisect worktree rewritten; do
		[ "$(command git -C main rev-parse "refs/$namespace/saved")" = "$first" ]
		command git -C main reflog show --format='%H %gs' "refs/$namespace/saved" >"$TEST_DIR/actual-$namespace-log"
		cmp "$TEST_DIR/$namespace-log" "$TEST_DIR/actual-$namespace-log"
	done
	[ "$(command git -C main symbolic-ref refs/worktree/alias)" = refs/worktree/saved ]
	[ "$(stat -c %a .bare 2>/dev/null || stat -f %Lp .bare)" = 700 ]
}

@test "migrate: isolates packed private refs from existing and future worktrees" {
	init_repo_with_remote repo
	cd repo
	command git checkout --quiet -b feature
	head=$(command git rev-parse HEAD)
	command git tag -a saved-tag -m saved "$head"
	tag=$(command git rev-parse saved-tag)
	for namespace in bisect worktree rewritten; do
		command git update-ref --create-reflog -m saved "refs/$namespace/saved" "$head"
		command git reflog show --format='%H %gs' "refs/$namespace/saved" >"$TEST_DIR/$namespace-log"
		rm ".git/refs/$namespace/saved"
	done
	printf '# pack-refs with: peeled fully-peeled sorted\n%s refs/bisect/saved\n%s refs/heads/feature\n%s refs/heads/main\n%s refs/rewritten/saved\n%s refs/tags/saved-tag\n^%s\n%s refs/worktree/saved\n' "$head" "$head" "$head" "$head" "$tag" "$head" "$head" >.git/packed-refs
	command git symbolic-ref refs/worktree/alias refs/worktree/saved
	chmod 444 .git/packed-refs
	run bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	command git worktree add --quiet -b future future main
	gitdir=$(command git -C feature rev-parse --absolute-git-dir)
	for namespace in bisect worktree rewritten; do
		[ "$(command git -C feature rev-parse "refs/$namespace/saved")" = "$head" ]
		[ -f "$gitdir/refs/$namespace/saved" ]
		command git -C feature reflog show --format='%H %gs' "refs/$namespace/saved" >"$TEST_DIR/actual-log"
		cmp "$TEST_DIR/$namespace-log" "$TEST_DIR/actual-log"
		for other in . main future; do
			run command git -C "$other" rev-parse --verify "refs/$namespace/saved"
			[ "$status" -ne 0 ]
		done
	done
	[ "$(command git -C feature symbolic-ref refs/worktree/alias)" = refs/worktree/saved ]
	run command git -C main rev-parse --verify refs/worktree/alias
	[ "$status" -ne 0 ]
	! grep -E ' refs/(worktree|bisect|rewritten)/' .bare/packed-refs
	[ "$(command git -C main rev-parse 'saved-tag^{}')" = "$head" ]
	[ "$(command git -C main rev-parse saved-tag)" = "$tag" ]
}

@test "migrate: handles a packed file containing only private refs and a loose shadow" {
	init_repo repo
	cd repo
	first=$(command git rev-parse HEAD)
	create_commit tracked.txt
	head=$(command git rev-parse HEAD)
	printf '# pack-refs with: sorted\n%s refs/worktree/saved\n' "$first" >.git/packed-refs
	command git update-ref refs/worktree/saved "$head"
	run bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[ "$(command git -C main rev-parse refs/worktree/saved)" = "$head" ]
	command git worktree add --quiet -b other other main
	run command git -C other rev-parse --verify refs/worktree/saved
	[ "$status" -ne 0 ]
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
	# Paths are moved, so they keep their attributes. Git itself rewrites
	# .git/config, and the worktree directories are new.
	python3 - "$gitdir" <<'PY'
import os, platform, subprocess, sys
paths = ['.', 'main/tracked.txt', 'main/ignored.txt', '.bare', '.bare/objects', '.bare/logs']
paths += [os.path.join(sys.argv[1], suffix) for suffix in ['index', 'logs/HEAD']]
for path in paths:
    if platform.system() == 'Darwin':
        value = bytes.fromhex(subprocess.check_output(['/usr/bin/xattr', '-px', 'user.git-wt-test', path], text=True))
    else:
        value = os.getxattr(path, 'user.git-wt-test', follow_symlinks=False)
    assert value == b'value\x00with\nbytes', path
PY
}

migration_acl_preserved() {
	local target="$1"
	init_repo repo
	cd repo
	create_commit tracked.txt
	local after="${2:-$target}"
	python3 - "$target" <<'PY' || skip "filesystem does not support ACLs"
import os, platform, struct, subprocess, sys
path = sys.argv[1]
if platform.system() == 'Darwin':
    subprocess.run(['/bin/chmod', '+a', 'everyone allow read', path], check=True)
else:
    # Directories need execute permission, or Git cannot use what it creates there.
    owner, named = (7, 5) if os.path.isdir(path) else (6, 4)
    entries = [(1, owner, 0xffffffff), (2, named, 12345), (4, named, 0xffffffff), (16, named, 0xffffffff), (32, 0, 0xffffffff)]
    acl = struct.pack('<I', 2) + b''.join(struct.pack('<HHI', *entry) for entry in entries)
    name = 'system.posix_acl_default' if os.path.isdir(path) else 'system.posix_acl_access'
    os.setxattr(path, name, acl)
PY
	run bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	python3 - "$after" <<'PY'
import os, platform, subprocess, sys
path = sys.argv[1]
if platform.system() == 'Darwin':
    assert 'group:everyone allow' in subprocess.check_output(['/bin/ls', '-led', path], text=True), path
else:
    name = 'system.posix_acl_default' if os.path.isdir(path) else 'system.posix_acl_access'
    assert os.getxattr(path, name), path
PY
}

@test "migrate: preserves file ACLs" {
	migration_acl_preserved tracked.txt main/tracked.txt
}

@test "migrate: preserves root directory ACLs" {
	migration_acl_preserved .
}

@test "migrate: preserves ACLs inside the Git database" {
	migration_acl_preserved .git/objects .bare/objects
}

@test "migrate: preserves special mode bits and symlink modes" {
	init_repo repo
	cd repo
	command git config core.sharedRepository group
	mkdir shared sticky
	# macOS refuses setgid when the user is not in the directory's group, as
	# in temporary directories that belong to wheel.
	if ! chmod 2775 shared 2>/dev/null || [ ! -g shared ]; then
		skip "cannot set the setgid bit in this temporary directory"
	fi
	chmod 1777 sticky
	chmod g+s .git/objects/*/
	(umask 077 && ln -s tracked.txt private-link)
	before=$(ls -ld shared sticky private-link | awk '{print $1}')
	run bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	[ "$(cd main && ls -ld shared sticky private-link | awk '{print $1}')" = "$before" ]
}

@test "migrate: keeps unknown Git directories shared without duplicating them" {
	init_repo repo
	cd repo
	mkdir -p .git/lfs/objects/aa
	printf 'large\n' >.git/lfs/objects/aa/blob
	command git update-ref ORIG_HEAD HEAD
	printf 'bisect\n' >.git/BISECT_LOG
	run bash -c 'printf "y\n" | "$1" migrate' _ "$GIT_WT"
	[ "$status" -eq 0 ]
	gitdir=$(command git -C main rev-parse --absolute-git-dir)
	[ -f .bare/lfs/objects/aa/blob ]
	[ ! -e "$gitdir/lfs" ]
	for stale in index ORIG_HEAD BISECT_LOG logs/HEAD; do
		[ ! -e ".bare/$stale" ]
	done
	[ -f "$gitdir/BISECT_LOG" ]
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

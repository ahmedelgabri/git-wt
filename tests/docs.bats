#!/usr/bin/env bats

load test_helper

setup() { setup_test_env; }
teardown() { teardown_test_env; }

@test "docs: documented list examples execute against the current binary" {
	init_bare_repo repo
	cd repo
	run python3 - "$GIT_WT" "$BATS_TEST_DIRNAME/../README.md" "$BATS_TEST_DIRNAME/../docs/index.md" "$BATS_TEST_DIRNAME/../skills/git-wt/SKILL.md" <<'PY'
import pathlib
import re
import shlex
import subprocess
import sys

binary = sys.argv[1]
for filename in sys.argv[2:]:
    text = pathlib.Path(filename).read_text()
    assert 'git wt list --json' not in text, filename
    for command in set(re.findall(r'git wt ([a-z][a-z-]*)', text)):
        subprocess.run([binary, command, '--help'], check=True, capture_output=True)
    for line in text.splitlines():
        if line.startswith('git wt list'):
            subprocess.run([binary, *shlex.split(line)[2:]], check=True, capture_output=True)
PY
	[ "$status" -eq 0 ]
}

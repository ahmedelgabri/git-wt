# Test helper for git-wt bats tests
# Provides setup/teardown functions and utilities for testing

# Path to the git-wt script under test
GIT_WT="${BATS_TEST_DIRNAME}/../git-wt"

# Create a temporary directory for test repos
setup_test_env() {
	# Use realpath to resolve symlinks (macOS /tmp -> /private/tmp)
	TEST_DIR=$(cd "$(mktemp -d)" && pwd -P)
	export TEST_DIR
	cd "$TEST_DIR" || exit 1
}

# Initialize a standard git repo with an initial commit and a local "remote"
# Usage: init_repo_with_remote [dirname]
# Creates both the repo and a bare "origin" to simulate remote operations
init_repo_with_remote() {
	local dirname="${1:-myrepo}"

	# Create a bare repo to act as origin
	mkdir -p "${dirname}-origin"
	(
		cd "${dirname}-origin" || exit 1
		command git init --quiet --bare
	)

	# Create the actual repo and link to origin
	mkdir -p "$dirname"
	(
		cd "$dirname" || exit 1
		command git init --quiet
		command git config user.email "test@test.com"
		command git config user.name "Test User"
		command git remote add origin "../${dirname}-origin"
		command git commit --quiet --allow-empty -m "initial commit"
		command git push --quiet -u origin HEAD 2>/dev/null || true
	)
}

# Clean up temporary directory
teardown_test_env() {
	if [[ -n ${TEST_DIR:-} && -d $TEST_DIR ]]; then
		rm -rf "$TEST_DIR"
	fi
}

# Initialize a standard git repo with an initial commit
# Usage: init_repo [dirname]
# Note: Does NOT change directory - caller must cd if needed
init_repo() {
	local dirname="${1:-.}"
	mkdir -p "$dirname"
	(
		cd "$dirname" || exit 1
		command git init --quiet
		command git config user.email "test@test.com"
		command git config user.name "Test User"
		command git commit --quiet --allow-empty -m "initial commit"
	)
}

# Initialize a bare repo with the .bare directory structure (git-wt style)
# Usage: init_bare_repo [dirname]
# Note: Does NOT change directory - caller must cd if needed
init_bare_repo() {
	local dirname="${1:-.}"
	mkdir -p "$dirname"
	(
		cd "$dirname" || exit 1
		command git init --quiet --bare .bare
		echo "gitdir: ./.bare" >.git
		command git config core.bare false
		command git config user.email "test@test.com"
		command git config user.name "Test User"
		command git commit --quiet --allow-empty -m "initial commit"
	)
}

# Initialize a bare repo with a local "remote" (git-wt style)
# Usage: init_bare_repo_with_remote [dirname]
# Creates both the bare repo and a bare "origin" to simulate remote operations
init_bare_repo_with_remote() {
	local dirname="${1:-myrepo}"

	# Create a bare repo to act as origin
	mkdir -p "${dirname}-origin"
	(
		cd "${dirname}-origin" || exit 1
		command git init --quiet --bare
	)

	# Create the bare repo and link to origin
	mkdir -p "$dirname"
	(
		cd "$dirname" || exit 1
		command git init --quiet --bare .bare
		echo "gitdir: ./.bare" >.git
		command git config core.bare false
		command git config user.email "test@test.com"
		command git config user.name "Test User"
		command git remote add origin "../${dirname}-origin"
		command git commit --quiet --allow-empty -m "initial commit"
		command git push --quiet -u origin HEAD 2>/dev/null || true
	)
}

# Create a worktree with a new branch
# Usage: create_worktree <path> <branch>
create_worktree() {
	local path="$1"
	local branch="$2"
	command git worktree add -b "$branch" "$path" --quiet 2>/dev/null
}

# Create a worktree for an existing branch
# Usage: create_worktree_existing <path> <branch>
create_worktree_existing() {
	local path="$1"
	local branch="$2"
	command git worktree add "$path" "$branch" --quiet 2>/dev/null
}

# Create a file and commit it
# Usage: create_commit <filename> [message]
create_commit() {
	local filename="$1"
	local message="${2:-Add $filename}"
	echo "content of $filename" >"$filename"
	command git add "$filename"
	command git commit --quiet -m "$message"
}

# Get the current branch name
current_branch() {
	command git rev-parse --abbrev-ref HEAD
}

# Get the short SHA of HEAD
head_sha() {
	command git rev-parse --short=7 HEAD
}

# Assert that a worktree exists at path
# Usage: assert_worktree_exists <path>
assert_worktree_exists() {
	local path="$1"
	command git worktree list --porcelain | grep -q "^worktree $path$"
}

# Assert that a worktree does not exist at path
# Usage: assert_worktree_not_exists <path>
assert_worktree_not_exists() {
	local path="$1"
	! command git worktree list --porcelain | grep -q "^worktree $path$"
}

# Assert that a branch exists
# Usage: assert_branch_exists <branch>
assert_branch_exists() {
	local branch="$1"
	command git show-ref --verify --quiet "refs/heads/$branch"
}

# Assert that a branch does not exist
# Usage: assert_branch_not_exists <branch>
assert_branch_not_exists() {
	local branch="$1"
	! command git show-ref --verify --quiet "refs/heads/$branch"
}

# Test helper for git-wt bats tests
# Provides setup/teardown functions and utilities for testing

# Path to the git-wt binary under test
# Use GIT_WT env var if set, otherwise build from source
if [[ -z ${GIT_WT:-} ]]; then
	GIT_WT="${BATS_TEST_DIRNAME}/../git-wt"
	# Build from Go source if the binary doesn't exist or is stale
	if [[ ! -x $GIT_WT ]] || [[ -f "${BATS_TEST_DIRNAME}/../cmd/git-wt/main.go" && "${BATS_TEST_DIRNAME}/../cmd/git-wt/main.go" -nt $GIT_WT ]]; then
		(cd "${BATS_TEST_DIRNAME}/.." && go build -o git-wt ./cmd/git-wt/) || {
			echo "Failed to build git-wt binary" >&2
			exit 1
		}
	fi
fi

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
		command git init --quiet --bare -b main
	)

	# Create the actual repo and link to origin
	mkdir -p "$dirname"
	(
		cd "$dirname" || exit 1
		command git init --quiet -b main
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
		command git init --quiet -b main
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
		command git init --quiet --bare .bare -b main
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
		command git init --quiet --bare -b main
	)

	# Create the bare repo and link to origin
	mkdir -p "$dirname"
	(
		cd "$dirname" || exit 1
		command git init --quiet --bare .bare -b main
		echo "gitdir: ./.bare" >.git
		command git config core.bare false
		command git config user.email "test@test.com"
		command git config user.name "Test User"
		command git remote add origin "../${dirname}-origin"
		command git commit --quiet --allow-empty -m "initial commit"
		command git push --quiet -u origin HEAD 2>/dev/null || true
	)
}

# Initialize a bare repo with a custom-named remote (git-wt style)
# Usage: init_bare_repo_with_custom_remote <remote-name> [dirname]
# Creates both the bare repo and a bare remote with the given name
init_bare_repo_with_custom_remote() {
	local remote_name="${1}"
	local dirname="${2:-myrepo}"

	# Create a bare repo to act as the remote
	mkdir -p "${dirname}-${remote_name}"
	(
		cd "${dirname}-${remote_name}" || exit 1
		command git init --quiet --bare -b main
	)

	# Create the bare repo and link to the remote
	mkdir -p "$dirname"
	(
		cd "$dirname" || exit 1
		command git init --quiet --bare .bare -b main
		echo "gitdir: ./.bare" >.git
		command git config core.bare false
		command git config user.email "test@test.com"
		command git config user.name "Test User"
		command git remote add "$remote_name" "../${dirname}-${remote_name}"
		command git commit --quiet --allow-empty -m "initial commit"
		command git push --quiet -u "$remote_name" HEAD 2>/dev/null || true
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

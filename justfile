# Default: list available recipes
default:
    @just --list

# Build the binary
build:
    go build -o git-wt ./cmd/git-wt/

# Run Go unit tests
test *args:
    go test {{ args }} ./...

# Run Go unit tests with race detector
test-race *args:
    go test -race {{ args }} ./...

# Run E2E tests (bats)
test-e2e *args:
    bats {{ args }} tests/

# Run all tests (unit + E2E)
test-all: test test-e2e

# Run go vet
vet:
    go vet ./...

# Run staticcheck
staticcheck:
    staticcheck ./...

# Run govulncheck
govulncheck:
    govulncheck ./...

# Format all files
fmt:
    nix fmt

# Check formatting without modifying files
fmt-check:
    nix fmt -- --fail-on-change

# Run all checks (lint + tests + race + E2E + format)
check: vet staticcheck test-race test-e2e fmt-check

# Build with Nix
nix-build:
    nix build

# Run nix flake check
nix-check:
    nix flake check

# Remove build artifacts
clean:
    rm -f git-wt

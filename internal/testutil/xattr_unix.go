//go:build darwin || linux

package testutil

import (
	"errors"
	"testing"

	"golang.org/x/sys/unix"
)

// RequireXattrs skips fixtures that cannot be represented by the test filesystem
// (including Linux Nix sandboxes). Call before preparing metadata to preserve;
// copy and verification errors must still fail tests after this probe succeeds.
func RequireXattrs(t *testing.T, path string) {
	t.Helper()
	const name = "user.git-wt-test-probe"
	err := unix.Lsetxattr(path, name, nil, 0)
	if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) {
		t.Skipf("test filesystem does not support xattrs: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Lremovexattr(path, name); err != nil {
		t.Fatal(err)
	}
}

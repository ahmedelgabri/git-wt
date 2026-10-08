package cmd

import (
	"context"
	"fmt"

	"github.com/ahmedelgabri/git-wt/internal/git"
)

func requireGitVersion(ctx context.Context) error {
	version, err := git.QueryContext(ctx, "--version")
	if err != nil {
		return fmt.Errorf("git-wt requires Git 2.48.0 or newer; could not run git --version: %w", err)
	}
	if !gitVersionAtLeast(version, 2, 48) {
		return fmt.Errorf("git-wt requires Git 2.48.0 or newer; found %q. Upgrade Git on PATH", version)
	}
	return nil
}

// gitVersionAtLeast reports whether git --version output is at least
// major.minor. Unparseable versions count as older.
func gitVersionAtLeast(version string, major, minor int) bool {
	var gotMajor, gotMinor, gotPatch int
	if _, err := fmt.Sscanf(version, "git version %d.%d.%d", &gotMajor, &gotMinor, &gotPatch); err != nil {
		return false
	}
	return gotMinor >= 0 && gotPatch >= 0 && (gotMajor > major || gotMajor == major && gotMinor >= minor)
}

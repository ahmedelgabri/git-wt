package cmd

import "fmt"

// gitVersionAtLeast reports whether git --version output is at least
// major.minor. Unparseable versions count as older.
func gitVersionAtLeast(version string, major, minor int) bool {
	var gotMajor, gotMinor int
	if _, err := fmt.Sscanf(version, "git version %d.%d.", &gotMajor, &gotMinor); err != nil {
		return false
	}
	return gotMinor >= 0 && (gotMajor > major || gotMajor == major && gotMinor >= minor)
}

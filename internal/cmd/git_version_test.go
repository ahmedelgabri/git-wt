package cmd

import "testing"

func TestGitVersionAtLeast(t *testing.T) {
	for _, tc := range []struct {
		version string
		want    bool
	}{
		{"git version 2.39.5 (Apple Git-154)", false},
		{"git version 2.45.4", false},
		{"git version 2.46.0", true},
		{"git version 2.46.0.windows.1", true},
		{"git version 2.54.0", true},
		{"git version 3.0.0", true},
		{"git version 3.-1.0", false},
		{"git version 2.46garbage", false},
		{"git version unknown", false},
		{"not a Git version", false},
	} {
		t.Run(tc.version, func(t *testing.T) {
			if got := gitVersionAtLeast(tc.version, 2, 46); got != tc.want {
				t.Fatalf("at least 2.46 = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMigrationRequiresRelativeWorktreeConfigOnlyOnNewerGit(t *testing.T) {
	for _, relative := range []bool{false, true} {
		required := migrationRequiredConfig(migratePlan{relativeWorktrees: relative})
		found := false
		for _, setting := range required {
			if setting[0] == "extensions.relativeworktrees" {
				found = true
			}
		}
		if found != relative {
			t.Fatalf("relativeWorktrees=%v: requires extensions.relativeworktrees = %v", relative, found)
		}
	}
}

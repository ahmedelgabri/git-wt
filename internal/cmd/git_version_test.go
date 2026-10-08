package cmd

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestGitVersionAtLeast(t *testing.T) {
	for _, tc := range []struct {
		version string
		want    bool
	}{
		{"git version 2.36.0", false},
		{"git version 2.38.0", false},
		{"git version 2.39.5 (Apple Git-154)", false},
		{"git version 2.46.0", false},
		{"git version 2.47.9", false},
		{"git version 2.48.0", true},
		{"git version 2.48.0.windows.1", true},
		{"git version 2.48.0 (Apple Git-154)", true},
		{"git version 2.48.1", true},
		{"git version 2.55.0", true},
		{"git version 3.0.0", true},
		{"git version 3.-1.0", false},
		{"git version 2.48.-1", false},
		{"git version 2.48.", false},
		{"git version 2.48garbage", false},
		{"git version unknown", false},
		{"not a Git version", false},
	} {
		t.Run(tc.version, func(t *testing.T) {
			if got := gitVersionAtLeast(tc.version, 2, 48); got != tc.want {
				t.Fatalf("at least 2.48.0 = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRootChecksGitBeforeSubcommands(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if rootCmd.PersistentPreRunE == nil {
		t.Fatal("root command does not check the Git version")
	}
	for _, name := range slices.Concat(supportedCommandNames, passthroughCommandNames) {
		t.Run(name, func(t *testing.T) {
			command, _, err := rootCmd.Find([]string{name})
			if err != nil {
				t.Fatal(err)
			}
			err = rootCmd.PersistentPreRunE(command, nil)
			if err == nil || !strings.Contains(err.Error(), "Git 2.48.0 or newer") || !strings.Contains(err.Error(), "git --version") {
				t.Fatalf("missing Git error = %v", err)
			}
		})
	}
}

func TestRequireGitVersionWithInstalledGit(t *testing.T) {
	t.Setenv("DEBUG", "1")
	if err := requireGitVersion(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRequireGitVersionWithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := requireGitVersion(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled version check = %v", err)
	}
}

func TestRootHelpDoesNotRequireGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := rootCmd.PersistentPreRunE(rootCmd, nil); err != nil {
		t.Fatal(err)
	}
	command, _, err := rootCmd.Find([]string{"list"})
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--help", "-h"} {
		if err := rootCmd.PersistentPreRunE(command, []string{flag}); err != nil {
			t.Fatal(err)
		}
	}
	if err := rootCmd.PersistentPreRunE(command, []string{"--", "--help"}); err == nil {
		t.Fatal("help after the argument terminator bypassed the Git version check")
	}
}

func TestMigrationRequiresRelativeWorktreeConfig(t *testing.T) {
	required := migrationRequiredConfig(migratePlan{})
	for _, setting := range [][2]string{
		{"worktree.userelativepaths", "true"},
		{"core.repositoryformatversion", "1"},
		{"extensions.relativeworktrees", "true"},
	} {
		if !slices.Contains(required, setting) {
			t.Fatalf("migration does not require %v", setting)
		}
	}
}

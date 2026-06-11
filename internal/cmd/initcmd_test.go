package cmd

import (
	"strings"
	"testing"
)

func TestRunInitSupportedShells(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			var sb strings.Builder
			if err := runInit(&sb, shell, true); err != nil {
				t.Fatalf("runInit(%q) returned error: %v", shell, err)
			}
			out := sb.String()
			for _, want := range []string{"git-wt", "switch", "add", "cd "} {
				if !strings.Contains(out, want) {
					t.Errorf("runInit(%q) output missing %q", shell, want)
				}
			}
		})
	}
}

func TestRunInitGitWrapper(t *testing.T) {
	for _, shell := range []string{"bash", "fish"} {
		var with, without strings.Builder
		if err := runInit(&with, shell, true); err != nil {
			t.Fatalf("runInit(%q, true) returned error: %v", shell, err)
		}
		if err := runInit(&without, shell, false); err != nil {
			t.Fatalf("runInit(%q, false) returned error: %v", shell, err)
		}
		if !strings.Contains(with.String(), "command git ") {
			t.Errorf("runInit(%q, true) output missing git() wrapper", shell)
		}
		if strings.Contains(without.String(), "command git $argv\n    end\nend") && shell == "fish" {
			t.Errorf("runInit(%q, false) output should omit git() wrapper", shell)
		}
		if len(without.String()) >= len(with.String()) {
			t.Errorf("runInit(%q, false) output should be shorter than with wrapper", shell)
		}
	}
}

func TestRunInitUnsupportedShell(t *testing.T) {
	var sb strings.Builder
	err := runInit(&sb, "powershell", true)
	if err == nil {
		t.Fatal("runInit(\"powershell\") expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported shell") {
		t.Errorf("error %q should mention unsupported shell", err)
	}
}

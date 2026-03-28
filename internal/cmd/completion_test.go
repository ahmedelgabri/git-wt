package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestGenBashCompletion_containsSubcommandBridge(t *testing.T) {
	var buf bytes.Buffer
	if err := genBashCompletion(rootCmd, &buf); err != nil {
		t.Fatalf("genBashCompletion returned error: %v", err)
	}

	output := buf.String()

	// The _git_wt bridge function definition should appear exactly once.
	if count := strings.Count(output, "_git_wt() {"); count != 1 {
		t.Errorf("expected _git_wt bridge to appear once, got %d", count)
	}

	// It should rewrite COMP_WORDS and delegate to __start_git-wt.
	if !strings.Contains(output, `COMP_WORDS=(git-wt "${COMP_WORDS[@]:2}")`) {
		t.Error("missing COMP_WORDS rewrite in _git_wt bridge")
	}

	if !strings.Contains(output, "__start_git-wt") {
		t.Error("missing __start_git-wt delegation in _git_wt bridge")
	}
}

func TestGenBashCompletion_preservesOriginal(t *testing.T) {
	var buf bytes.Buffer
	if err := genBashCompletion(rootCmd, &buf); err != nil {
		t.Fatalf("genBashCompletion returned error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "complete -o default -F __start_git-wt git-wt") &&
		!strings.Contains(output, "complete -o default -o nospace -F __start_git-wt git-wt") {
		t.Error("missing original complete registration for standalone git-wt")
	}
}

func TestGenZshCompletion_containsSubcommandShim(t *testing.T) {
	var buf bytes.Buffer
	if err := genZshCompletion(rootCmd, &buf); err != nil {
		t.Fatalf("genZshCompletion returned error: %v", err)
	}

	output := buf.String()

	// The shim should appear exactly once inside the _git-wt() function.
	if count := strings.Count(output, `words[1]="git-wt"`); count != 1 {
		t.Errorf("expected subcommand normalization shim to appear once, got %d", count)
	}

	// Verify the shim comes after the function definition and before the
	// first local variable declaration.
	shimIdx := strings.Index(output, `Normalize "wt"`)
	funcIdx := strings.Index(output, "_git-wt()\n{")
	localIdx := strings.Index(output, "local shellCompDirectiveError")

	if shimIdx == -1 || funcIdx == -1 || localIdx == -1 {
		t.Fatal("missing expected sections in zsh completion output")
	}

	if !(funcIdx < shimIdx && shimIdx < localIdx) {
		t.Error("subcommand shim is not in the expected position (after function opening, before locals)")
	}
}

func TestGenZshCompletion_preservesCompdef(t *testing.T) {
	var buf bytes.Buffer
	if err := genZshCompletion(rootCmd, &buf); err != nil {
		t.Fatalf("genZshCompletion returned error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "#compdef git-wt") {
		t.Error("missing #compdef header")
	}

	if !strings.Contains(output, "compdef _git-wt git-wt") {
		t.Error("missing compdef registration")
	}
}

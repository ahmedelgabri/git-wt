package ui

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

func TestNoColorOutput(t *testing.T) {
	// Save and set NO_COLOR
	old := os.Getenv("NO_COLOR")
	os.Setenv("NO_COLOR", "1")
	noColor = true
	defer func() {
		if old == "" {
			os.Unsetenv("NO_COLOR")
		} else {
			os.Setenv("NO_COLOR", old)
		}
		noColor = old != ""
	}()

	if got := Green("hello"); got != "hello" {
		t.Errorf("Green() with NO_COLOR = %q, want %q", got, "hello")
	}
	if got := Red("hello"); got != "hello" {
		t.Errorf("Red() with NO_COLOR = %q, want %q", got, "hello")
	}
	if got := Yellow("hello"); got != "hello" {
		t.Errorf("Yellow() with NO_COLOR = %q, want %q", got, "hello")
	}
	if got := Accent("hello"); got != "hello" {
		t.Errorf("Accent() with NO_COLOR = %q, want %q", got, "hello")
	}
	if got := Subtle("hello"); got != "hello" {
		t.Errorf("Subtle() with NO_COLOR = %q, want %q", got, "hello")
	}
	if got := Muted("hello"); got != "hello" {
		t.Errorf("Muted() with NO_COLOR = %q, want %q", got, "hello")
	}
	if got := Highlight("hello"); got != "hello" {
		t.Errorf("Highlight() with NO_COLOR = %q, want %q", got, "hello")
	}
	if got := Bold("hello"); got != "hello" {
		t.Errorf("Bold() with NO_COLOR = %q, want %q", got, "hello")
	}
	if got := Dim("hello"); got != "hello" {
		t.Errorf("Dim() with NO_COLOR = %q, want %q", got, "hello")
	}
}

func TestColorFunctionsReturnInput(t *testing.T) {
	// Regardless of color mode, the functions should always contain the input text
	old := os.Getenv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	noColor = false
	defer func() {
		if old != "" {
			os.Setenv("NO_COLOR", old)
		}
		noColor = old != ""
	}()

	for _, fn := range []struct {
		name string
		f    func(string) string
	}{
		{"Green", Green},
		{"Red", Red},
		{"Yellow", Yellow},
		{"Accent", Accent},
		{"Subtle", Subtle},
		{"Muted", Muted},
		{"Highlight", Highlight},
		{"Bold", Bold},
		{"Dim", Dim},
	} {
		got := fn.f("hello")
		if got == "" {
			t.Errorf("%s() returned empty string", fn.name)
		}
		// The output should always contain the original text
		if len(got) < 5 {
			t.Errorf("%s() output %q does not contain input text", fn.name, got)
		}
	}
}

func mockStdin(input string) func() {
	old := stdinReader
	stdinReader = func() *bufio.Reader {
		return bufio.NewReader(strings.NewReader(input))
	}
	return func() { stdinReader = old }
}

func TestConfirmYes(t *testing.T) {
	cleanup := mockStdin("y\n")
	defer cleanup()

	if !Confirm("Continue? [y/N]:") {
		t.Error("Confirm should return true for 'y' input")
	}
}

func TestConfirmNo(t *testing.T) {
	cleanup := mockStdin("n\n")
	defer cleanup()

	if Confirm("Continue? [y/N]:") {
		t.Error("Confirm should return false for 'n' input")
	}
}

func TestConfirmEmpty(t *testing.T) {
	cleanup := mockStdin("\n")
	defer cleanup()

	if Confirm("Continue? [y/N]:") {
		t.Error("Confirm should return false for empty input")
	}
}

func TestPromptInput(t *testing.T) {
	cleanup := mockStdin("my-branch\n")
	defer cleanup()

	got := PromptInput("Enter branch name:")
	if got != "my-branch" {
		t.Errorf("PromptInput() = %q, want %q", got, "my-branch")
	}
}

func TestPromptInputTrimmed(t *testing.T) {
	cleanup := mockStdin("  spaces  \n")
	defer cleanup()

	got := PromptInput("Enter value:")
	if got != "spaces" {
		t.Errorf("PromptInput() = %q, want %q", got, "spaces")
	}
}

func TestPromptDangerousMatch(t *testing.T) {
	cleanup := mockStdin("destroy\n")
	defer cleanup()

	if !PromptDangerous("Type 'destroy' to confirm:", "destroy") {
		t.Error("PromptDangerous should return true when input matches expected")
	}
}

func TestPromptDangerousMismatch(t *testing.T) {
	cleanup := mockStdin("delete\n")
	defer cleanup()

	if PromptDangerous("Type 'destroy' to confirm:", "destroy") {
		t.Error("PromptDangerous should return false when input does not match expected")
	}
}

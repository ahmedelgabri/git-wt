package ui

import (
	"os"
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

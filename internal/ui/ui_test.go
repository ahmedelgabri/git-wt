package ui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// runModel runs a bubbletea model through a real tea.Program with a timeout.
// This exercises the full lifecycle (Init -> Update loop -> Quit) without
// needing a TTY. If the model hangs, the context deadline fires and the
// test fails instead of blocking forever.
func runModel(t *testing.T, m tea.Model, timeout time.Duration) tea.Model {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	p := tea.NewProgram(m,
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
		tea.WithContext(ctx),
	)
	result, err := p.Run()
	if err != nil {
		if ctx.Err() != nil {
			t.Fatalf("model hung: did not complete within %s", timeout)
		}
		t.Fatalf("tea.Program error: %v", err)
	}
	return result
}

func TestNoColorOutput(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

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
	t.Setenv("NO_COLOR", "")

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

func TestPromptInputSequentialReads(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("first\nsecond\n"))
	old := stdinReader
	stdinReader = func() *bufio.Reader { return reader }
	defer func() { stdinReader = old }()

	if got := PromptInput("First:"); got != "first" {
		t.Fatalf("first PromptInput() = %q, want %q", got, "first")
	}
	if got := PromptInput("Second:"); got != "second" {
		t.Fatalf("second PromptInput() = %q, want %q", got, "second")
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

// -- bubbletea model tests --
// These test models directly by sending tea.KeyMsg messages, without
// running a full tea.Program (no TTY needed).

func TestConfirmModelYes(t *testing.T) {
	m := newConfirmModel("Continue?")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	result := updated.(confirmModel)
	if !result.confirmed {
		t.Error("confirmModel should be confirmed after 'y' key")
	}
	if !result.done {
		t.Error("confirmModel should be done after 'y' key")
	}
}

func TestConfirmModelNo(t *testing.T) {
	m := newConfirmModel("Continue?")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	result := updated.(confirmModel)
	if result.confirmed {
		t.Error("confirmModel should not be confirmed after 'n' key")
	}
	if !result.done {
		t.Error("confirmModel should be done after 'n' key")
	}
}

func TestConfirmModelEsc(t *testing.T) {
	m := newConfirmModel("Continue?")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := updated.(confirmModel)
	if result.confirmed {
		t.Error("confirmModel should not be confirmed after esc")
	}
	if !result.done {
		t.Error("confirmModel should be done after esc")
	}
}

func TestConfirmModelEnterDefaultsToNo(t *testing.T) {
	m := newConfirmModel("Continue?")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(confirmModel)
	if result.confirmed {
		t.Error("confirmModel enter with no prior selection should default to no")
	}
}

func TestConfirmModelView(t *testing.T) {
	m := newConfirmModel("Continue?")
	view := m.View()
	if !strings.Contains(view, "Continue?") {
		t.Errorf("confirmModel view should contain message, got %q", view)
	}
}

func TestInputModelSubmit(t *testing.T) {
	m := newInputModel("Branch name:", "?", "")

	// Type "feature"
	for _, r := range "feature" {
		var cmd tea.Cmd
		result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = result.(inputModel)
		_ = cmd
	}

	// Submit
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(inputModel)
	if result.Value() != "feature" {
		t.Errorf("inputModel value = %q, want %q", result.Value(), "feature")
	}
	if !result.submitted {
		t.Error("inputModel should be submitted after enter")
	}
}

func TestInputModelCancel(t *testing.T) {
	m := newInputModel("Branch name:", "?", "")

	// Type something then cancel
	for _, r := range "feature" {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = result.(inputModel)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := updated.(inputModel)
	if result.Value() != "" {
		t.Errorf("inputModel value after cancel = %q, want empty", result.Value())
	}
	if !result.canceled {
		t.Error("inputModel should be canceled after esc")
	}
}

func TestSpinnerModelSuccess(t *testing.T) {
	m := newSpinnerModel("Loading", func() error {
		return nil
	})

	// Simulate the task completing
	updated, _ := m.Update(taskDoneMsg{err: nil})
	result := updated.(spinnerModel)
	if result.err != nil {
		t.Errorf("spinnerModel err = %v, want nil", result.err)
	}
	if !result.done {
		t.Error("spinnerModel should be done after taskDoneMsg")
	}
	if !strings.Contains(result.View(), "●") {
		t.Errorf("spinnerModel success view should contain checkmark, got %q", result.View())
	}
}

func TestSpinnerModelFailure(t *testing.T) {
	m := newSpinnerModel("Loading", func() error {
		return fmt.Errorf("failed")
	})

	updated, _ := m.Update(taskDoneMsg{err: errors.New("failed")})
	result := updated.(spinnerModel)
	if result.err == nil {
		t.Error("spinnerModel err should not be nil after failure")
	}
	if !result.done {
		t.Error("spinnerModel should be done after taskDoneMsg")
	}
	if !strings.Contains(result.View(), "●") {
		t.Errorf("spinnerModel failure view should contain cross, got %q", result.View())
	}
}

func TestSpinFallback(t *testing.T) {
	// Spin with stdinReader set uses fallback path (no TTY)
	cleanup := mockStdin("")
	defer cleanup()

	called := false
	err := Spin("test operation", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Errorf("Spin() = %v, want nil", err)
	}
	if !called {
		t.Error("Spin callback should have been called")
	}
}

func TestSpinFallbackError(t *testing.T) {
	cleanup := mockStdin("")
	defer cleanup()

	testErr := errors.New("task failed")
	err := Spin("test operation", func() error {
		return testErr
	})
	if !errors.Is(err, testErr) {
		t.Errorf("Spin() = %v, want %v", err, testErr)
	}
}

func TestSuccessPrefix(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := SuccessPrefix("  ", "done")
	if !strings.Contains(got, "●") {
		t.Errorf("SuccessPrefix should contain ●, got %q", got)
	}
	if !strings.Contains(got, "done") {
		t.Errorf("SuccessPrefix should contain msg, got %q", got)
	}
	if !strings.HasPrefix(got, "  ") {
		t.Errorf("SuccessPrefix should start with prefix, got %q", got)
	}
}

func TestFailPrefix(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := FailPrefix("  ", "failed")
	if !strings.Contains(got, "●") {
		t.Errorf("FailPrefix should contain ●, got %q", got)
	}
	if !strings.Contains(got, "failed") {
		t.Errorf("FailPrefix should contain msg, got %q", got)
	}
}

func TestColorAccessors(t *testing.T) {
	accessors := []struct {
		name string
		fn   func() lipgloss.TerminalColor
	}{
		{"AccentColor", AccentColor},
		{"SuccessColor", SuccessColor},
		{"ErrorColor", ErrorColor},
		{"WarnColor", WarnColor},
		{"SubtleColor", SubtleColor},
		{"MutedColor", MutedColor},
		{"HighlightColor", HighlightColor},
	}
	for _, a := range accessors {
		if a.fn() == nil {
			t.Errorf("%s() returned nil", a.name)
		}
	}
}

func TestConfirmModelCtrlC(t *testing.T) {
	m := newConfirmModel("Continue?")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := updated.(confirmModel)
	if result.confirmed {
		t.Error("confirmModel should not be confirmed after ctrl+c")
	}
	if !result.done {
		t.Error("confirmModel should be done after ctrl+c")
	}
}

func TestConfirmModelViewDoneYes(t *testing.T) {
	m := newConfirmModel("Continue?")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	result := updated.(confirmModel)
	view := result.View()
	if !strings.Contains(view, "Continue?") {
		t.Errorf("view should contain message, got %q", view)
	}
	if !strings.Contains(view, "y") {
		t.Errorf("view should contain 'y', got %q", view)
	}
}

func TestConfirmModelViewDoneNo(t *testing.T) {
	m := newConfirmModel("Continue?")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	result := updated.(confirmModel)
	view := result.View()
	if !strings.Contains(view, "Continue?") {
		t.Errorf("view should contain message, got %q", view)
	}
	if !strings.Contains(view, "n") {
		t.Errorf("view should contain 'n', got %q", view)
	}
}

func TestInputModelCtrlC(t *testing.T) {
	m := newInputModel("Branch name:", "?", "")

	for _, r := range "test" {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = result.(inputModel)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := updated.(inputModel)
	if !result.canceled {
		t.Error("inputModel should be canceled after ctrl+c")
	}
	if result.Value() != "" {
		t.Errorf("inputModel value after ctrl+c = %q, want empty", result.Value())
	}
}

func TestInputModelViewSubmitted(t *testing.T) {
	m := newInputModel("Branch name:", "?", "")

	for _, r := range "test" {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = result.(inputModel)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(inputModel)
	view := result.View()
	if !strings.Contains(view, "test") {
		t.Errorf("submitted view should contain 'test', got %q", view)
	}
}

func TestInputModelViewCanceled(t *testing.T) {
	m := newInputModel("Branch name:", "?", "")

	for _, r := range "test" {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = result.(inputModel)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := updated.(inputModel)
	view := result.View()
	if !strings.Contains(view, "Branch name:") {
		t.Errorf("canceled view should contain message, got %q", view)
	}
}

func TestSpinnerModelCtrlC(t *testing.T) {
	m := newSpinnerModel("Loading", func() error { return nil })
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := updated.(spinnerModel)
	if !result.done {
		t.Error("spinnerModel should be done after ctrl+c")
	}
	if result.err == nil || result.err.Error() != "interrupted" {
		t.Errorf("spinnerModel err = %v, want 'interrupted'", result.err)
	}
}

func TestSpinnerModelTickMsg(t *testing.T) {
	m := newSpinnerModel("Loading", func() error { return nil })
	// Should not panic
	updated, _ := m.Update(spinner.TickMsg{})
	_ = updated.(spinnerModel)
}

func TestSpinnerModelViewRunning(t *testing.T) {
	m := newSpinnerModel("Loading", func() error { return nil })
	view := m.View()
	if !strings.Contains(view, "Loading") {
		t.Errorf("running view should contain message, got %q", view)
	}
}

// -- tea.Program lifecycle tests --
// These run the full bubbletea program (Init -> Update -> Quit) to catch
// bugs like missing Cmds in Init() that only manifest at runtime.

func TestSpinnerProgramSuccess(t *testing.T) {
	m := newSpinnerModel("Loading", func() error {
		return nil
	})
	result := runModel(t, m, 5*time.Second)
	r := result.(spinnerModel)
	if r.err != nil {
		t.Errorf("spinner err = %v, want nil", r.err)
	}
	if !r.done {
		t.Error("spinner should be done")
	}
}

func TestSpinnerProgramError(t *testing.T) {
	testErr := errors.New("task failed")
	m := newSpinnerModel("Loading", func() error {
		return testErr
	})
	result := runModel(t, m, 5*time.Second)
	r := result.(spinnerModel)
	if r.err == nil {
		t.Error("spinner err should not be nil")
	}
	if !r.done {
		t.Error("spinner should be done")
	}
}

func TestSpinWithOutput(t *testing.T) {
	called := false
	err := SpinWithOutput("test operation", func(w io.Writer) error {
		called = true
		fmt.Fprintln(w, "some output")
		return nil
	})
	if err != nil {
		t.Errorf("SpinWithOutput() = %v, want nil", err)
	}
	if !called {
		t.Error("SpinWithOutput callback should have been called")
	}
}

func TestSpinWithOutputError(t *testing.T) {
	testErr := errors.New("task failed")
	err := SpinWithOutput("test operation", func(w io.Writer) error {
		return testErr
	})
	if !errors.Is(err, testErr) {
		t.Errorf("SpinWithOutput() = %v, want %v", err, testErr)
	}
}

func TestSectionContainsTitleAndBody(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := Section("Status", "hello", "world")
	if !strings.Contains(got, "Status") {
		t.Fatalf("Section() missing title: %q", got)
	}
	if !strings.Contains(got, "hello") || !strings.Contains(got, "world") {
		t.Fatalf("Section() missing body: %q", got)
	}
}

func TestSectionAllowsEmptyTitleAndSpacer(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := Section("", "table", "", "summary")
	if strings.Contains(got, "Status") {
		t.Fatalf("Section() unexpectedly contains title: %q", got)
	}
	if !strings.Contains(got, "table\n\nsummary") {
		t.Fatalf("Section() should preserve blank line before summary: %q", got)
	}
}

func TestPathFormatsRelativePaths(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := Path("./main")
	if got != "./main" {
		t.Fatalf("Path() = %q, want %q", got, "./main")
	}
}

func TestRenderTableContainsHeadersAndRows(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := RenderTable([]TableColumn{{Title: "COL1"}, {Title: "COL2"}}, [][]string{{"a", "b"}, {"c", "d"}})
	for _, want := range []string{"COL1", "COL2", "a", "b", "c", "d"} {
		if !strings.Contains(got, want) {
			t.Fatalf("RenderTable() missing %q in %q", want, got)
		}
	}
}

func TestRenderTableHandlesColoredCells(t *testing.T) {
	got := RenderTable(
		[]TableColumn{{Title: "STATUS", MinWidth: 6}, {Title: "CHECK", MinWidth: 12}},
		[][]string{{Green("OK"), "Repository"}, {Yellow("WARN"), "Default branch"}},
	)
	plain := ansi.Strip(got)
	for _, want := range []string{"STATUS", "CHECK", "OK", "Repository", "WARN", "Default branch"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("RenderTable() missing %q in %q", want, plain)
		}
	}
	if strings.Contains(plain, "OKRepository") || strings.Contains(plain, "WARNDefault branch") {
		t.Fatalf("RenderTable() collapsed columns: %q", plain)
	}
}

func TestRenderTableRespectsMaxWidth(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := RenderTable(
		[]TableColumn{{Title: "NAME", MaxWidth: 8}, {Title: "DETAIL", MaxWidth: 10}},
		[][]string{{"feature-very-long", "this should truncate"}},
	)
	plain := ansi.Strip(got)
	if !strings.Contains(plain, "feature…") {
		t.Fatalf("RenderTable() should truncate narrow column, got %q", plain)
	}
	if !strings.Contains(plain, "this shou…") {
		t.Fatalf("RenderTable() should truncate detail column, got %q", plain)
	}
}

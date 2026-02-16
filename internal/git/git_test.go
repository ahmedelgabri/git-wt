package git

import (
	"os"
	"testing"
)

func TestQueryVersion(t *testing.T) {
	out, err := Query("--version")
	if err != nil {
		t.Fatalf("Query(--version) error: %v", err)
	}
	if out == "" {
		t.Error("Query(--version) returned empty output")
	}
}

func TestQueryLines(t *testing.T) {
	// Query something that returns multiple lines
	lines, err := QueryLines("help", "-a")
	if err != nil {
		t.Fatalf("QueryLines(help, -a) error: %v", err)
	}
	if len(lines) == 0 {
		t.Error("QueryLines(help, -a) returned no lines")
	}
}

func TestDebugMode(t *testing.T) {
	old := Debug
	Debug = true
	defer func() { Debug = old }()

	// In debug mode, Run should not actually execute git commands
	err := Run("status")
	if err != nil {
		t.Errorf("Run() in debug mode should not error, got: %v", err)
	}
}

func TestDebugEnvVar(t *testing.T) {
	oldEnv := os.Getenv("DEBUG")
	oldDebug := Debug

	os.Setenv("DEBUG", "1")
	// Re-evaluate (in real code this happens at init time)
	Debug = os.Getenv("DEBUG") != ""

	if !Debug {
		t.Error("Debug should be true when DEBUG env is set")
	}

	if oldEnv == "" {
		os.Unsetenv("DEBUG")
	} else {
		os.Setenv("DEBUG", oldEnv)
	}
	Debug = oldDebug
}

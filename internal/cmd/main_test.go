package cmd

import (
	"os"
	"testing"

	"github.com/ahmedelgabri/git-wt/internal/testutil"
)

func TestMain(m *testing.M) {
	testutil.IsolateGit()
	// Migration registers journals in the user's state directory.
	state, err := os.MkdirTemp("", "git-wt-state-")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("XDG_STATE_HOME", state)
	code := m.Run()
	_ = os.RemoveAll(state)
	os.Exit(code)
}

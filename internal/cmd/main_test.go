package cmd

import (
	"os"
	"testing"

	"github.com/ahmedelgabri/git-wt/internal/testutil"
)

func TestMain(m *testing.M) { testutil.IsolateGit(); os.Exit(m.Run()) }

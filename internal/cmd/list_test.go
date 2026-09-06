package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ahmedelgabri/git-wt/internal/worktree"
)

func TestParseListArgs(t *testing.T) {
	for _, tc := range []struct {
		name                string
		args                []string
		json, help, invalid bool
		native              []string
	}{
		{name: "native"},
		{name: "json", args: []string{"--json"}, json: true},
		{name: "explicit true", args: []string{"--json=true"}, json: true},
		{name: "disabled", args: []string{"--json=false", "--porcelain", "-z"}, native: []string{"--porcelain", "-z"}},
		{name: "repeated", args: []string{"--json", "--json=false"}},
		{name: "native options", args: []string{"--verbose", "--expire", "now"}, native: []string{"--verbose", "--expire", "now"}},
		{name: "conflicting options retained", args: []string{"--porcelain", "--json"}, json: true, native: []string{"--porcelain"}},
		{name: "terminator", args: []string{"--", "--json"}, native: []string{"--", "--json"}},
		{name: "help", args: []string{"--help"}, help: true},
		{name: "short help", args: []string{"-h"}, help: true},
		{name: "invalid boolean", args: []string{"--json=invalid"}, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			asJSON, help, native, err := parseListArgs(tc.args)
			if (err != nil) != tc.invalid || asJSON != tc.json || help != tc.help || !reflect.DeepEqual(native, tc.native) {
				t.Fatalf("got json=%v help=%v native=%q err=%v", asJSON, help, native, err)
			}
		})
	}
}

func TestListJSONRejectsNativeOptionsBeforeQueryingGit(t *testing.T) {
	for _, args := range [][]string{
		{"--json", "--porcelain"},
		{"--porcelain", "--json"},
		{"--json", "-z"},
		{"--json", "--verbose"},
		{"--json", "--expire=now"},
		{"--json", "unexpected"},
	} {
		cmd := newListCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		err := runList(cmd, args)
		if err == nil || !strings.Contains(err.Error(), "cannot be combined") || out.Len() != 0 {
			t.Fatalf("args=%q: out=%q error=%v", args, out.String(), err)
		}
	}
}

func TestWorktreesJSONSchema(t *testing.T) {
	var out bytes.Buffer
	entry := worktree.Entry{
		Path: "/repo/space \"quote\"\tand\nnewline", Head: strings.Repeat("a", 64),
		Detached: true, Locked: true, LockedReason: " reason\nwith whitespace ",
		Prunable: true, PrunableReason: "missing gitdir",
	}
	if err := writeWorktreesJSON(&out, []worktree.Entry{entry}); err != nil {
		t.Fatal(err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{{
		"path": entry.Path, "branch": "", "head": entry.Head,
		"detached": true, "locked": true, "locked_reason": entry.LockedReason,
		"prunable": true, "prunable_reason": entry.PrunableReason,
	}}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("JSON = %#v, want %#v", decoded, want)
	}
}

func TestWorktreesJSONEmpty(t *testing.T) {
	var out bytes.Buffer
	if err := writeWorktreesJSON(&out, nil); err != nil {
		t.Fatal(err)
	}
	if out.String() != "[]\n" {
		t.Fatalf("empty JSON = %q", out.String())
	}
}

func TestWorktreesJSONRejectsInvalidUTF8(t *testing.T) {
	var out bytes.Buffer
	err := writeWorktreesJSON(&out, []worktree.Entry{{Path: "/valid"}, {Path: "/invalid\xff"}})
	if err == nil || out.Len() != 0 {
		t.Fatalf("error=%v, output=%q", err, out.String())
	}
}

type failedJSONWriter struct{ err error }

func (w failedJSONWriter) Write([]byte) (int, error) { return 0, w.err }

func TestWorktreesJSONReportsWriteFailure(t *testing.T) {
	want := errors.New("write failed")
	if err := writeWorktreesJSON(failedJSONWriter{want}, nil); !errors.Is(err, want) {
		t.Fatalf("error=%v", err)
	}
}

func TestListAliasAndJSONCompletion(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"ls"})
	if err != nil || command.Name() != "list" {
		t.Fatalf("alias resolves to %v, %v", command, err)
	}
	if command.Flags().Lookup("json") == nil {
		t.Fatal("JSON flag is not registered for completion")
	}
}

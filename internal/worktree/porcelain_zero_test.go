package worktree

import "testing"

func TestParsePorcelainZeroPreservesPathBytes(t *testing.T) {
	path := "/repo/feature\nwith\ttabs and spaces "
	input := "worktree /repo/database\x00bare\x00\x00worktree " + path + "\x00HEAD 1234567890\x00branch refs/heads/feature\x00locked reason\nwith newline\x00\x00"
	entries := ParsePorcelain(input)
	if len(entries) != 1 || entries[0].Path != path || entries[0].LockedReason != "reason\nwith newline" {
		t.Fatalf("parsed records: %#v", entries)
	}
}

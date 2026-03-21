package cmd

import "testing"

func TestParseBranchStatus(t *testing.T) {
	input := `# branch.oid abc1234
# branch.head main
# branch.upstream origin/main
# branch.ab +2 -1
1 .M N... 100644 100644 100644 abc123 abc123 file.txt`

	upstream, ahead, behind, dirty := parseBranchStatus(input)
	if upstream != "origin/main" {
		t.Fatalf("upstream = %q, want %q", upstream, "origin/main")
	}
	if ahead != 2 || behind != 1 {
		t.Fatalf("ahead/behind = %d/%d, want 2/1", ahead, behind)
	}
	if !dirty {
		t.Fatal("dirty should be true")
	}
}

func TestParseBranchStatusCleanLocal(t *testing.T) {
	input := `# branch.oid abc1234
# branch.head detached`

	upstream, ahead, behind, dirty := parseBranchStatus(input)
	if upstream != "" {
		t.Fatalf("upstream = %q, want empty", upstream)
	}
	if ahead != 0 || behind != 0 {
		t.Fatalf("ahead/behind = %d/%d, want 0/0", ahead, behind)
	}
	if dirty {
		t.Fatal("dirty should be false")
	}
}

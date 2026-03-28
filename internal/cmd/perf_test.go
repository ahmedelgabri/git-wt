package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ahmedelgabri/git-wt/internal/worktree"
)

func benchmarkRemoteBranchOutput(count int) string {
	lines := make([]string, 0, count)
	for i := 0; i < count; i++ {
		lines = append(lines, fmt.Sprintf("origin/feature/%04d\t1710000000\tEngineer %d\tImprove subsystem %d", i, i%10, i))
	}
	return strings.Join(lines, "\n")
}

func benchmarkWorktreeEntries(count int) []worktree.Entry {
	entries := make([]worktree.Entry, count)
	for i := 0; i < count; i++ {
		entries[i] = worktree.Entry{
			Path:   fmt.Sprintf("/repo/feature/%04d", i),
			Branch: fmt.Sprintf("feature/%04d", i),
			Head:   "abc1234",
		}
	}
	return entries
}

func benchmarkRemovalItems(count int) []removalItem {
	items := make([]removalItem, count)
	for i := 0; i < count; i++ {
		items[i] = removalItem{
			Action: removalActionRemove,
			Target: removalTarget{
				path:   fmt.Sprintf("/repo/feature/%04d", i),
				branch: fmt.Sprintf("feature/%04d", i),
			},
			Reason: "fully merged into main",
		}
	}
	return items
}

func BenchmarkParseRemoteBranchCandidates1K(b *testing.B) {
	output := benchmarkRemoteBranchOutput(1000)
	checkedOut := map[string]bool{"feature/0001": true, "feature/0002": true}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parseRemoteBranchCandidates(output, checkedOut)
	}
}

func BenchmarkRemoteBranchCandidatesToPickerItems1K(b *testing.B) {
	candidates := parseRemoteBranchCandidates(benchmarkRemoteBranchOutput(1000), nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = remoteBranchCandidatesToPickerItems(candidates)
	}
}

func BenchmarkEntriesToPickerItems1K(b *testing.B) {
	entries := benchmarkWorktreeEntries(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = entriesToPickerItems(entries)
	}
}

func BenchmarkRemovalCandidatesToPickerItems1K(b *testing.B) {
	items := benchmarkRemovalItems(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = removalCandidatesToPickerItems(items)
	}
}

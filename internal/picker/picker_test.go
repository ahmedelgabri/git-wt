package picker

import (
	"fmt"
	"testing"
)

func TestItemFilterValue(t *testing.T) {
	item := Item{Label: "feature-branch", Value: "/path/to/wt"}
	if got := item.FilterValue(); got != "feature-branch" {
		t.Errorf("FilterValue() = %q, want %q", got, "feature-branch")
	}
}

func TestItemSelected(t *testing.T) {
	item := Item{Label: "test"}
	if item.Selected() {
		t.Error("new item should not be selected")
	}
	item.selected = true
	if !item.Selected() {
		t.Error("item should be selected after setting")
	}
}

func TestFormatSelected(t *testing.T) {
	single := []Item{{Label: "main"}}
	if got := FormatSelected(single); got != "main" {
		t.Errorf("FormatSelected(1) = %q, want %q", got, "main")
	}

	multi := []Item{{Label: "a"}, {Label: "b"}, {Label: "c"}}
	if got := FormatSelected(multi); got != "3 items" {
		t.Errorf("FormatSelected(3) = %q, want %q", got, "3 items")
	}
}

func TestModelCollectResultSingle(t *testing.T) {
	items := []Item{
		{Label: "main", Value: "/project/main"},
		{Label: "feature", Value: "/project/feature"},
	}
	m := newModel(Config{Items: items})
	m.cursor = 1

	result := m.collectResult()
	if result.Canceled {
		t.Error("result should not be canceled")
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Label != "feature" {
		t.Errorf("selected item = %q, want %q", result.Items[0].Label, "feature")
	}
}

func TestModelCollectResultMulti(t *testing.T) {
	items := []Item{
		{Label: "main", Value: "/project/main"},
		{Label: "feature", Value: "/project/feature"},
		{Label: "bugfix", Value: "/project/bugfix"},
	}
	items[0].selected = true
	items[2].selected = true

	m := newModel(Config{Items: items, Multi: true})
	m.items = items

	result := m.collectResult()
	if result.Canceled {
		t.Error("result should not be canceled")
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if result.Items[0].Label != "main" {
		t.Errorf("first selected = %q, want %q", result.Items[0].Label, "main")
	}
	if result.Items[1].Label != "bugfix" {
		t.Errorf("second selected = %q, want %q", result.Items[1].Label, "bugfix")
	}
}

func TestModelCollectResultMultiNoneSelected(t *testing.T) {
	items := []Item{
		{Label: "main", Value: "/project/main"},
		{Label: "feature", Value: "/project/feature"},
	}
	m := newModel(Config{Items: items, Multi: true})
	m.cursor = 1

	// When nothing is explicitly selected in multi mode, focused item is used
	result := m.collectResult()
	if result.Canceled {
		t.Error("result should not be canceled")
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item (focused), got %d", len(result.Items))
	}
	if result.Items[0].Label != "feature" {
		t.Errorf("focused item = %q, want %q", result.Items[0].Label, "feature")
	}
}

func TestModelFilterEmpty(t *testing.T) {
	items := []Item{
		{Label: "main"},
		{Label: "feature"},
		{Label: "bugfix"},
	}
	m := newModel(Config{Items: items})
	m.applyFilter()

	if len(m.filtered) != 3 {
		t.Errorf("empty filter should show all %d items, got %d", 3, len(m.filtered))
	}
}

func TestApplyFilterPopulatesMatches(t *testing.T) {
	items := []Item{
		{Label: "main"},
		{Label: "feature"},
		{Label: "bugfix"},
	}
	m := newModel(Config{Items: items})

	// Simulate typing "feat" into the filter
	m.filter.SetValue("feat")
	m.applyFilter()

	if len(m.filtered) == 0 {
		t.Fatal("expected at least one filtered result for 'feat'")
	}
	if len(m.matches) != len(m.filtered) {
		t.Fatalf("matches length (%d) should equal filtered length (%d)", len(m.matches), len(m.filtered))
	}

	// "feature" should be in the results with match positions
	found := false
	for i, idx := range m.filtered {
		if m.items[idx].Label == "feature" {
			found = true
			if len(m.matches[i].MatchedIndexes) == 0 {
				t.Error("matched item should have non-empty MatchedIndexes")
			}
		}
	}
	if !found {
		t.Error("expected 'feature' in filtered results")
	}
}

func TestApplyFilterClearsMatchesOnEmpty(t *testing.T) {
	items := []Item{
		{Label: "main"},
		{Label: "feature"},
	}
	m := newModel(Config{Items: items})

	// Filter then clear
	m.filter.SetValue("main")
	m.applyFilter()
	m.filter.SetValue("")
	m.applyFilter()

	if m.matches != nil {
		t.Errorf("matches should be nil after clearing filter, got %d entries", len(m.matches))
	}
	if len(m.filtered) != 2 {
		t.Errorf("all items should be shown after clearing filter, got %d", len(m.filtered))
	}
}

func TestRenderLabelNoMatch(t *testing.T) {
	m := newModel(Config{Items: []Item{{Label: "hello"}}})
	// No filter applied, so matches is nil
	got := m.renderLabel(0, "hello")
	if got != "hello" {
		t.Errorf("renderLabel with no matches should return plain label, got %q", got)
	}
}

func BenchmarkApplyFilter50k(b *testing.B) {
	items := make([]Item, 50_000)
	for i := range items {
		items[i] = Item{
			Label: fmt.Sprintf("feature/PROJ-%d-some-branch-name", i),
			Value: fmt.Sprintf("/workspace/feature-PROJ-%d", i),
		}
	}
	m := newModel(Config{Items: items})
	m.filter.SetValue("proj-123")

	b.ResetTimer()
	for range b.N {
		m.applyFilter()
	}
}

func BenchmarkApplyFilterEmpty50k(b *testing.B) {
	items := make([]Item, 50_000)
	for i := range items {
		items[i] = Item{
			Label: fmt.Sprintf("feature/PROJ-%d-some-branch-name", i),
			Value: fmt.Sprintf("/workspace/feature-PROJ-%d", i),
		}
	}
	m := newModel(Config{Items: items})
	m.filter.SetValue("")

	b.ResetTimer()
	for range b.N {
		m.applyFilter()
	}
}

func TestEmptyItems(t *testing.T) {
	result, err := Run(Config{Items: nil})
	if err != nil {
		t.Fatalf("Run with empty items should not error: %v", err)
	}
	if !result.Canceled {
		t.Error("Run with empty items should return canceled")
	}
}

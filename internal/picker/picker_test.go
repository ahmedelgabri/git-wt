package picker

import (
	"testing"
)

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

func TestEmptyItems(t *testing.T) {
	result, err := Run(Config{Items: nil})
	if err != nil {
		t.Fatalf("Run with empty items should not error: %v", err)
	}
	if !result.Canceled {
		t.Error("Run with empty items should return canceled")
	}
}

func TestFormatFzfLine(t *testing.T) {
	tests := []struct {
		name string
		item Item
		want string
	}{
		{
			name: "label only",
			item: Item{Label: "feature", Value: "/project/feature"},
			want: "/project/feature\tfeature",
		},
		{
			name: "label with desc",
			item: Item{Label: "feature [feat]", Value: "/project/feature", Desc: "~/project/feature"},
			want: "/project/feature\tfeature [feat] · ~/project/feature",
		},
		{
			name: "empty desc",
			item: Item{Label: "main", Value: "/project/main", Desc: ""},
			want: "/project/main\tmain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFzfLine(tt.item)
			if got != tt.want {
				t.Errorf("formatFzfLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- GIT_WT_SELECT env var bypass tests ---

func TestResolveEnvSelectionByValue(t *testing.T) {
	cfg := Config{
		Items: []Item{
			{Label: "main [main]", Value: "/project/main"},
			{Label: "feature [feature]", Value: "/project/feature"},
		},
	}
	result, err := resolveEnvSelection(cfg, "/project/feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Canceled {
		t.Fatal("should not be canceled")
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Value != "/project/feature" {
		t.Errorf("value = %q, want %q", result.Items[0].Value, "/project/feature")
	}
}

func TestResolveEnvSelectionByLabel(t *testing.T) {
	cfg := Config{
		Items: []Item{
			{Label: "main", Value: "/project/main"},
			{Label: "feature", Value: "/project/feature"},
		},
	}
	result, err := resolveEnvSelection(cfg, "feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Canceled {
		t.Fatal("should not be canceled")
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Label != "feature" {
		t.Errorf("label = %q, want %q", result.Items[0].Label, "feature")
	}
}

func TestResolveEnvSelectionMulti(t *testing.T) {
	cfg := Config{
		Items: []Item{
			{Label: "main", Value: "/project/main"},
			{Label: "feature", Value: "/project/feature"},
			{Label: "bugfix", Value: "/project/bugfix"},
		},
	}
	result, err := resolveEnvSelection(cfg, "main,bugfix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Canceled {
		t.Fatal("should not be canceled")
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if result.Items[0].Label != "main" {
		t.Errorf("first = %q, want %q", result.Items[0].Label, "main")
	}
	if result.Items[1].Label != "bugfix" {
		t.Errorf("second = %q, want %q", result.Items[1].Label, "bugfix")
	}
}

func TestResolveEnvSelectionNoMatch(t *testing.T) {
	cfg := Config{
		Items: []Item{
			{Label: "main", Value: "/project/main"},
		},
	}
	result, err := resolveEnvSelection(cfg, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Canceled {
		t.Fatal("should be canceled when no match")
	}
}

func TestRunWithEnvBypass(t *testing.T) {
	t.Setenv("GIT_WT_SELECT", "feature")
	result, err := Run(Config{
		Items: []Item{
			{Label: "main", Value: "/project/main"},
			{Label: "feature", Value: "/project/feature"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Canceled {
		t.Fatal("should not be canceled")
	}
	if len(result.Items) != 1 || result.Items[0].Label != "feature" {
		t.Errorf("expected feature, got %v", result.Items)
	}
}

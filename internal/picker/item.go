package picker

import "fmt"

// Item represents a selectable item in the picker.
type Item struct {
	Label    string // Display text
	Value    string // Underlying value (e.g., worktree path)
	Desc     string // Optional description shown dimmed
	selected bool
}

func (i Item) FilterValue() string { return i.Label }
func (i Item) Title() string       { return i.Label }
func (i Item) Description() string { return i.Desc }

// Selected returns whether this item is selected (for multi-select mode).
func (i Item) Selected() bool { return i.selected }

// Result holds the outcome of a picker interaction.
type Result struct {
	Items    []Item
	Canceled bool
}

// Config configures the picker behavior.
type Config struct {
	Items       []Item
	Multi       bool   // Enable multi-select (TAB to toggle)
	Prompt      string
	Header      string
	PreviewFunc func(Item) string // Generate preview content for focused item
}

// FormatSelected returns a human-readable string of selected items for display.
func FormatSelected(items []Item) string {
	if len(items) == 1 {
		return items[0].Label
	}
	return fmt.Sprintf("%d items", len(items))
}

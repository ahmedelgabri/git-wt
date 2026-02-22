package picker

import "fmt"

// Item represents a selectable item in the picker.
type Item struct {
	Label string // Display text
	Value string // Underlying value (e.g., worktree path)
	Desc  string // Optional description shown dimmed
}

// Result holds the outcome of a picker interaction.
type Result struct {
	Items    []Item
	Canceled bool
}

// Config configures the picker behavior.
type Config struct {
	Items      []Item
	Multi      bool // Enable multi-select (TAB to toggle)
	Prompt     string
	Header     string
	PreviewCmd string // Shell command for fzf --preview ({1} = item Value)
}

// FormatSelected returns a human-readable string of selected items for display.
func FormatSelected(items []Item) string {
	if len(items) == 1 {
		return items[0].Label
	}
	return fmt.Sprintf("%d items", len(items))
}

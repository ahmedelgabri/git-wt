package picker

import (
	"os"
	"strings"
	"sync"

	fzf "github.com/junegunn/fzf/src"
)

// formatFzfLine formats an Item as a tab-delimited line for fzf input.
// Format: value\tlabel[ · desc]
// The value is hidden via --with-nth=2.. but accessible as {1} in --preview.
func formatFzfLine(item Item) string {
	display := item.Label
	if item.Desc != "" {
		display += " · " + item.Desc
	}
	return item.Value + "\t" + display
}

// Run displays the picker and returns the result.
// If GIT_WT_SELECT is set, bypasses the TUI and selects matching items directly.
func Run(cfg Config) (Result, error) {
	if len(cfg.Items) == 0 {
		return Result{Canceled: true}, nil
	}

	if sel := os.Getenv("GIT_WT_SELECT"); sel != "" {
		return resolveEnvSelection(cfg, sel)
	}

	// Build fzf args
	args := []string{
		"--ansi",
		"--delimiter", "\t",
		"--with-nth", "2..",
		"--height", "50%",
	}

	if cfg.Multi {
		args = append(args, "--multi")
	}
	if cfg.Prompt != "" {
		args = append(args, "--prompt", cfg.Prompt)
	}
	if cfg.Header != "" {
		args = append(args, "--header", cfg.Header)
	}
	if cfg.PreviewCmd != "" {
		args = append(args, "--preview", cfg.PreviewCmd)
		args = append(args, "--preview-window", "right:50%:wrap")
	}

	opts, err := fzf.ParseOptions(true, args)
	if err != nil {
		return Result{}, err
	}

	// Feed items into fzf
	inputChan := make(chan string)
	go func() {
		for _, item := range cfg.Items {
			inputChan <- formatFzfLine(item)
		}
		close(inputChan)
	}()

	// Collect selected items from fzf
	outputChan := make(chan string, len(cfg.Items))
	var selected []string
	var wg sync.WaitGroup
	wg.Go(func() {
		for s := range outputChan {
			selected = append(selected, s)
		}
	})

	opts.Input = inputChan
	opts.Output = outputChan

	code, err := fzf.Run(opts)
	wg.Wait()
	if err != nil && code != fzf.ExitInterrupt {
		return Result{}, err
	}
	if code == fzf.ExitInterrupt || code == fzf.ExitNoMatch {
		return Result{Canceled: true}, nil
	}

	// Parse selected output back to Items. Each output line is the full
	// fzf line (value\tlabel...), so extract the value (first field).
	var resultItems []Item
	for _, line := range selected {
		value := line
		if idx := strings.IndexByte(line, '\t'); idx >= 0 {
			value = line[:idx]
		}
		if item, ok := findItem(cfg.Items, value); ok {
			resultItems = append(resultItems, item)
		}
	}

	if len(resultItems) == 0 {
		return Result{Canceled: true}, nil
	}
	return Result{Items: resultItems}, nil
}

// resolveEnvSelection matches comma-separated values from GIT_WT_SELECT against
// item Value first, then Label. Returns Canceled if nothing matches.
func resolveEnvSelection(cfg Config, sel string) (Result, error) {
	parts := strings.Split(sel, ",")
	var matched []Item
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if item, ok := findItem(cfg.Items, part); ok {
			matched = append(matched, item)
		}
	}
	if len(matched) == 0 {
		return Result{Canceled: true}, nil
	}
	return Result{Items: matched}, nil
}

func findItem(items []Item, needle string) (Item, bool) {
	// Match by Value first
	for _, item := range items {
		if item.Value == needle {
			return item, true
		}
	}
	// Fall back to Label
	for _, item := range items {
		if item.Label == needle {
			return item, true
		}
	}
	return Item{}, false
}

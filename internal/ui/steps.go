package ui

import (
	"context"
	"io"
)

// Step describes one unit of work in a sequential command flow.
type Step struct {
	Message    string
	ShowOutput bool
	Run        func(ctx context.Context, w io.Writer) error
}

// RunSteps executes steps sequentially using the shared task runner for each
// step, stopping on the first error.
func RunSteps(steps []Step) error {
	for _, step := range steps {
		if step.ShowOutput {
			if err := SpinWithOutputContext(step.Message, step.Run); err != nil {
				return err
			}
			continue
		}
		if err := SpinContext(step.Message, func(ctx context.Context) error {
			return step.Run(ctx, io.Discard)
		}); err != nil {
			return err
		}
	}
	return nil
}

package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
)

func TestRunStepsExecutesInOrder(t *testing.T) {
	cleanup := mockStdin("")
	defer cleanup()

	var ran []string
	err := RunSteps([]Step{
		{Message: "step 1", Run: func(context.Context, io.Writer) error {
			ran = append(ran, "step 1")
			return nil
		}},
		{Message: "step 2", ShowOutput: true, Run: func(_ context.Context, w io.Writer) error {
			ran = append(ran, "step 2")
			fmt.Fprintln(w, "hello")
			return nil
		}},
	})
	if err != nil {
		t.Fatalf("RunSteps() = %v, want nil", err)
	}
	if len(ran) != 2 || ran[0] != "step 1" || ran[1] != "step 2" {
		t.Fatalf("RunSteps() order = %v, want [step 1 step 2]", ran)
	}
}

func TestRunStepsStopsOnError(t *testing.T) {
	cleanup := mockStdin("")
	defer cleanup()

	testErr := errors.New("boom")
	var ran []string
	err := RunSteps([]Step{
		{Message: "step 1", Run: func(context.Context, io.Writer) error {
			ran = append(ran, "step 1")
			return testErr
		}},
		{Message: "step 2", Run: func(context.Context, io.Writer) error {
			ran = append(ran, "step 2")
			return nil
		}},
	})
	if !errors.Is(err, testErr) {
		t.Fatalf("RunSteps() err = %v, want %v", err, testErr)
	}
	if len(ran) != 1 || ran[0] != "step 1" {
		t.Fatalf("RunSteps() should stop after first error, ran = %v", ran)
	}
}

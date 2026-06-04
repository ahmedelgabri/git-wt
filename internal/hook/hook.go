package hook

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
)

func LoadConfig(key string) ([]string, error) {
	out, err := git.Query("config", "--get-all", key)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func Run(ctx context.Context, hooks []string, dir string, w io.Writer) error {
	for _, h := range hooks {
		if git.Debug() {
			fmt.Fprintf(w, "[in %s] sh -c %s\n", dir, h)
			continue
		}
		cmd := exec.CommandContext(ctx, "sh", "-c", h)
		cmd.Dir = dir
		cmd.Stdout = w
		cmd.Stderr = w
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("hook %q failed: %w", h, err)
		}
	}
	return nil
}

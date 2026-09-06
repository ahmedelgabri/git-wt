package hook

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
)

type Event string

const (
	BeforeAdd    Event = "beforeadd"
	AfterAdd     Event = "afteradd"
	BeforeRemove Event = "beforeremove"
	AfterRemove  Event = "afterremove"
)

type Invocation struct {
	Event        Event
	Dir          string
	WorktreePath string
	Branch       string
	BareRoot     string
}

func Load(event Event) ([]string, error) {
	return LoadConfig("wt." + string(event))
}

func LoadConfig(key string) ([]string, error) {
	out, err := git.QueryRaw("config", "--null", "--get-all", key)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	out = strings.TrimSuffix(out, "\x00")
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\x00"), nil
}

func Run(ctx context.Context, hooks []string, invocation Invocation, w io.Writer) error {
	for _, h := range hooks {
		if git.Debug() {
			fmt.Fprintf(w, "[%s in %s] sh -c %s\n", configKey(invocation.Event), invocation.Dir, h)
			continue
		}
		cmd := exec.CommandContext(ctx, "sh", "-c", h)
		cmd.Dir = invocation.Dir
		cmd.Env = append(
			git.RepositoryEnv(),
			"GIT_WT_EVENT="+string(invocation.Event),
			"GIT_WT_PATH="+invocation.WorktreePath,
			"GIT_WT_BRANCH="+invocation.Branch,
			"GIT_WT_BARE_ROOT="+invocation.BareRoot,
		)
		cmd.Stdout = w
		cmd.Stderr = w
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s hook %q failed: %w", configKey(invocation.Event), h, err)
		}
	}
	return nil
}

func configKey(event Event) string {
	return "wt." + string(event)
}

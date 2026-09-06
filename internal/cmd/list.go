package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
	"github.com/spf13/cobra"
)

func init() { rootCmd.AddCommand(newListCommand()) }

func newListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list [--json | git-list-options]",
		Aliases: []string{"ls"},
		Short:   "List worktrees using native Git output or JSON",
		Long: `List worktrees. Without --json, arguments and output pass through to
'git worktree list' unchanged. The alias 'ls' supports the same options.

--json prints an array of non-bare worktrees with absolute paths, branch names,
full HEAD object IDs, and detached, locked, and prunable state. All fields are
always present; absent strings are empty and an empty list is []. JSON mode
cannot be combined with native Git list options.`,
		Example: "  git wt list\n  git wt ls --json\n  git wt list --porcelain -z",
		// Keep native options intact, including options unknown to this binary.
		// Register --json for help/completion, but dispatch from the raw arguments.
		DisableFlagParsing: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE:               runList,
	}
	cmd.Flags().Bool("json", false, "Print worktrees as a JSON array")
	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	asJSON, help, nativeArgs, err := parseListArgs(args)
	if err != nil {
		return err
	}
	if help {
		return cmd.Help()
	}
	if !asJSON {
		return git.QueryRun(append([]string{"worktree", "list"}, nativeArgs...)...)
	}
	if len(nativeArgs) > 0 {
		return fmt.Errorf("--json cannot be combined with native list options or arguments: %s", strings.Join(nativeArgs, " "))
	}
	entries, err := worktree.ListContext(cmd.Context())
	if err != nil {
		return err
	}
	return writeWorktreesJSON(cmd.OutOrStdout(), entries)
}

func parseListArgs(args []string) (asJSON, help bool, native []string, err error) {
	for i, arg := range args {
		switch {
		case arg == "--":
			return asJSON, help, append(native, args[i:]...), nil
		case arg == "--help" || arg == "-h":
			help = true
		case arg == "--json":
			asJSON = true
		case strings.HasPrefix(arg, "--json="):
			asJSON, err = strconv.ParseBool(strings.TrimPrefix(arg, "--json="))
			if err != nil {
				return false, help, nil, fmt.Errorf("invalid --json value: %w", err)
			}
		default:
			native = append(native, arg)
		}
	}
	return asJSON, help, native, nil
}

// Keep the public schema independent of internal Entry representation changes.
type worktreeJSON struct {
	Path           string `json:"path"`
	Branch         string `json:"branch"`
	Head           string `json:"head"`
	Detached       bool   `json:"detached"`
	Locked         bool   `json:"locked"`
	LockedReason   string `json:"locked_reason"`
	Prunable       bool   `json:"prunable"`
	PrunableReason string `json:"prunable_reason"`
}

func writeWorktreesJSON(w io.Writer, entries []worktree.Entry) error {
	rows := make([]worktreeJSON, 0, len(entries))
	for _, entry := range entries {
		// encoding/json otherwise silently replaces invalid UTF-8, which could
		// make a script act on a different path than Git reported.
		for _, value := range []string{entry.Path, entry.Branch, entry.Head, entry.LockedReason, entry.PrunableReason} {
			if !utf8.ValidString(value) {
				return fmt.Errorf("worktree metadata contains invalid UTF-8; use list --porcelain -z instead")
			}
		}
		rows = append(rows, worktreeJSON{
			Path: entry.Path, Branch: entry.Branch, Head: entry.Head,
			Detached: entry.Detached, Locked: entry.Locked, LockedReason: entry.LockedReason,
			Prunable: entry.Prunable, PrunableReason: entry.PrunableReason,
		})
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(rows)
}

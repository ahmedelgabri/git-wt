package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all worktrees",
	Long: `List all worktrees. This is a pass-through to 'git worktree list', so all
git worktree list flags are supported (e.g. --porcelain).

Use --json for structured output.`,
	FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	SilenceUsage:       true,
	SilenceErrors:      true,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOutput, _ := cmd.Flags().GetBool("json")
		if jsonOutput {
			if len(args) > 0 {
				return fmt.Errorf("--json cannot be combined with git worktree list flags")
			}
			entries, err := worktree.List()
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(entries)
		}

		fullArgs := append([]string{"worktree", "list"}, args...)
		return git.QueryRun(fullArgs...)
	},
}

func init() {
	listCmd.Flags().Bool("json", false, "Output worktrees as JSON")
	rootCmd.AddCommand(listCmd)
}

package cmd

import (
	"fmt"

	"github.com/ahmedelgabri/git-wt/internal/picker"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
	"github.com/spf13/cobra"
)

var switchCmd = &cobra.Command{
	Use:   "switch",
	Short: "Interactively switch to a different worktree",
	Long: `Interactively select a worktree with a fuzzy picker and print its path.
Use with cd to change directories: cd $(git wt switch)`,
	Example:       `  cd $(git wt switch)`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, err := worktree.List()
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			fmt.Println("No worktrees available")
			return nil
		}

		items := entriesToPickerItems(entries)

		result, err := picker.Run(picker.Config{
			Items:  items,
			Prompt: "Switch to worktree: ",
			PreviewFunc: func(item picker.Item) string {
				return generateWorktreePreview(item, "switch")
			},
		})
		if err != nil {
			return err
		}

		if !result.Canceled && len(result.Items) > 0 {
			// Output the path for cd integration
			fmt.Println(result.Items[0].Value)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(switchCmd)
}

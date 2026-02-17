package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var manCmd = &cobra.Command{
	Use:    "man [directory]",
	Short:  "Generate man pages",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		header := &doc.GenManHeader{
			Section: "1",
			Source:  "git-wt",
		}
		return doc.GenManTree(rootCmd, header, args[0])
	},
}

func init() {
	rootCmd.AddCommand(manCmd)
}

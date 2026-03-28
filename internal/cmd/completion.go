package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:    "completion [bash|zsh|fish]",
	Short:  "Generate shell completion script",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return genBashCompletion(rootCmd, os.Stdout)
		case "zsh":
			return genZshCompletion(rootCmd, os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		default:
			return cmd.Help()
		}
	},
}

// bashGitSubcommandShim is appended to the bash completion so that "git wt"
// completions work in addition to standalone "git-wt".
//
// Bash's git completion calls _git_wt (underscore form) when completing
// "git wt ...". This bridge rewrites COMP_WORDS from (git wt ...) to
// (git-wt ...) and delegates to cobra's generated entry point.
const bashGitSubcommandShim = `
# Bridge for "git wt" subcommand completion in bash.
# Bash's git completion calls _git_wt() when completing "git wt ...".
_git_wt() {
    COMP_WORDS=(git-wt "${COMP_WORDS[@]:2}")
    (( COMP_CWORD -= 1 ))
    __start_git-wt
}
`

// zshGitSubcommandShim is injected into the zsh completion function so that
// completions work when invoked as "git wt" (a git subcommand) in addition
// to the standalone "git-wt" command.
//
// zsh's _git uses '(-)*:: :->option-or-argument' which shifts words so that
// words[1] is the subcommand name ("wt"), not "git". Cobra's __complete
// mechanism needs words[1] to be the binary name ("git-wt"), so we rewrite it.
const zshGitSubcommandShim = `    # Normalize "wt" to "git-wt" so completions work as a git subcommand.
    # When zsh's _git dispatches here, words[1] is already "wt" (not "git").
    if [[ "${words[1]}" = "wt" ]]; then
        words[1]="git-wt"
    fi

`

// genBashCompletion generates bash completions with a _git_wt bridge so that
// completions work for both "git-wt" (standalone) and "git wt" (subcommand).
func genBashCompletion(cmd *cobra.Command, w io.Writer) error {
	var buf bytes.Buffer
	if err := cmd.GenBashCompletion(&buf); err != nil {
		return err
	}

	output := buf.String() + bashGitSubcommandShim

	_, err := io.WriteString(w, output)
	return err
}

// genZshCompletion generates zsh completions with an injected shim so that
// completions work for both "git-wt" (standalone) and "git wt" (subcommand).
func genZshCompletion(cmd *cobra.Command, w io.Writer) error {
	var buf bytes.Buffer
	if err := cmd.GenZshCompletion(&buf); err != nil {
		return err
	}

	output := buf.String()

	const marker = "_git-wt()\n{\n"
	output = strings.Replace(output, marker, marker+zshGitSubcommandShim, 1)

	_, err := io.WriteString(w, output)
	return err
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

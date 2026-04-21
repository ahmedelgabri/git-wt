package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:    "completion [bash|zsh|zsh-git|fish]",
	Short:  "Generate shell completion script",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return genBashCompletion(rootCmd, os.Stdout)
		case "zsh":
			return genZshCompletion(rootCmd, os.Stdout)
		case "zsh-git":
			return genZshGitSubcommandCompletion(os.Stdout)
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

// zshGitSubcommandShim is injected into the generated _git-wt() function so
// Cobra receives the binary name it expects when completion is invoked via the
// git-subcommand bridge.
//
// zsh's _git shifts words so that words[1] is the subcommand name ("wt"), not
// "git". Cobra's __complete mechanism needs words[1] to be the binary name
// ("git-wt"), so we rewrite it before the generated logic runs.
const zshGitSubcommandShim = `    # Normalize "wt" to "git-wt" so Cobra sees the expected command name.
    if [[ "${words[1]}" = "wt" ]]; then
        words[1]="git-wt"
    fi

`

// zshGitSubcommandBridge is installed as a separate _git_wt completion file so
// git's own zsh completion can autoload it for `git wt ...`.
//
// Git dispatches `git wt` completion to _git_wt(), not Cobra's generated
// _git-wt(). The bridge loads the real completion function, normalizes words[1]
// to the standalone binary name Cobra expects, and delegates to it.
const zshGitSubcommandBridge = `#autoload

# Bridge for "git wt" subcommand completion in zsh.
_git_wt() {
    if [[ "${words[1]}" = "wt" ]]; then
        words[1]="git-wt"
    fi

    autoload -Uz +X _git-wt || return 1
    _git-wt "$@"
}

if [ "$funcstack[1]" = "_git_wt" ]; then
    _git_wt "$@"
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

// genZshCompletion generates the standalone zsh completion for git-wt and
// injects a small shim so Cobra also works when the function is reached via the
// git-subcommand bridge.
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

// genZshGitSubcommandCompletion generates the zsh bridge installed as _git_wt
// so git's completion system can dispatch `git wt ...` to Cobra's _git-wt.
func genZshGitSubcommandCompletion(w io.Writer) error {
	_, err := io.WriteString(w, zshGitSubcommandBridge)
	return err
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

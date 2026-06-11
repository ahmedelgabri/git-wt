package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// posixIntegration is shared by bash and zsh. It defines a git-wt function
// that shadows the binary so switch/add can change the current shell's
// directory, plus an optional git() wrapper so `git wt` reaches the function.
const posixIntegration = `# git-wt shell integration
# Wraps git-wt so 'switch' and 'add' change directory in the current shell.
git-wt() {
	case "$1" in
	switch | add)
		local dir
		dir="$(command git-wt "$@")" || return $?
		if [ -n "$dir" ] && [ -d "$dir" ]; then
			cd "$dir" || return 1
		elif [ -n "$dir" ]; then
			printf '%s\n' "$dir"
		fi
		;;
	*)
		command git-wt "$@"
		;;
	esac
}
`

const posixGitWrapper = `
# Route 'git wt ...' through the git-wt function above so it can cd too.
# All other git commands pass through unchanged.
git() {
	if [ "$1" = "wt" ]; then
		shift
		git-wt "$@"
	else
		command git "$@"
	fi
}
`

const fishIntegration = `# git-wt shell integration
# Wraps git-wt so 'switch' and 'add' change directory in the current shell.
function git-wt --wraps git-wt
    if contains -- "$argv[1]" switch add
        set -l dir (command git-wt $argv)
        set -l code $status
        test $code -ne 0; and return $code
        if test -n "$dir" -a -d "$dir"
            cd $dir
        else if test -n "$dir"
            printf '%s\n' $dir
        end
    else
        command git-wt $argv
    end
end
`

const fishGitWrapper = `
# Route 'git wt ...' through the git-wt function above so it can cd too.
# All other git commands pass through unchanged.
function git --wraps git
    if test "$argv[1]" = wt
        git-wt $argv[2..]
    else
        command git $argv
    end
end
`

var initNoGitWrapper bool

var initCmd = &cobra.Command{
	Use:   "init [bash|zsh|fish]",
	Short: "Print shell integration for automatic directory switching",
	Long: `Print a shell script that makes 'git wt switch' and 'git wt add' change
the current shell's directory instead of printing a path.

A subprocess can never change its parent shell's working directory, so the
script defines a git-wt shell function (and a thin git wrapper) that runs
the real binary and cd's to the path it prints.

Add to your shell config:

  bash (~/.bashrc):
    eval "$(git-wt init bash)"

  zsh (~/.zshrc):
    eval "$(git-wt init zsh)"

  fish (~/.config/fish/config.fish):
    git-wt init fish | source

Use --no-git-wrapper to skip the git() wrapper if another tool already
wraps the git command; 'git-wt switch' will still cd, while 'git wt switch'
keeps printing the path.`,
	Example:       `  eval "$(git-wt init zsh)"`,
	Args:          cobra.ExactArgs(1),
	ValidArgs:     []string{"bash", "zsh", "fish"},
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit(os.Stdout, args[0], !initNoGitWrapper)
	},
}

func runInit(w io.Writer, shell string, withGitWrapper bool) error {
	var integration, gitWrapper string

	switch shell {
	case "bash", "zsh":
		integration = posixIntegration
		gitWrapper = posixGitWrapper
	case "fish":
		integration = fishIntegration
		gitWrapper = fishGitWrapper
	default:
		return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish)", shell)
	}

	if _, err := io.WriteString(w, integration); err != nil {
		return err
	}
	if withGitWrapper {
		if _, err := io.WriteString(w, gitWrapper); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	initCmd.Flags().BoolVar(&initNoGitWrapper, "no-git-wrapper", false, "omit the git() wrapper function (only wrap the git-wt command)")
	rootCmd.AddCommand(initCmd)
}

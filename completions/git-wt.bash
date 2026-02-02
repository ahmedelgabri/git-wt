# shellcheck disable=SC2207  # mapfile alternative is less readable for completions
_git_wt() {
	local cur prev words cword
	_init_completion || return

	local commands="clone migrate add remove rm destroy update u switch list lock unlock move prune repair help"

	if [[ $cword -eq 1 ]]; then
		COMPREPLY=($(compgen -W "$commands" -- "$cur"))
		return
	fi

	local cmd="${words[1]}"

	case "$cmd" in
	clone)
		# clone takes a URL and optional folder name
		# No completion for URLs, but complete local paths for folder name
		if [[ $cword -eq 3 ]]; then
			_filedir -d
		fi
		;;
	add)
		case "$prev" in
		-b | -B)
			# Complete branch names for -b/-B flags
			local branches
			branches=$(git branch -a --format='%(refname:short)' 2>/dev/null | sed 's|^origin/||')
			COMPREPLY=($(compgen -W "$branches" -- "$cur"))
			;;
		*)
			if [[ $cur == -* ]]; then
				COMPREPLY=($(compgen -W "-b -B --help -h" -- "$cur"))
			else
				# Complete with remote branches or directories
				local branches
				branches=$(git branch -r --format='%(refname:short)' 2>/dev/null | grep -v HEAD | sed 's|^origin/||')
				COMPREPLY=($(compgen -W "$branches" -- "$cur"))
				_filedir -d
			fi
			;;
		esac
		;;
	remove | rm)
		if [[ $cur == -* ]]; then
			COMPREPLY=($(compgen -W "--dry-run -n --help -h" -- "$cur"))
		else
			# Complete with worktree paths
			local worktrees
			worktrees=$(git worktree list --porcelain 2>/dev/null | grep '^worktree ' | sed 's/^worktree //' | grep -v '\.bare$')
			COMPREPLY=($(compgen -W "$worktrees" -- "$cur"))
		fi
		;;
	destroy)
		if [[ $cur == -* ]]; then
			COMPREPLY=($(compgen -W "--dry-run -n --help -h" -- "$cur"))
		else
			# Complete with worktree paths
			local worktrees
			worktrees=$(git worktree list --porcelain 2>/dev/null | grep '^worktree ' | sed 's/^worktree //' | grep -v '\.bare$')
			COMPREPLY=($(compgen -W "$worktrees" -- "$cur"))
		fi
		;;
	update | u | switch | list | migrate | help)
		if [[ $cur == -* ]]; then
			COMPREPLY=($(compgen -W "--help -h" -- "$cur"))
		fi
		;;
	lock | unlock | move | prune | repair)
		# Pass-through commands - complete with worktree paths
		local worktrees
		worktrees=$(git worktree list --porcelain 2>/dev/null | grep '^worktree ' | sed 's/^worktree //' | grep -v '\.bare$')
		COMPREPLY=($(compgen -W "$worktrees" -- "$cur"))
		;;
	esac
}

complete -F _git_wt git-wt

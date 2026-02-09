# Fish completions for git-wt

# Disable file completions by default
complete -c git-wt -f

# Helper functions
function __fish_git_wt_needs_command
    set -l cmd (commandline -opc)
    test (count $cmd) -eq 1
end

function __fish_git_wt_using_command
    set -l cmd (commandline -opc)
    test (count $cmd) -gt 1; and test "$cmd[2]" = "$argv[1]"
end

function __fish_git_wt_worktrees
    git worktree list --porcelain 2>/dev/null | string match -r '^worktree (.*)' | string replace 'worktree ' '' | string match -v '*.bare'
end

function __fish_git_wt_remote_branches
    git branch -r --format='%(refname:short)' 2>/dev/null | string match -v 'HEAD' | string replace 'origin/' ''
end

function __fish_git_wt_all_branches
    git branch -a --format='%(refname:short)' 2>/dev/null | string replace 'origin/' '' | sort -u
end

# Top-level flags
complete -c git-wt -n __fish_git_wt_needs_command -s h -l help -d 'Show help'

# Main commands
complete -c git-wt -n __fish_git_wt_needs_command -a clone -d 'Clone a repository with worktree structure'
complete -c git-wt -n __fish_git_wt_needs_command -a migrate -d 'Migrate an existing repository to use worktrees (experimental)'
complete -c git-wt -n __fish_git_wt_needs_command -a add -d 'Create a new worktree'
complete -c git-wt -n __fish_git_wt_needs_command -a remove -d 'Remove worktree(s) and delete local branch(es)'
complete -c git-wt -n __fish_git_wt_needs_command -a rm -d 'Remove worktree(s) and delete local branch(es)'
complete -c git-wt -n __fish_git_wt_needs_command -a destroy -d 'Remove worktree(s) and delete local and remote branch(es)'
complete -c git-wt -n __fish_git_wt_needs_command -a update -d 'Fetch and update the default branch worktree'
complete -c git-wt -n __fish_git_wt_needs_command -a u -d 'Fetch and update the default branch worktree'
complete -c git-wt -n __fish_git_wt_needs_command -a switch -d 'Interactively switch to a different worktree'
complete -c git-wt -n __fish_git_wt_needs_command -a list -d 'List all worktrees'
complete -c git-wt -n __fish_git_wt_needs_command -a lock -d 'Lock a worktree'
complete -c git-wt -n __fish_git_wt_needs_command -a unlock -d 'Unlock a worktree'
complete -c git-wt -n __fish_git_wt_needs_command -a move -d 'Move a worktree'
complete -c git-wt -n __fish_git_wt_needs_command -a prune -d 'Prune stale worktree information'
complete -c git-wt -n __fish_git_wt_needs_command -a repair -d 'Repair worktree administrative files'
complete -c git-wt -n __fish_git_wt_needs_command -a help -d 'Show help'

# clone completions
complete -c git-wt -n '__fish_git_wt_using_command clone' -s h -l help -d 'Show help'
complete -c git-wt -n '__fish_git_wt_using_command clone' -F -d 'Folder name'

# add completions
complete -c git-wt -n '__fish_git_wt_using_command add' -s h -l help -d 'Show help'
complete -c git-wt -n '__fish_git_wt_using_command add' -s b -d 'Create new branch' -xa '(__fish_git_wt_all_branches)'
complete -c git-wt -n '__fish_git_wt_using_command add' -s B -d 'Create/reset branch' -xa '(__fish_git_wt_all_branches)'
complete -c git-wt -n '__fish_git_wt_using_command add' -xa '(__fish_git_wt_remote_branches)'

# remove/rm completions
complete -c git-wt -n '__fish_git_wt_using_command remove' -s h -l help -d 'Show help'
complete -c git-wt -n '__fish_git_wt_using_command remove' -s n -l dry-run -d 'Show what would be removed'
complete -c git-wt -n '__fish_git_wt_using_command remove' -xa '(__fish_git_wt_worktrees)'

complete -c git-wt -n '__fish_git_wt_using_command rm' -s h -l help -d 'Show help'
complete -c git-wt -n '__fish_git_wt_using_command rm' -s n -l dry-run -d 'Show what would be removed'
complete -c git-wt -n '__fish_git_wt_using_command rm' -xa '(__fish_git_wt_worktrees)'

# destroy completions
complete -c git-wt -n '__fish_git_wt_using_command destroy' -s h -l help -d 'Show help'
complete -c git-wt -n '__fish_git_wt_using_command destroy' -s n -l dry-run -d 'Show what would be destroyed'
complete -c git-wt -n '__fish_git_wt_using_command destroy' -xa '(__fish_git_wt_worktrees)'

# update/u completions
complete -c git-wt -n '__fish_git_wt_using_command update' -s h -l help -d 'Show help'
complete -c git-wt -n '__fish_git_wt_using_command u' -s h -l help -d 'Show help'

# switch completions
complete -c git-wt -n '__fish_git_wt_using_command switch' -s h -l help -d 'Show help'

# list completions
complete -c git-wt -n '__fish_git_wt_using_command list' -s h -l help -d 'Show help'

# migrate completions
complete -c git-wt -n '__fish_git_wt_using_command migrate' -s h -l help -d 'Show help'

# lock/unlock/move/prune/repair completions
complete -c git-wt -n '__fish_git_wt_using_command lock' -s h -l help -d 'Show help'
complete -c git-wt -n '__fish_git_wt_using_command lock' -xa '(__fish_git_wt_worktrees)'
complete -c git-wt -n '__fish_git_wt_using_command unlock' -s h -l help -d 'Show help'
complete -c git-wt -n '__fish_git_wt_using_command unlock' -xa '(__fish_git_wt_worktrees)'
complete -c git-wt -n '__fish_git_wt_using_command move' -s h -l help -d 'Show help'
complete -c git-wt -n '__fish_git_wt_using_command move' -xa '(__fish_git_wt_worktrees)'
complete -c git-wt -n '__fish_git_wt_using_command prune' -s h -l help -d 'Show help'
complete -c git-wt -n '__fish_git_wt_using_command prune' -xa '(__fish_git_wt_worktrees)'
complete -c git-wt -n '__fish_git_wt_using_command repair' -s h -l help -d 'Show help'
complete -c git-wt -n '__fish_git_wt_using_command repair' -xa '(__fish_git_wt_worktrees)'

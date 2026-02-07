#compdef git-wt
#description Git worktree management tool

_git-wt() {
    local -a commands
    commands=(
        'clone:Clone a repository with worktree structure'
        'migrate:Migrate an existing repository to use worktrees'
        'add:Create a new worktree'
        'remove:Remove worktree(s) and delete local branch(es)'
        'rm:Remove worktree(s) and delete local branch(es)'
        'destroy:Remove worktree(s) and delete local and remote branch(es)'
        'update:Fetch and update the default branch worktree'
        'u:Fetch and update the default branch worktree'
        'switch:Interactively switch to a different worktree'
        'list:List all worktrees'
        'lock:Lock a worktree'
        'unlock:Unlock a worktree'
        'move:Move a worktree'
        'prune:Prune stale worktree information'
        'repair:Repair worktree administrative files'
        'help:Show help'
    )

    _arguments -C \
        '(- *)'{-h,--help}'[Show help]' \
        '1: :->command' \
        '*:: :->args'

    case $state in
        command)
            _describe -t commands 'git-wt commands' commands
            ;;
        args)
            case $words[1] in
                clone)
                    _arguments \
                        '(-h --help)'{-h,--help}'[Show help]' \
                        '1:repository URL:' \
                        '2:folder name:_files -/'
                    ;;
                add)
                    _arguments \
                        '(-h --help)'{-h,--help}'[Show help]' \
                        '-b[Create new branch]:branch name:__git_branch_names' \
                        '-B[Create/reset branch]:branch name:__git_branch_names' \
                        '*:path or commit-ish:__git_wt_add_completions'
                    ;;
                remove|rm)
                    _arguments \
                        '(-h --help)'{-h,--help}'[Show help]' \
                        '(-n --dry-run)'{-n,--dry-run}'[Show what would be removed]' \
                        '*:worktree:__git_wt_worktrees'
                    ;;
                destroy)
                    _arguments \
                        '(-h --help)'{-h,--help}'[Show help]' \
                        '(-n --dry-run)'{-n,--dry-run}'[Show what would be destroyed]' \
                        '*:worktree:__git_wt_worktrees'
                    ;;
                update|u|switch|list|migrate|help)
                    _arguments \
                        '(-h --help)'{-h,--help}'[Show help]'
                    ;;
                lock|unlock|move|prune|repair)
                    _arguments \
                        '(-h --help)'{-h,--help}'[Show help]' \
                        '*:worktree:__git_wt_worktrees'
                    ;;
            esac
            ;;
    esac
}

__git_wt_worktrees() {
    local -a worktrees
    worktrees=(${(f)"$(git worktree list --porcelain 2>/dev/null | grep '^worktree ' | sed 's/^worktree //' | grep -v '\.bare$')"})
    _describe -t worktrees 'worktrees' worktrees
}

__git_wt_add_completions() {
    local -a branches
    branches=(${(f)"$(git branch -r --format='%(refname:short)' 2>/dev/null | grep -v HEAD | sed 's|^origin/||')"})
    _describe -t branches 'remote branches' branches
    _files -/
}

__git_branch_names() {
    local -a branches
    branches=(${(f)"$(git branch -a --format='%(refname:short)' 2>/dev/null | sed 's|^origin/||' | sort -u)"})
    _describe -t branches 'branches' branches
}

_git-wt "$@"

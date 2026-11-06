package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const bashCompletion = `# git-release completion for bash. Load with: . <(git release complete bash)
# Hooks into git's own completion (git-completion.bash) via _git_release.
_git_release()
{
	case "$prev" in
	--prefix)
		__gitcomp "v"
		return
		;;
	--branch)
		__gitcomp_nl "$(__git_heads)"
		return
		;;
	undo)
		__gitcomp "--yes --dry-run --force-remote $(git tag --list 2>/dev/null)"
		return
		;;
	config)
		__gitcomp "--init"
		return
		;;
	complete)
		__gitcomp "bash zsh fish"
		return
		;;
	help)
		return
		;;
	esac
	case "$cur" in
	--*)
		__gitcomp "--dry-run --yes --branch= --prefix= --pre= --sign --no-pre"
		return
		;;
	esac
	__gitcomp "major minor patch undo config complete help"
}
`

const zshCompletion = `#compdef git-release
# git-release completion for zsh. Load with: . <(git release complete zsh)
# zsh's git completion picks up _git-release for "git release".
_git-release() {
	local curcontext="$curcontext" state line
	typeset -A opt_args
	local -a subcmds
	subcmds=(
		'major:bump the major version'
		'minor:bump the minor version'
		'patch:bump the patch version'
		'undo:revert the latest release'
		'config:show effective configuration (--init writes a starter file)'
		'complete:print shell completion script'
		'help:show detailed help'
	)
	_arguments -C \
		'--dry-run[show what would happen, change nothing]' \
		'(-y --yes)'{-y,--yes}'[skip the confirmation prompt]' \
		'--branch=[release branch]:branch:__git_branch_names' \
		'--prefix=[tag style]:prefix:(v)' \
		'--pre=[pre-release identifier]:identifier:' \
		'--sign[create a signed tag]' \
		'--no-pre[skip pre steps]' \
		'1: :->cmd' \
		'*:: :->args'
	case $state in
	cmd)
		_describe -t commands 'git release command' subcmds
		;;
	args)
		case $line[1] in
		undo)
			_arguments '--yes[skip confirmation]' '--dry-run[only print the plan]' '--force-remote[also delete the remote tag]' '1:tag:__git_tags'
			;;
		config)
			_arguments '--init[write a starter config file]'
			;;
		complete)
			_values 'shell' bash zsh fish
			;;
		esac
		;;
	esac
}
`

const fishCompletion = `# git-release completion for fish. Load with: git release complete fish | source
complete -c git -n '__fish_git_using_command release' -f
complete -c git -n '__fish_git_using_command release; and not __fish_seen_subcommand_from undo config complete help' -a 'major minor patch undo config complete help'
complete -c git -n '__fish_git_using_command release' -l dry-run -d 'show what would happen, change nothing'
complete -c git -n '__fish_git_using_command release' -s y -l yes -d 'skip the confirmation prompt'
complete -c git -n '__fish_git_using_command release' -l branch -x -a '(git for-each-ref --format="%(refname:short)" refs/heads 2>/dev/null)' -d 'release branch'
complete -c git -n '__fish_git_using_command release' -l prefix -x -a 'v' -d 'tag style'
complete -c git -n '__fish_git_using_command release' -l pre -x -d 'pre-release identifier'
complete -c git -n '__fish_git_using_command release' -l sign -d 'create a signed tag'
complete -c git -n '__fish_git_using_command release' -l no-pre -d 'skip pre steps'
complete -c git -n '__fish_git_using_command release; and __fish_seen_subcommand_from undo' -l force-remote -d 'also delete the remote tag'
complete -c git -n '__fish_git_using_command release; and __fish_seen_subcommand_from config' -l init -d 'write a starter config file'
complete -c git -n '__fish_git_using_command release; and __fish_seen_subcommand_from complete' -a 'bash zsh fish'
`

func (a *App) complete(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: git release complete [bash|zsh|fish]")
	}
	shell := ""
	if len(args) == 1 {
		shell = args[0]
	} else {
		shell = filepath.Base(os.Getenv("SHELL"))
	}
	switch shell {
	case "bash":
		io.WriteString(a.Out, bashCompletion)
	case "zsh":
		io.WriteString(a.Out, zshCompletion)
	case "fish":
		io.WriteString(a.Out, fishCompletion)
	case "":
		return errors.New("cannot detect shell: pass bash, zsh or fish")
	default:
		return fmt.Errorf("unsupported shell %q (supported: bash, zsh, fish)", shell)
	}
	return nil
}

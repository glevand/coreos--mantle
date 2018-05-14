#!/bin/bash
#
# Bash completion for CoreOS Mantle.
#
# https://www.gnu.org/software/bash/manual/html_node/Programmable-Completion-Builtins.html
#

_cork() {
	#set -x
	local cmd="${1##*/}"
	local line=${COMP_LINE}
	local word=${COMP_WORDS[COMP_CWORD]}

	case "${line}" in
	*cork*create*)
		COMPREPLY=( $(compgen -W '--chroot --manifest-branch --manifest-name --manifest-url --replace --sdk-version --verify --verify-key --verify-signature' --  ${word} ) )
		;;
	*)
		case "${word}" in
		--*)
			COMPREPLY=( $(compgen -W '--debug --help --log-level --verbose' --  ${word}) )
			;;
		-*)
			COMPREPLY=( $(compgen -W '-d --debug -h --help --log-level -v --verbose' --  ${word}) )
			;;
		*)
			COMPREPLY=( $(compgen -W 'build create delete download download-image enter setup update verify version' --  ${word}) )
			;;
		esac
		;;
	esac

	return 0
}

complete -F _cork cork

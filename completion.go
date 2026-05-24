package main

import (
	"fmt"
	"os"
)

func cmdCompletion(args []string) {
	shell := ""
	if len(args) > 0 {
		shell = args[0]
	}
	switch shell {
	case "powershell", "pwsh":
		fmt.Print(completionPowerShell)
	case "bash":
		fmt.Print(completionBash)
	default:
		fmt.Fprintf(os.Stderr, "Usage: gossh completion <shell>\n\nSupported shells: powershell, bash\n")
		os.Exit(2)
	}
}

const completionPowerShell = `Register-ArgumentCompleter -Native -CommandName gossh -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $subcommands = 'push','pull','ls','kubectl','helm','version','help','completion'

    $tokens = $commandAst.CommandElements
    $n      = $tokens.Count

    # Complete subcommand
    if ($n -le 2) {
        return $subcommands |
            Where-Object { $_ -like "$wordToComplete*" } |
            ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
            }
    }

    $sub = $tokens[1].Value

    # Only suggest flags when the current word starts with -
    if (-not $wordToComplete.StartsWith('-')) { return }

    $flags = switch ($sub) {
        'push'       { '-server','-remote-dir','-identity','-dry-run','-exclude' }
        'pull'       { '-server','-remote-dir','-identity' }
        'ls'         { '-server','-remote-dir','-identity','-l','-a','-h','-R' }
        'kubectl'    { '-server','-identity' }
        'helm'       { '-server','-identity' }
        'completion' { 'powershell','bash' }
        default      { '-server','-remote-dir','-identity' }
    }

    $flags |
        Where-Object { $_ -like "$wordToComplete*" } |
        ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }
}
`

const completionBash = `_gossh_completion() {
    local cur prev
    cur="${COMP_WORDS[COMP_CWORD]}"

    local subcommands="push pull ls kubectl helm version help completion"

    if [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=($(compgen -W "${subcommands}" -- "${cur}"))
        return
    fi

    local sub="${COMP_WORDS[1]}"

    if [[ "${cur}" == -* ]]; then
        local flags
        case "${sub}" in
            push)       flags="-server -remote-dir -identity -dry-run -exclude" ;;
            pull)       flags="-server -remote-dir -identity" ;;
            ls)         flags="-server -remote-dir -identity -l -a -h -R" ;;
            kubectl)    flags="-server -identity" ;;
            helm)       flags="-server -identity" ;;
            *)          flags="-server -remote-dir -identity" ;;
        esac
        COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
        return
    fi

    if [[ "${sub}" == "completion" ]]; then
        COMPREPLY=($(compgen -W "powershell bash" -- "${cur}"))
        return
    fi

    COMPREPLY=($(compgen -f -- "${cur}"))
}

complete -F _gossh_completion gossh
`

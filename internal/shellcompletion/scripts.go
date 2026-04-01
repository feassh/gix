package shellcompletion

func bashScript() string {
	return `# bash completion for gix
_gix_completion() {
    local IFS=$'\n'
    local current="${COMP_WORDS[COMP_CWORD]}"
    local count=$((COMP_CWORD - 1))
    local args=()
    if (( count > 0 )); then
        args=("${COMP_WORDS[@]:1:$count}")
    fi
    COMPREPLY=($(compgen -W "$("${COMP_WORDS[0]}" __complete "${args[@]}" "$current")" -- "$current"))
}

complete -F _gix_completion gix`
}

func zshScript() string {
	return `#compdef gix
autoload -U +X bashcompinit && bashcompinit

_gix_completion() {
    local IFS=$'\n'
    local current="${words[CURRENT]}"
    local -a args
    if (( CURRENT > 2 )); then
        args=("${(@)words[2,CURRENT-1]}")
    else
        args=()
    fi
    local -a completions
    completions=(${(f)"$(${words[1]} __complete "${args[@]}" "$current")"})
    compadd -- "${completions[@]}"
}

compdef _gix_completion gix`
}

func fishScript() string {
	return `function __gix_complete
    set -l tokens (commandline -opc)
    set -e tokens[1]
    set -l current (commandline -ct)
    if test (count $tokens) -gt 0
        if test "$tokens[-1]" = "$current"
            set -e tokens[-1]
        end
    end
    gix __complete $tokens $current
end

complete -c gix -f -a '(__gix_complete)'`
}

func powerShellScript() string {
	return `Register-ArgumentCompleter -CommandName gix -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $tokens = @()
    foreach ($element in $commandAst.CommandElements) {
        $tokens += $element.Extent.Text
    }
    if ($tokens.Count -gt 0) {
        $tokens = $tokens[1..($tokens.Count - 1)]
    }
    if ($tokens.Count -gt 0 -and $tokens[-1] -eq $wordToComplete) {
        if ($tokens.Count -eq 1) {
            $tokens = @()
        } else {
            $tokens = $tokens[0..($tokens.Count - 2)]
        }
    }
    $completions = & gix __complete @tokens $wordToComplete
    foreach ($completion in $completions) {
        [System.Management.Automation.CompletionResult]::new($completion, $completion, 'ParameterValue', $completion)
    }
}`
}

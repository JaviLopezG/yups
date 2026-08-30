#!/usr/bin/env bash
# completion.bash - Programmable tab-completion for yups.
# Provides dynamic flag, argument, and model autocompletion.

_yups-completion() {
    local cur prev words cword
    _init_completion -n = 2>/dev/null || {
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"
        words=("${COMP_WORDS[@]}")
        cword=$COMP_CWORD
    }

    local yupsFlags="-h --help -V --version -i --install-yups -u --uninstall-yups --update-yups --advanced --model --query --test-models --"

    if [ "$prev" = "--model" ]; then
        local models=""
        local cfgFile="${HOME}/.yups/config.toml"
        if [ -f "$cfgFile" ]; then
            models=$(grep -E '^[[:space:]]*(available-models|default-model|advanced-model)' "$cfgFile" 2>/dev/null | tr -d '[],"' | sed 's/.*=//')
        fi
        if [ -z "$models" ] && command -v ollama >/dev/null 2>&1; then
            models=$(ollama list 2>/dev/null | awk 'NR>1 {print $1}')
        fi
        if [ -z "$models" ]; then
            models="qwen3-coder:latest qwen3.8:latest gemma3:latest gemma4:latest codestral:latest"
        fi
        COMPREPLY=( $(compgen -W "$models" -- "$cur") )
        return 0
    fi

    if [[ "$cur" == -* ]]; then
        COMPREPLY=( $(compgen -W "$yupsFlags" -- "$cur") )
        return 0
    fi

    local hasCmd=0
    for (( i=1; i < cword; i++ )); do
        local w="${words[i]}"
        if [[ "$w" != -* ]] || [[ "$w" == "--" ]]; then
            hasCmd=1
            break
        fi
    done

    if [ $hasCmd -eq 0 ]; then
        COMPREPLY=( $(compgen -c -- "$cur") )
    else
        COMPREPLY=( $(compgen -f -- "$cur") )
    fi
}

complete -F _yups-completion yups


#!/usr/bin/env bash
# yups.bash - Main entrypoint for YUPS shell integration.
# THIS FILE IS AUTOMATICALLY UPDATED ON REINSTALLATION OR UPDATE.
# Discovers and sources all modular shell scripts (*.bash) located in ~/.yups/shell/.

_yups_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd)"
if [ -d "$_yups_dir" ]; then
    for _yups_script in "$_yups_dir"/*.bash; do
        if [ -f "$_yups_script" ] && [ "$_yups_script" != "$_yups_dir/yups.bash" ]; then
            source "$_yups_script"
        fi
    done
    unset _yups_script
fi
unset _yups_dir


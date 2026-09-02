#!/usr/bin/env bash
# env.bash - Shell wrapper function for the yups command.
# THIS FILE IS AUTOMATICALLY UPDATED ON REINSTALLATION OR UPDATE.
# Captures recent command history and passes it to yups for contextual analysis.

yups() {
    local yupsHist yupsMarker
    yupsHist=$(mktemp "${TMPDIR:-/tmp}/yups-hist.XXXXXX.kk" 2>/dev/null || echo "/tmp/yups-hist.$$.kk")
    yupsMarker=$(mktemp "${TMPDIR:-/tmp}/yups-marker.XXXXXX.kk" 2>/dev/null || echo "/tmp/yups-marker.$$.kk")
    local oldHistTimeFormat="$HISTTIMEFORMAT"
    HISTTIMEFORMAT="%Y-%m-%d %H:%M:%S  " history 25 > "$yupsHist" 2>/dev/null
    HISTTIMEFORMAT="$oldHistTimeFormat"
    YUPS_READLINE_MARKER="$yupsMarker" YUPS_SESSION_HISTORY="$yupsHist" command yups "$@"
    local ret=$?
    if [ -s "$yupsMarker" ]; then
        local markerContent
        markerContent=$(cat "$yupsMarker" 2>/dev/null)
        if [ "$markerContent" != "executed" ] && [ -n "$markerContent" ]; then
            export YUPS_SCRIPT="$markerContent"
            history -s "$markerContent" 2>/dev/null
            if [ -t 0 ] && [ -t 1 ]; then
                local nextCmd
                read -e -i "$markerContent" -p "${PS1@P}" nextCmd
                if [ -n "$nextCmd" ]; then
                    history -s "$nextCmd" 2>/dev/null
                    eval "$nextCmd"
                    ret=$?
                fi
            fi
        fi
    fi
    rm -f "$yupsHist" "$yupsMarker"
    return $ret
}


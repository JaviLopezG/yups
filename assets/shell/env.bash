#!/usr/bin/env bash
# env.bash - Shell wrapper function for the yups command.
# Captures recent command history and passes it to yups for contextual analysis.

yups() {
    local yupsHist
    yupsHist=$(mktemp "${TMPDIR:-/tmp}/yups-hist.XXXXXX.kk" 2>/dev/null || echo "/tmp/yups-hist.$$.kk")
    HISTTIMEFORMAT= history 25 > "$yupsHist" 2>/dev/null
    YUPS_SESSION_HISTORY="$yupsHist" command yups "$@"
    local ret=$?
    rm -f "$yupsHist"
    return $ret
}


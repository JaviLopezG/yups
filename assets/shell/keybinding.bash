#!/usr/bin/env bash
# keybinding.bash - Readline key binding integration for yups.
# Binds a keyboard shortcut to explain or fix the current command line in-place.

_yups-readline-binding() {
    local origLine="$READLINE_LINE"
    local origPoint="$READLINE_POINT"
    local oldTtySettings
    oldTtySettings=$(stty -g < /dev/tty 2>/dev/null)

    # Enable canonical mode, echo, and carriage return translation
    stty sane icanon echo icrnl < /dev/tty 2>/dev/null

    local yupsMarker
    yupsMarker=$(mktemp "${TMPDIR:-/tmp}/yups-exec.XXXXXX.kk" 2>/dev/null || echo "/tmp/yups-exec.$$.kk")
    local yupsHist
    yupsHist=$(mktemp "${TMPDIR:-/tmp}/yups-hist.XXXXXX.kk" 2>/dev/null || echo "/tmp/yups-hist.$$.kk")
    local oldHistTimeFormat="$HISTTIMEFORMAT"
    HISTTIMEFORMAT="%%Y-%%m-%%d %%H:%%M:%%S  " history 25 > "$yupsHist" 2>/dev/null
    HISTTIMEFORMAT="$oldHistTimeFormat"

    if [ -z "$READLINE_LINE" ]; then
        YUPS_READLINE_MARKER="$yupsMarker" YUPS_SESSION_HISTORY="$yupsHist" command yups < /dev/tty
    else
        YUPS_READLINE_MARKER="$yupsMarker" YUPS_SESSION_HISTORY="$yupsHist" command yups -- "$READLINE_LINE" < /dev/tty
    fi

    # Restore previous terminal settings for Readline
    if [ -n "$oldTtySettings" ]; then
        stty "$oldTtySettings" < /dev/tty 2>/dev/null
    fi

    if [ -s "$yupsMarker" ]; then
        local markerContent
        markerContent=$(cat "$yupsMarker" 2>/dev/null)
        if [ "$markerContent" = "executed" ]; then
            READLINE_LINE=""
            READLINE_POINT=0
        elif [ -n "$markerContent" ]; then
            READLINE_LINE="$markerContent"
            READLINE_POINT=${#READLINE_LINE}
        else
            READLINE_LINE=""
            READLINE_POINT=0
        fi
    else
        READLINE_LINE="$origLine"
        READLINE_POINT="$origPoint"
    fi
    rm -f "$yupsMarker" "$yupsHist"
}

# ==============================================================================
# READLINE KEY BINDING CONFIGURATION & KEY CODES REFERENCE
# ==============================================================================
#
# Active key: %s
#
# ------------------------------------------------------------------------------
# HOW TO CUSTOMIZE THIS SHORTCUT MANUALLY:
# ------------------------------------------------------------------------------
# You can edit the 'bind -x' directive below.
# Syntax: bind -x '"<escape-sequence>": _yups-readline-binding'
#
# TIP: To find the exact escape sequence produced by your terminal emulator for
# any key combination, run 'cat' or in a bash prompt press 'Ctrl+V' followed by
# your desired key. For example, pressing Ctrl+V then F1 outputs: ^[OP
# (where '^[' denotes the escape character '\e').
#
# ------------------------------------------------------------------------------
# COMMON KEY CODES REFERENCE TABLE (GNU Readline / Bash):
# ------------------------------------------------------------------------------
#
# 1. FUNCTION KEYS (F1 - F12):
#    F1:   \eOP      (alternatives in some terms: \e[11~ or \e[[A)
#    F2:   \eOQ      (alternatives: \e[12~ or \e[[B)
#    F3:   \eOR      (alternatives: \e[13~ or \e[[C)
#    F4:   \eOS      (alternatives: \e[14~ or \e[[D)
#    F5:   \e[15~
#    F6:   \e[17~
#    F7:   \e[18~
#    F8:   \e[19~
#    F9:   \e[20~
#    F10:  \e[21~
#    F11:  \e[23~
#    F12:  \e[24~
#
# 2. CONTROL COMBINATIONS (Ctrl + Key):
#    Format: \C-<key> (lowercase)
#    Ctrl+a: \C-a    Ctrl+b: \C-b    Ctrl+c: \C-c    Ctrl+d: \C-d
#    Ctrl+e: \C-e    Ctrl+f: \C-f    Ctrl+g: \C-g    Ctrl+h: \C-h
#    Ctrl+i: \C-i (Tab)              Ctrl+j: \C-j (LineFeed)
#    Ctrl+k: \C-k    Ctrl+l: \C-l    Ctrl+m: \C-m (Enter)
#    Ctrl+n: \C-n    Ctrl+o: \C-o    Ctrl+p: \C-p    Ctrl+q: \C-q
#    Ctrl+r: \C-r    Ctrl+s: \C-s    Ctrl+t: \C-t    Ctrl+u: \C-u
#    Ctrl+v: \C-v    Ctrl+w: \C-w    Ctrl+x: \C-x    Ctrl+y: \C-y
#    Ctrl+z: \C-z
#    Ctrl+Space: \C-@ or \C-   Ctrl+]: \C-]   Ctrl+^: \C-^   Ctrl+_: \C-_
#
# 3. ALT / META COMBINATIONS (Alt + Key):
#    Format: \e<key> or \M-<key>
#    Alt+a .. Alt+z: \ea .. \ez
#    Alt+0 .. Alt+9: \e0 .. \e9
#    Alt+Space: \e 
#
# 4. SHIFT COMBINATIONS:
#    Shift+Tab (BackTab): \e[Z
#    Shift+F1:  \e[1;2P   (or \e[25~)     Shift+F7:  \e[18;2~ (or \e[33~)
#    Shift+F2:  \e[1;2Q   (or \e[26~)     Shift+F8:  \e[19;2~ (or \e[34~)
#    Shift+F3:  \e[1;2R   (or \e[28~)     Shift+F9:  \e[20;2~
#    Shift+F4:  \e[1;2S   (or \e[29~)     Shift+F10: \e[21;2~
#    Shift+F5:  \e[15;2~  (or \e[31~)     Shift+F11: \e[23;2~
#    Shift+F6:  \e[17;2~  (or \e[32~)     Shift+F12: \e[24;2~
#    Shift+Up:   \e[1;2A  Shift+Down:  \e[1;2B
#    Shift+Right:\e[1;2C  Shift+Left:  \e[1;2D
#
# 5. COMBINED MODIFIERS (Ctrl+Alt / Ctrl+Shift / Alt+Shift):
#    Ctrl+Alt+a .. Ctrl+Alt+z: \e\C-a .. \e\C-z
#    Ctrl+Up:    \e[1;5A  Ctrl+Down:   \e[1;5B
#    Ctrl+Right: \e[1;5C  Ctrl+Left:   \e[1;5D
#    Alt+Up:     \e[1;3A  Alt+Down:    \e[1;3B
#    Alt+Right:  \e[1;3C  Alt+Left:    \e[1;3D
#
# 6. NAVIGATION & EDITING KEYS:
#    Insert:     \e[2~    Delete:      \e[3~
#    Home:       \e[H     (or \e[1~, \eOH)
#    End:        \e[F     (or \e[4~, \eOF)
#    Page Up:    \e[5~    Page Down:   \e[6~
#    Up Arrow:   \e[A     Down Arrow:  \e[B
#    Right Arrow:\e[C     Left Arrow:  \e[D
#
# 7. SPECIAL MODIFIERS (Left/Right Modifiers & Super/Meta Key):
#    - Left vs Right Modifiers (Ctrl, Alt, Shift): The OS and terminal emulator
#      convert physical keycodes to uniform ASCII/escape bytes before passing
#      them to Readline. Hence, Left-Ctrl and Right-Ctrl (or Left-Shift and
#      Right-Shift) send identical codes to the shell. Right-Alt (AltGr) on
#      many international layouts is reserved for alternate characters and may
#      not trigger Meta escape sequences.
#    - Super / Windows / Command Key (Meta): Handled primarily by the window
#      manager/desktop environment (GNOME, KDE, Sway, macOS). When configured
#      to pass through to terminal emulators, it typically translates to Meta
#      (\e prefix) or custom terminal-defined sequences.
# ==============================================================================

%s


// Package assets provides embedded static resources (shell scripts, python tools).
package assets

import (
	_ "embed"
)

// InspectSessionPy contains the contents of inspect-session.py.
//
//go:embed inspect-session.py
var InspectSessionPy string

// ShellYupsBash contains the contents of assets/shell/yups.bash.
//
//go:embed shell/yups.bash
var ShellYupsBash string

// ShellEnvBash contains the contents of assets/shell/env.bash.
//
//go:embed shell/env.bash
var ShellEnvBash string

// ShellCompletionBash contains the contents of assets/shell/completion.bash.
//
//go:embed shell/completion.bash
var ShellCompletionBash string

// ShellKeybindingBash contains the template contents of assets/shell/keybinding.bash.
//
//go:embed shell/keybinding.bash
var ShellKeybindingBash string

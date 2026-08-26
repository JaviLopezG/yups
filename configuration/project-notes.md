# Environment-specific notes

Behaviours of `yups` that only show up in specific environments. None of these
are bugs to fix blindly: they are documented trade-offs and known interactions.

## `--install-yups` may touch the shell prompt (PS1) — VS Code / Cline
Planned behaviour for a future iteration: installing yups customises the shell
prompt (for example to show the yups marker in the PS1). Changing PS1 from an
installer can confuse tools that need to *see* clean command output, notably:

- **VS Code terminal + shell-integration**: prompt decorations injected by the
  installer may break command-output boundary detection.
- **AI coding agents running in the IDE** (e.g. Cline): they rely on that same
  shell integration (or on timing heuristics when it is missing) to decide
  whether a command finished; extra prompt escapes make them report "command
  completion could not be observed".

Mitigations to implement when PS1 support lands:

1. Only modify PS1 for interactive shells (guard with `case $- in *i*)` or
   `$- == *i*`), never in non-interactive/CI shells.
2. Write the change into `.bashrc`/`.zshrc` as a separate, clearly marked,
   removable block (`# >>> yups >>>` ... `# <<< yups <<<`) so uninstalling
   restores the original prompt exactly.
3. Provide `yups --install-yups --no-prompt` to opt out.


Related to the above but independent of yups: agents using the VS Code terminal
sometimes lose stdout capture mid-session (commands still run; output is not
reported back). This happened for real while developing yups and was diagnosed
on 2026-08-25. It is an environment issue, not something yups should work
around.

### Symptom

Every command run by the agent reported:

> `[Shell integration did not report command completion]`

even trivial ones (`echo`). Commands always executed fine; what failed was VS
Code's ability to observe command start/end, so Cline fell back to a timing
heuristic that often returned nothing and always assumed failure. Explicitly
ruled out: losing window focus or clicking around, and long sleeps (an instant
`echo` failed too).

### Root cause

Two independent customizations were fighting VS Code's shell integration (the
OSC 633 markers VS Code injects into the prompt plus a `DEBUG` trap to detect
command boundaries):

1. **Personal prompt/title customization in `~/.bashrc`.** `_update_prompt` was
   assigned directly (`PROMPT_COMMAND=_update_prompt`) and later prepended to
   (`PROMPT_COMMAND="_mytitle_precmd; $PROMPT_COMMAND"`). Running extra commands
   at prompt time fires VS Code's `DEBUG` trap out of order and, depending on
   load order, can overwrite the integration wrapper `__vsc_prompt_cmd_original`
   entirely.

2. **systemd's own terminal integration ("OSC 3008").**
   `/etc/profile.d/80-systemd-osc-context.sh` (systemd >= 257) appends
   `__systemd_osc_context_precmdline` to the `PROMPT_COMMAND` array and sets
   `PS0`. That function runs *after* VS Code's `__vsc_precmd` at every prompt
   redraw, triggering VS Code's `DEBUG` trap mid-prompt. Smoking gun found in
   the session state: `__vsc_current_command=__systemd_osc_context_precmdline`.

   The result is a desynced state machine: VS Code never sees a clean
   command-end marker, so command completion is never reported. This most likely
   started with a system update bringing the systemd snippet, which explains why
   capture worked at first and then silently stopped.

Live confirmation: neutralizing the systemd hook inside the running shell
(`PS0=''`, filtering `*systemd_osc*` entries out of `PROMPT_COMMAND`, `unset -f`
of its functions) made capture work again immediately.

### Solution applied (in `~/.bashrc`, outside this repo)

The whole customization block is wrapped so it only loads in ordinary
interactive shells, and is skipped entirely when the agent runs commands inside
VS Code:

```bash
if [ -z "$CLINE_ACTIVE" ] || [[ "$TERM_PROGRAM" != "vscode" ]]; then
    # ...aliases, _update_prompt, _mytitle_precmd, binds...
fi
```

Cline exports `CLINE_ACTIVE=1` in terminals it drives, so in that case none of
the personal `PROMPT_COMMAND`/`PS1` hooks are registered and VS Code's shell
integration works untouched. Verified with a repeatable echo/sleep battery
(mixed `sleep 1/3/2`, a 5x loop, and 20 rapid echos): full output captured and
completion observed successfully.

If it ever recurs, fallbacks are (a) redirecting command output to a temporary
file and reading it, or (b) disabling the systemd snippet as root, following the
instructions in its own header comment:

```bash
test -h /etc/profile.d/80-systemd-osc-context.sh && \
    rm -v /etc/profile.d/80-systemd-osc-context.sh && \
    ln -s /dev/null /etc/tmpfiles.d/20-systemd-osc-context.conf
```

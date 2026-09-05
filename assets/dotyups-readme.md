# yups Configuration and Customizations

This directory (`~/.yups/`) contains your local configuration, session logs, and
optional asset overrides.

By default, all standard assets (system prompt, help text, color themes, command
whitelists, and session names) are embedded directly inside the `yups` binary.

## Customizing Assets

If you wish to customize any default behaviors or tables, you can create the
corresponding file in this directory. When a file exists here, `yups` loads your
version instead of the embedded binary resource:

- `prompts/system_prompt.txt`: Custom system prompt template used for LLM
  inference (supports `{{NONCE}}` and `{{SYSTEM_CONTEXT}}` placeholders).
- `prompts/help.txt`: Custom CLI help text displayed on `yups --help`.
- `data/theme.toml`: Semantic ANSI terminal color theme and palette.
- `data/session-names.txt`: Custom wordlist for session slug names.
- `data/whitelist_commands.txt`: Read-only commands permitted unconditionally
  for the `command-run` tool.
- `data/whitelist_wrappers.txt`: Safe command wrappers (e.g. `sudo`, `env`).
- `data/whitelist_conditional_commands.txt`: Commands permitted conditionally
  when safe flags are used.

For reference copies of the default assets and syntax details, see the
repository assets directory:
https://code.javilopezg.com/javilopezg/yups/src/branch/main/assets/

## Other Files and Directories

- `config.toml`: General settings (Ollama endpoint, default and advanced models,
  limits, repository URLs).
- `state.toml`: Internal state tracking installed version, migrations, and
  keybindings.
- `logs/`: Session execution traces (`sessions.log`), tool execution traces
  (`tools.log`), and incident logs (`incidents.log`).
- `shell/`: Shell integration scripts (`yups.bash`, `env.bash`,
  `completion.bash`, `keybinding.bash`).
- `scripts/`: Utility tools such as `inspect-session.py`.

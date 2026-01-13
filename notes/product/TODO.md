# Yups

## Mission

The primary goal of Yups is to help users manage frustration with the terminal (Bash for now).

## Procedure

To accomplish its mission, Yups handles terminal errors using Bash functions and hooks like `command_not_found_handle` (error 127), `trap DEBUG`, the command prompt, and history analysis to stay informed about the user's intent.

Yups can also leverage external knowledge sources like tldr, navi cheatsheets, compgen, hash, complete, bin, Arch Wiki, The Fuck, and others.

It can also manage inference models:
- **Local**: Gemma functions via llama.cpp.
- **Remote**: Private servers (e.g., Gemma 3 running with Ollama on Marvin, a local server accessible via an API on Trillian — a VPS in Germany) or Hugging Face services.

**Note**: AI usage is not mandatory; it is a tool to accomplish the mission, not the objective itself.

We can provide models with system information and allow them to execute commands. We must classify commands as:
- **Harmless**: Non-mutating (e.g., `ls`).
- **Reversible**: Can be undone (e.g., `chmod +x file`).
- **Dangerous**: Destructive (e.g., `rm`).
- **Forbidden**: Critical system operations (e.g., formatting a drive).

We can also use dry-run commands to simulate execution or sandboxing when command safety is uncertain.

Since the mission is to reduce frustration, we must avoid unexpected lags. Messages should be clear, and the interface neat.

We must understand the user's intent even if it is not explicitly stated.

## Current Status

The project was halted for several weeks. It is a weekend project, so development time is inconsistent—ranging from three-day sprints to a single hour in a month.

I don't recall the specific details of the last changes.

The 'MVP' (Python script) only translated package manager instructions (e.g., "You are on Fedora and used `apt`, you should use `dnf`").

The next version must be a fast, easy-to-distribute program that can infer the user's intent and suggest the correct commands.

## TODOs

1. [ ] Manage Bash's "command not found" event.
2. [ ] Determine (using Gemma functions or a custom parser) which token caused the error.
3. [ ] Identify if a similar command exists (fuzzy search over `compgen -c`?). If yes, suggest the corrected `READLINE_LINE`.
4. [ ] If no similar command is installed, querying the package manager for a package that provides the missing command. If found, suggest installing it.
5. [ ] If the previous steps fail, query the inference model (providing context like command history, `pwd`, `ls` output) to determine the user's intent. It may execute commands to gather more info.
    - **Constraint**: Since this process takes time, we must ask the user if they want to wait, as they expect millisecond execution.
    - Always provide any useful information gathered, even if a full solution isn't found.

For the command error handler (errors != 127), we follow a similar process but focus on one problem at a time.

## Future

We may refine an online model specifically for this task (Codename: Project Babel Fish :)).

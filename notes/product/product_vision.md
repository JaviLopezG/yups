# Product Vision: The Straw-Boss

> **"You are the Manager. YUPS is the Straw-boss."**

## Core Concept
YUPS is not just a tool; it's an assistant that handles the "dirty work" of terminal management so the user can focus on actual tasks.

*   **The Problem**: Terminal frustration. Forgotten flags (`tar -xzvf`?), distro-specific commands (`dnf` vs `apt`), and typos (`git stats`).
*   **The Solution**: An intelligent wrapper that intervenes when things go wrong (errors) or when asked (natural language).

## Key Statistics / Values
1.  **Safety First**:
    *   Dangerous commands must be intercepted.
    *   **Dry-Run Protocol**: Simulate execution before applying changes (especially for `rm`, recursive deletes, etc.).
2.  **Polyglot**:
    *   The user shouldn't need to know if they are on Arch or Ubuntu.
    *   `yups install vlc` should work everywhere.
3.  **Low Latency**:
    *   The terminal must feel snappy.
    *   AI is used as a fallback, not the primary loop, unless explicitly requested.

## Roadmap & MVP Status
*   **Current MVP**: Python script. Functional but requires Python runtime.
*   **Next Version**: Go Binary. Single file, easy distribution, potential for embedded "Small Language Model" (SLM) for offline intelligence.

## User Persona
*   **Developers**: Who juggle multiple environments.
*   **Sysadmins**: Who need to be sure before hitting Enter on a production server.
*   **Beginners**: Who are intimidated by `man` pages.

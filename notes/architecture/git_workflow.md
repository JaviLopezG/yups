# Git Workflow & Repository Management

## Repository Structure
The project uses a **Monorepo** strategy. All components (Client, Server, Web, Infra) live in the same `yups` repository.
*   **Root**: Scripts and configuration for the repository itself.
*   **Subdirectories**: Distinct applications/modules.

## Branching Strategy (Gitflow)
The project follows a standard **Gitflow** model adapted for this project's intermittent development cycle:

*   **`main` / `master`**: Production-ready code. Releases are tagged here (e.g., `v1.0`).
*   **`develop`**: The main integration branch for the next release.
*   **Feature Branches** (`feature/*`): Created from `develop` for specific tasks or features. Merged back into `develop`.
*   **Release Branches** (`release/*`): Preparation for a new production release.
*   **Hotfix Branches** (`hotfix/*`): Urgent fixes for production, branched from `main`.

## Submodules
*   **`cli/llama.cpp`**: Linked to `https://github.com/ggml-org/llama.cpp`. Used by the Go CLI for internal bindings.

## Automation & Hooks
*   **Commit Signing**: Commits are co-authored by Gemini where applicable.
*   **CI/CD**: (To be documented - check `.github/` folder).

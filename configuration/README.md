# configuration/

Project-specific knowledge that is not needed to build or run `yups`, but
that anyone operating an environment around it (CI runners, release workers,
unusual shells...) will eventually need.

| File | Contents |
|---|---|
| [forgejo-runner.md](forgejo-runner.md) | What a Forgejo/Gitea runner needs to execute the CI workflows and the goreleaser worker. |
| [project-notes.md](project-notes.md) | Known interactions between yups and specific environments (VS Code/Cline, PS1 customisations...). |

New project-specific configuration knowledge goes here: if a behaviour of
yups surprises someone in a concrete setup, document it instead of letting
the next person rediscover it.

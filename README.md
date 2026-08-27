# yups

`yups` explains shell commands, prints the `#_?` marker in ANSI 256-colour 214,
and manages its own installation.

```bash
$ yups                # prints #_? in colour 214
$ yups -- ls -al /var/cache/man/
$ yups -- "ls -l || ps aux"
$ yups --help         # help with the options available so far
$ yups --install-yups
$ yups --uninstall-yups
$ yups --update-yups
```

## Commands

- `-- <command...>` (or `yups <command...>`): explains a shell command line.
  It tokenizes simple or compound pipelines (connected by `||`, `&&`, `;`, `|`,
  `|&`, `&`), unpacks clustered flags (`-al` -> `-a`, `-l`), identifies
  wrappers (`sudo`, `time`...), inspects positional arguments on the filesystem,
  and fetches documentation prioritizing `<command> --help` before falling back
  to `man` and `whatis`.
  Local basic documentation is displayed immediately. When a command or flag is
  not found locally, or when the command line includes a comment (`#...`),
  it announces the query and connects to the configured Ollama inference
  endpoint via direct HTTP. Shell comments are interpreted as user goals or
  questions, allowing the LLM to explain discrepancies and suggest optimal
  commands along with low-cost system context (OS distribution, working directory
  structure, referenced file snippets, and recent shell history).
  If the LLM suggests a command, `yups` prompts for action:
  `[Yes/no/edit/modifications]` (`y` executes immediately in subshell, `n` aborts,
  `e` enables interactive inline editing with cursor navigation, `m` sends
  feedback/followup to the LLM in a multi-turn conversation).
  If Ollama is unreachable, `yups` reports the connection error and, if unconfigured,
  suggests running `yups --install-yups`.

- `--help`: shows the help.

- `--version`: prints `yups vX.Y.Z` (overridden at build time; a plain
  `go build` reports `dev`).

- `--install-yups`:

  1. If the executable is already reachable through any `PATH` directory, it
     reports that it is already installed.
  2. Otherwise it checks whether a command named `yups` already exists in one of
     the well-known system binary directories (`/usr/local/bin`, `/usr/bin`,
     ...); if so, it reports that it is already installed.
  3. When several coexisting copies show up anywhere, the user is informed of
     all discovered duplicates while operating with the first instance found in
     `PATH` (matching `which` behavior).
  4. Then it looks for the first `PATH` directory where the current user can
     write. When there is none and the user has administrator privileges
     (membership of a well-known administrator group —`sudo`, `sudoer`,
     `sudoers`, `wheel` or `admin`—, or passwordless sudo rights) it suggests
     repeating the previous command with `sudo !!`; otherwise it reports that
     the installation is not possible.
  5. If everything is fine, the executable is copied into that directory, prompts
     for the Ollama inference endpoint (`http://localhost:11434` by default),
     automatically probes `/api/tags` to discover and configure models, and
     initializes `~/.yups/config.toml`.

- `--uninstall-yups`:

  1. If there is no `yups` command anywhere (PATH plus the well-known binary
     directories), it reports that it is already uninstalled.
  2. It locates every directory holding a copy of the command. When there are
     several, it asks whether to uninstall for all users; declining keeps the
     system-wide copies and removes only the ones inside the user home.
  3. It deletes every target copy the current user has permission to remove.
  4. If some copy could not be removed and the user has administrator privileges
     (see above), `sudo !!` is suggested.
  5. Finally, when `~/.yups` exists, it asks whether to keep configuration
     files and request history (default: yes). If the user declines, and has
     administrator privileges, it asks whether to delete the configuration and
     history for all users or only for the current user. Piped or closed stdin
     takes the defaults of every question.

- `--update-yups`:

  1. Loads `~/.yups/config.toml`. A missing file means defaults (with a warning
     and an offer to run the per-user installation); a corrupt file aborts with
     an explicit error.
  2. Queries the latest release from the primary repository (Forgejo) and, when
     unreachable, from the fallback (GitHub).
  3. If the release version is older than the current version (or if running a
     `dev` build), it informs the user and prompts for confirmation before
     performing a downgrade. If the versions match, it reports that it is up to
     date and exits.
  4. Downloads the platform archive plus `checksums.txt`, verifies the sha256
     digest, extracts the archive into an ephemeral staging directory (`yups-*.kk`)
     and self-validates the new binary (`--version` must report exactly the
     release tag).
  5. It replaces its own process image with the new binary (`--update-apply`),
     which atomically substitutes the keeper instance found in `PATH`
     (temporary `.kk` file in the target directory, `chmod 0755`, rename),
     updates `config.version`, applies any pending migrations tracked in
     `~/.yups/state.toml`, and safely cleans up the ephemeral staging directory.

Exit codes: `0` success, `1` error, `2` wrong usage.

### Policy decisions

Where does yups look, and what does it consider an administrator?

| Decision                     | Value                                                                                                              | Why                                                                                                                                                                       |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Install target               | First writable directory of the current `PATH`                                                                     | Respects whatever the user configured.                                                                                                                                    |
| "Already installed" check    | Every `PATH` directory plus `/usr/local/sbin`, `/usr/local/bin`, `/usr/sbin`, `/usr/bin`, `/sbin`, `/bin`          | Catches copies reachable through non-default PATHs.                                                                                                                       |
| Administrator groups         | `sudo`, `sudoer`, `sudoers` (Debian family), `wheel` (Fedora/RHEL/Arch/openSUSE), `admin` (historic Ubuntu, macOS) | The default administrator group of the mainstream distros.                                                                                                                |
| Administrator probe fallback | `sudo -n true` succeeds                                                                                            | Covers NOPASSWD sudoers and root on single-user systems where no group matches.                                                                                           |
| Write-permission probe       | Create + remove a temporary `.kk` file in the directory                                                            | More reliable than access(2) with ACLs, read-only mounts or sticky bits.                                                                                                  |
| Config file                  | `~/.yups/config.toml`: `version`, `YUPS_REPO`, `YUPS_REPO_FALLBACK`, `inference-endpoint`, `default-model`, `advanced-model` | `version` records the installed version; initialized during `--install-yups`; a corrupt file is an explicit error instead of silent defaults.                              |
| Inference endpoint           | Default: `http://localhost:11434` (Ollama HTTP API)                                                                | Queried directly via HTTP without third-party libraries; models discovered via `/api/tags` with smart code-model prioritization.                                           |
| Self-update sources          | Primary `YUPS_REPO` (Forgejo), fallback `YUPS_REPO_FALLBACK` (GitHub)                                              | Forgejo is the canonical source of truth; GitHub only rescues outages.                                                                                                    |
| Binary swap                  | Copy to temp `.kk` file in the target dir, chmod 0755, rename                                                      | Same-directory rename is atomic and can never fail with EXDEV.                                                                                                            |
| Multi-location anomaly       | First occurrence in `PATH` (matching `which`); informs user of duplicates                                         | Operates consistently with the binary the user would execute from shell. Duplicates are left untouched.                                                                    |
| Uninstall questions          | Defaults: uninstall for all users, keep configuration (`~/.yups`)                                                  | Non-destructive defaults; prompted deletion allows single-user or all-users cleanup for administrators.                                                                   |

## Development

Requirements: Go >= 1.25 and, for the integration tests, Docker.

### Make

The repository ships a small `Makefile` that wraps the everyday commands (build,
static checks, tests) behind short, memorable targets. `make` is not required by
the Go toolchain — every target is a plain one-line `go` command (see the next
section) — but it keeps the canonical invocations written down exactly once:
build tags, test flags, distro selection and the output filter
(`scripts/pretty-tests.sh`) that colours `PASS`/`FAIL`, splits the CamelCase
test names (`TestInstallTwiceReportsAlreadyInstalled` →
`Test Install Twice Reports Already Installed`), shows subtests by their leaf
name only, indents nested subtests and, on an interactive terminal, shows the
parallel suite as a live view whose summary lists only `FAIL`/`SKIP` results (or
reports that there is nothing to review).

```bash
$ make build                                # builds ./yups
$ make test                                 # go vet + lint + unit tests
$ make test-integration                     # integration tests, every distro (default)
$ make test-integration DISTRO=fedora       # one specific distro
$ make test-integration DISTRO=fedora,arch  # several distros
```

The integration scenarios run in parallel — every scenario gets its own
throwaway container, so the whole four-distro matrix costs little more than a
single distro. Concurrency is capped with `-parallel 8`; lower it if your
machine starts sweating. The pretty printer repaints the grouped view live; set
`REPAINT=never` for plain streaming output.

The integration target selects the systems under test with the make variable
`DISTRO`, which it translates into the `YUPS_TEST_DISTRO` environment variable
consumed by the tests themselves.

### Direct Go commands

If you prefer to skip make, these are the plain commands it wraps.

Build:

```bash
$ go build -o yups .
```

Static checks and unit tests (no Docker needed; unit tests live next to the code
in `internal/app` and use an in-memory fake of the operating system):

```bash
$ gofmt -l .
$ go vet ./...
$ golangci-lint run        # optional; skipped by `make test` when missing
$ go test -v ./...
```

Integration tests (`integration/`, build tag `integration`) build a linux binary
and run every scenario inside stock distro containers — root installs, plain
users without permissions, users in administrator groups, writable `~/bin`
directories, repeated installs, partial uninstalls, etc. Every distro runs by
default:

```bash
$ YUPS_TEST_DISTRO=all go test -tags integration -parallel 8 ./integration/ -count=1 -v
```

Select fewer systems with the `YUPS_TEST_DISTRO` environment variable (comma
separated names; unset means all):

```bash
$ YUPS_TEST_DISTRO=fedora go test -tags integration ./integration/ -count=1 -v
$ YUPS_TEST_DISTRO=fedora,arch go test -tags integration ./integration/ -count=1 -v
```

## CI and releases

There is a single set of workflows, `.github/workflows/`, executed by both
remotes (Forgejo reads `.github/workflows` too — as long as no
`.forgejo/workflows` directory exists):

- **Every push / pull request**: `ci.yml` builds, vets, lints, runs the unit
  tests and the full integration matrix. GitHub hosted runners provide the
  docker daemon; self-hosted runners expose theirs through `DOCKER_HOST` (see
  [configuration/forgejo-runner.md](configuration/forgejo-runner.md) — jobs
  never need privileged mode).
- **Tag push `vX.Y.Z`**: `release.yml` runs the full suite again and only then
  invokes [goreleaser](https://goreleaser.com): linux amd64/arm64 tarballs,
  checksums and changelog. Publishing target depends on where the tag lands —
  GitHub (`.goreleaser.yaml`) or the self-hosted Forgejo instance
  (`.goreleaser-forgejo.yaml`) — using in both cases the automatic per-run
  token; no user-defined secrets are required.

What a self-hosted worker needs (labels, docker daemon, security model) is
documented in
[configuration/forgejo-runner.md](configuration/forgejo-runner.md); other
environment-specific knowledge lives next to it.

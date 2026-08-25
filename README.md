# yups

`yups` prints the `#_?` marker in ANSI 256-colour 214 and manages its own
installation.

```bash
$ yups            # prints #_? in colour 214
$ yups --help     # help with the options available so far
$ yups --install
$ yups --uninstall
```

## Commands

- `--help`: shows the help.
- `--version`: prints `yups vX.Y.Z` (overridden at build time; a plain
  `go build` reports `dev`).
- `--install`:
  1. If the executable is already reachable through any `PATH` directory, it
     reports that it is already installed.
  2. Otherwise it checks whether a command named `yups` already exists in one
     of the well-known system binary directories (`/usr/local/bin`, `/usr/bin`,
     ...); if so, it reports that it is already installed.
  3. Then it looks for the first `PATH` directory where the current user can
     write. When there is none and the user has administrator privileges
     (membership of a well-known administrator group —`sudo`, `sudoer`,
     `sudoers`, `wheel` or `admin`—, or passwordless sudo rights) it
     suggests repeating the previous command with `sudo !!`; otherwise it
     reports that the installation is not possible.
  4. If everything is fine, the executable is copied into that directory.

- `--uninstall`:
  1. If there is no `yups` command anywhere (PATH plus the well-known binary
     directories), it reports that it is already uninstalled.
  2. It locates every directory holding a copy of the command.
  3. It deletes every copy the current user has permission to remove.
  4. If some copy could not be removed and the user has administrator
     privileges (see above), `sudo !!` is suggested.
  5. Otherwise the user is informed that the uninstall is not possible.

Exit codes: `0` success, `1` error, `2` wrong usage.

### Policy decisions

Where does yups look, and what does it consider an administrator?

| Decision | Value | Why |
|---|---|---|
| Install target | First writable directory of the current `PATH` | Respects whatever the user configured. |
| "Already installed" check | Every `PATH` directory plus `/usr/local/sbin`, `/usr/local/bin`, `/usr/sbin`, `/usr/bin`, `/sbin`, `/bin` | Catches copies reachable through non-default PATHs. |
| Administrator groups | `sudo`, `sudoer`, `sudoers` (Debian family), `wheel` (Fedora/RHEL/Arch/openSUSE), `admin` (historic Ubuntu, macOS) | The default administrator group of the mainstream distros. |
| Administrator probe fallback | `sudo -n true` succeeds | Covers NOPASSWD sudoers and root on single-user systems where no group matches. |
| Write-permission probe | Create + remove a temporary `.kk` file in the directory | More reliable than access(2) with ACLs, read-only mounts or sticky bits. |


## Development

Requirements: Go >= 1.25 and, for the integration tests, Docker.

### Make

The repository ships a small `Makefile` that wraps the everyday commands
(build, static checks, tests) behind short, memorable targets. `make` is not
required by the Go toolchain — every target is a plain one-line `go` command
(see the next section) — but it keeps the canonical invocations written down
exactly once: build tags, test flags, distro selection and the output filter
(`scripts/pretty-tests.sh`) that colours `PASS`/`FAIL`, splits the CamelCase
test names (`TestInstallTwiceReportsAlreadyInstalled` → `Test Install Twice
Reports Already Installed`), shows subtests by their leaf name only, indents
nested subtests and, on an interactive terminal, shows the parallel suite as
a live view whose summary lists only `FAIL`/`SKIP` results (or reports that
there is nothing to review).

```bash
$ make build                                # builds ./yups
$ make test                                 # go vet + lint + unit tests
$ make test-integration                     # integration tests, every distro (default)
$ make test-integration DISTRO=fedora       # one specific distro
$ make test-integration DISTRO=fedora,arch  # several distros
```

The integration scenarios run in parallel — every scenario gets its own
throwaway container, so the whole four-distro matrix costs little more than
a single distro. Concurrency is capped with `-parallel 8`; lower it if your
machine starts sweating. The pretty printer repaints the grouped view live;
set `REPAINT=never` for plain streaming output.

The integration target selects the systems under test with the make variable
`DISTRO`, which it translates into the `YUPS_TEST_DISTRO` environment
variable consumed by the tests themselves.

### Direct Go commands

If you prefer to skip make, these are the plain commands it wraps.

Build:

```bash
$ go build -o yups .
```

Static checks and unit tests (no Docker needed; unit tests live next to the
code in `internal/app` and use an in-memory fake of the operating system):

```bash
$ gofmt -l .
$ go vet ./...
$ golangci-lint run        # optional; skipped by `make test` when missing
$ go test -v ./...
```

Integration tests (`integration/`, build tag `integration`) build a linux
binary and run every scenario inside stock distro containers — root installs,
plain users without permissions, users in administrator groups, writable
`~/bin` directories, repeated installs, partial uninstalls, etc. Every distro
runs by default:

```bash
$ YUPS_TEST_DISTRO=all go test -tags integration -parallel 8 ./integration/ -count=1 -v
```

Select fewer systems with the `YUPS_TEST_DISTRO` environment variable
(comma separated names; unset means all):

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
  docker daemon; self-hosted runners expose theirs through `DOCKER_HOST`
  (see [configuration/forgejo-runner.md](configuration/forgejo-runner.md) —
  jobs never need privileged mode).
- **Tag push `vX.Y.Z`**: `release.yml` runs the full suite again and only
  then invokes [goreleaser](https://goreleaser.com): linux amd64/arm64
  tarballs, checksums and changelog. Publishing target depends on where the
  tag lands — GitHub (`.goreleaser.yaml`) or the self-hosted Forgejo
  instance (`.goreleaser-forgejo.yaml`) — using in both cases the automatic
  per-run token; no user-defined secrets are required.

What a self-hosted worker needs (labels, docker daemon, security model) is
documented in
[configuration/forgejo-runner.md](configuration/forgejo-runner.md); other
environment-specific knowledge lives next to it.



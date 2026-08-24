# yups

`yups` prints the `#_?` marker in ANSI 256-colour 214 and manages its own
installation.

```console
$ yups            # prints #_? in colour 214
$ yups --help     # help with the options available so far
$ yups --install
$ yups --uninstall
```

## Commands

- `--help`: shows the help.
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

## Development

Requirements: Go >= 1.25 and, for the integration tests, Docker.

```console
$ make build              # builds ./yups
$ make test               # gofmt + go vet + unit tests
$ make test-integration   # ubuntu:24.04 containers, users and permissions
```

The unit tests live next to the code (`internal/app`) and use an in-memory
fake of the operating system. The integration tests (`integration/`, build tag
`integration`) build a linux binary and run every scenario inside stock
distro containers — `ubuntu:24.04` (the default), `fedora:latest`,
`archlinux:latest` and `opensuse/leap:latest`: root installs, plain users
without permissions, users in the `sudo` group, writable `~/bin`
directories, repeated installs, partial uninstalls, etc.

Select the target systems with `YUPS_TEST_DISTRO` (comma separated names or
`all`; unset means ubuntu):

```console
$ make test-integration                      # only ubuntu (default)
$ make test-integration DISTRO=arch          # a single distro
$ make test-integration DISTRO=fedora,arch   # several distros
$ make test-integration-all                  # every supported distro
```


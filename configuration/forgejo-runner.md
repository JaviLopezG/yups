# Running the CI workflows on a Forgejo/Gitea worker

The repository ships its workflows in `.github/workflows/` (`ci.yml` on
every push/pull request, `release.yml` when a tag matching `vX.Y.Z` is
pushed). Both GitHub and Forgejo/Gitea execute them: Forgejo reads
`.github/workflows`, `.gitea/workflows` and `.forgejo/workflows` - but only
uses **one** directory per repository, in that order of precedence. That is
why this repository must **not** contain a `.forgejo/workflows` folder: its
existence would silently disable the shared workflows.

This document explains what a self-hosted Forgejo/Gitea runner needs to
execute them.

## What our workflows require

1. **A docker daemon reachable from the job container.** The integration
   tests start throwaway containers (`ubuntu`, `fedora`, `archlinux`,
   `opensuse/leap`). Jobs talk to it over `DOCKER_HOST`; they never need
   privileged mode themselves (see "Security model" below).
2. **The docker CLI inside the job container**, or an apt-based image where
   it can be installed (the workflows install it if missing).
3. **Go** (installed by `actions/setup-go` from `go.mod`).
4. For releases: nothing extra - goreleaser authenticates with the token
   each forge injects automatically.

## Runner labels

Jobs select their image with `runs-on: ubuntu-latest`, so your runner must
have at least this label, mapped to an image where the docker CLI is either
present or installable:

```yaml
labels:
  - ubuntu-latest:docker://ghcr.io/catthehacker/ubuntu:act-latest
```

The stock example labels shipped with forgejo-runner (`debian`, `node20`,
`docker`) are not used by this repository and can be replaced freely.

> [!NOTE]
> **Note for maintainers and forks:** if you run your own worker, create
> the labels your workflows refer to. The only one used today is `ubuntu-latest`;
> if a future workflow targets another system natively (e.g. `runs-on:
> fedora` to run tests inside a Fedora container instead of through docker),
> add a label for it - e.g. `- fedora:docker://docker.io/library/fedora:latest`
> - and keep this file updated.

## Docker daemon for the jobs

Preferred setup (used by the repository owner): a single admin-owned
Docker-in-Docker sidecar next to the runner, and the daemon handed to job
containers through the environment:

```yaml
services:
  docker-in-docker:
    image: docker:dind
    privileged: true          # dind needs it; this is fixed infrastructure,
    restart: unless-stopped   # not something workflows can request
    command: ['dockerd', '-H', 'tcp://0.0.0.0:2375', '--tls=false']

  runner:
    ...
    environment:
      DOCKER_HOST: tcp://docker-in-docker:2375   # for the runner process
```

and, crucially, also for the job containers, via the runner config:

```yaml
runner:
  envs:
    DOCKER_HOST: tcp://docker-in-docker:2375
```

Setting `DOCKER_HOST` only on the runner service is a common mistake: jobs
run in their own containers and would not inherit it.

With this layout keep `container.docker_host: "-"` in the runner config: it
prevents mounting `/var/run/docker.sock` into job containers; they talk TCP
to the dind sidecar through `DOCKER_HOST`. Do not switch it to `automount`
unless you move to socket sharing.

## Security model

Forgejo Actions performs remote code execution: anyone who can push code
that triggers workflows runs arbitrary commands under your runner's
privileges. Therefore:

- Do **not** grant job containers privileged mode and do **not** mount
  `/var/run/docker.sock` into them (that is why no workflow here declares a
  privileged `services:` block). If external users ever get access to the
  instance, that would hand them root on the runner host.
- The dind sidecar above *is* privileged, but it is fixed infrastructure
  defined by the administrator, identical for every job, and not something
  a workflow can influence.
- If a task ever genuinely requires privileged jobs, register a dedicated
  runner restricted to the specific repository/user instead of widening the
  shared one. See "Securing Forgejo Actions Deployments" in the Forgejo docs.

## Forgejo instance settings

- Actions must be enabled (default since v1.21).
- `DEFAULT_ACTIONS_URL` should point to `https://data.forgejo.org` (the
  default) so that `uses: actions/checkout@v4` and `actions/setup-go@v5`
  resolve.

## Tokens

No user-defined secrets are required. Every platform injects an automatic,
per-run, repository-scoped token as `secrets.GITHUB_TOKEN`:

| Platform | Automatic secret | goreleaser publishes to |
|---|---|---|
| GitHub | `secrets.GITHUB_TOKEN` | GitHub (`.goreleaser.yaml`) |
| Forgejo/Gitea | `secrets.GITHUB_TOKEN` (alias of its own run token) | the self-hosted instance (`.goreleaser-forgejo.yaml`) |

Each platform also injects its run token as a **real environment variable**
in the job container (`GITHUB_TOKEN` on GitHub; `GITHUB_TOKEN`,
`GITEA_TOKEN` or `FORGEJO_TOKEN`, depending on the runner, on
Forgejo/Gitea). Since goreleaser aborts with "multiple tokens found" when
more than one is defined at once, the release step receives the token
under a neutral name (`YUPS_RELEASE_TOKEN`) and re-exports it -- after
unsetting every other token variable -- to the single name its target
requires (`GITHUB_TOKEN` on GitHub, `GITEA_TOKEN` elsewhere). If you
prefer releases published under a specific account instead of the CI bot,
define your own token secret and swap the mapping in
`.github/workflows/release.yml`.

## Known issues

### goreleaser-action cannot be used in the shared release workflow

`uses: goreleaser/goreleaser-action@v6` breaks Forgejo runs: the runner
resolves short action references against `DEFAULT_ACTIONS_URL` (typically
`data.forgejo.org`) and that repository does not exist there. Because the
runner clones every remote action before running anything, the whole job
fails at preparation time with "unable to clone ... remote: Not found".

The release workflow therefore installs goreleaser directly from its GitHub
releases, verifying its `checksums.txt`, and runs it with `run:` steps.

### Forgejo: checkout fails with "path escapes from parent"

Symptom: every job fails at the first step (`actions/checkout`) with

```
copyDir: failed to copy content to container:
Error response from daemon: statat var/run/act/actions/<hash>: path escapes from parent
```

Cause: an incompatibility between the runner's `docker cp`-style copy of
action code into the job container and **recent Docker engines** (the
validation of destination paths was tightened; `docker:dind` floating on
`latest` currently ships one of them). It is unrelated to the contents of
the workflows.

Workaround (already applied in the owner's compose): pin the dind sidecar
to a known-good engine instead of `latest`:

```yaml
services:
  docker-in-docker:
    image: docker:27.5.1-dind   # instead of docker:dind
    ...
```

Keep an eye on forgejo-runner releases for a version that adapts to newer
daemons, then unpin.

### Registry flakiness on first-time pulls

Pulling several images concurrently (the parallel suite starts up to 8
scenarios at once) occasionally trips the registry/daemon into spurious
`page not found` errors or stalled downloads. The test harness already
handles this: `TestMain` pre-pulls every image sequentially with timeouts
and retries before any scenario starts, and scenarios run their containers
with `--pull=never`, so a network hiccup can fail fast but never hang the
suite.

## Other settings worth knowing

- `runner.capacity`: how many jobs run concurrently. Each integration job
  starts up to `-parallel 8` containers, so capacity 2-3 is plenty.
- `runner.timeout` (default 1h): the first run pulls four distro images;
  keep it above 30 minutes.
- `container.valid_volumes`: leave empty unless you really need bind mounts
  inside jobs; nothing in this repository requires them.

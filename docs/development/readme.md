# Development Guide

Developer documentation for building, testing, and releasing GRAIN.

## Table of Contents

- [Development Environment](#development-environment)
- [Running Locally](#running-locally)
- [Testing](#testing)
- [Local Release Builds](#local-release-builds)
- [Continuous Integration](#continuous-integration)
- [Cutting a Release](#cutting-a-release)
- [Version Information](#version-information)
- [Code Standards](#code-standards)

## Development Environment

### Prerequisites

- **Go 1.23+** — [download](https://go.dev/)
- **Docker** with Compose v2 — [download](https://www.docker.com/products/docker-desktop/)
- A C toolchain for building the embedded **nostrdb** library:
  - Linux: `build-essential autoconf automake libtool`
  - macOS: `brew install autoconf automake libtool` (Xcode CLT already provides gcc/make)
  - Windows: [MSYS2](https://www.msys2.org/) with `mingw-w64-x86_64-gcc`, `autoconf`, `automake`, `libtool`, `make`

Docker alone is sufficient if you only run tests or the local release build — those run inside containers that carry their own toolchain.

## Running Locally

```bash
git clone https://github.com/0ceanslim/grain.git
cd grain
go mod download

# Build the nostrdb C library (first time only, or after updating the submodule)
cd server/db/nostrdb && bash build.sh && cd ../../..

# Run the relay against a local data directory
go run . --data-dir ./data
```

Configuration files (`config.yml`, `whitelist.yml`, `blacklist.yml`, `relay_metadata.json`) are auto-created in the data directory from embedded examples on first run. The files hot-reload — edit and save, and the server restarts with the new settings.

## Testing

Integration tests live in `tests/integration/` and run against real grain instances brought up by `tests/docker/docker-compose.yml`. The suite spins up eight containers, each with its own scenario config (default, rate-limit, blacklist, whitelist, auth, timecheck, eventpurge, hot-reload), so tests can assert config-driven behavior without mutating a shared container mid-run.

```bash
cd tests
make test              # full cycle: up → run → down → collect logs
make test-interactive  # leave containers running between runs
make test-single TEST=TestRateLimit_GlobalEvent
make test-file FILE=blacklist_test.go
make test-stop         # tear down + collect logs into tests/logs/
```

The Makefile uses `docker compose` (v2) by default. Override with `DOCKER_COMPOSE=...` if you need the legacy v1 binary.

## Local Release Builds

`docs/development/` contains a Docker-based build system for producing a release binary on your local machine. It builds the **native platform only** — full multi-platform release builds run in CI, not locally.

```bash
cd docs/development
make release        # tests + native-platform binary, output in build/dist/
make dev-release    # skip tests (faster iteration)
make clean          # nuke build container + artifacts
make version        # print detected version
```

The binary is staged in `build/dist/grain-<os>-<arch>.tar.gz` with a SHA-256 checksum.

## Continuous Integration

Two workflows live in `.github/workflows/`:

### `ci.yml` — every push and PR

Runs the full integration suite on `ubuntu-latest` via `cd tests && make test`. On failure, uploads `tests/logs/` as a workflow artifact for post-mortem.

### `release.yml` — manually dispatched

Triggered from the Actions tab → **Release** → **Run workflow**. Does not run automatically on pushes or tags. See the next section for the dispatch flow.

## Cutting a Release

Releases are cut manually via workflow dispatch. The version is computed from the latest full-release tag, not from commit metadata, so you are always in control of what gets shipped.

### Inputs

| input | type | default | meaning |
|---|---|---|---|
| `bump` | `patch` / `minor` / `major` | `patch` | which component of the latest `vX.Y.Z` tag to bump |
| `prerelease` | boolean | `true` | emit an `-rcN` tag and mark as prerelease (default), or cut a full release |

### Flow

1. **compute** — strips any `-rc*` suffix from the latest full-release tag, applies the bump, and either appends the next free `-rcN` or uses the bare version. No tag is created yet.
2. **test** — runs the integration suite against the current `main`. If tests fail, nothing is tagged and the run stops.
3. **assets** — downloads HTMX and Hyperscript, builds Tailwind CSS, and rewrites `layout.html` to point at the bundled assets. The bundled `www/` is uploaded as a workflow artifact.
4. **build** — matrix across five native runners (Linux amd64/arm64, macOS arm64/amd64, Windows amd64). Each job downloads the bundled `www/`, compiles the nostrdb C library, and builds `grain` with build-time `ldflags` stamping `main.Version`, `main.BuildTime`, and `main.GitCommit`.
5. **release** — downloads all binary artifacts, generates `checksums.txt`, creates and pushes the tag, and publishes a GitHub release with `--generate-notes`. The release is marked prerelease when `prerelease=true`.

### Typical session

| Action | `bump` | `prerelease` | Produced tag | GH release type |
|---|---|---|---|---|
| First RC on the v0.5.0 line | `minor` | `true` | `v0.5.0-rc1` | prerelease |
| Found a bug, rebuilt, next RC | `minor` | `true` | `v0.5.0-rc2` | prerelease |
| Ship it | `minor` | `false` | `v0.5.0` | full release |
| Later bugfix RC | `patch` | `true` | `v0.5.1-rc1` | prerelease |
| Ship bugfix | `patch` | `false` | `v0.5.1` | full release |

The `bump` value is applied to the latest **full-release** tag, so RC iteration on the same version line always uses the same `bump`. Picking `major` or skipping straight to a different base starts a fresh `-rc1`.

### Safety rails

- Patch is the default and the no-input-needed path. Minor and major both require explicit dropdown selection.
- `prerelease` defaults to `true`. Cutting an actual release is always an explicit opt-out.
- Tags are created only after the full build matrix succeeds — a failed run leaves no dangling tag.
- The workflow refuses to overwrite an existing full-release tag.

## Version Information

The build-time version is stamped into the binary via `ldflags`:

```
-X main.Version=<tag>
-X main.BuildTime=<RFC3339>
-X main.GitCommit=<short sha>
```

`main.go` passes these into `server.SetVersionInfo`, which additionally forwards the version into `server/utils` so the NIP-11 relay info document (`/` with `Accept: application/nostr+json`) always reports the running binary's version — regardless of what the `version` field in `relay_metadata.json` says. That file's `version` field is effectively advisory.

```bash
grain --version
```

Shows version, build time, git commit, Go version, and platform.

## Code Standards

- Standard library first; reach for dependencies only when they earn their place.
- Structured logging — `log.Foo().Info("msg", "key", value)`, not `log.Printf`.
- Errors get wrapped with context, not swallowed.
- Integration tests live next to their scenario config in `tests/docker/configs/`.
- Pre-commit hook formats and tidies; see `.git/hooks/pre-commit` in the repo for the reference script.

## Resources

- **Nostr Protocol** — [NIPs Repository](https://github.com/nostr-protocol/nips)
- **Go Documentation** — [Go Language Docs](https://golang.org/doc/)
- **GRAIN Repository** — [GitHub](https://github.com/0ceanslim/grain)
- **Issue Tracker** — [GitHub Issues](https://github.com/0ceanslim/grain/issues)

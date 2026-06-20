# GRAIN documentation

GRAIN is a Nostr **relay** and an importable Go **client library**, with a web frontend that doubles as the reference client. Start here and follow the path for what you're doing.

## 🛠️ Running a relay (operators)

- **[installation.md](installation.md)** — install from a binary, source, or Docker; the data directory, first-run setup, and running GRAIN as a service (systemd / Windows NSSM / macOS launchd).
- **[configuration.md](configuration.md)** — the full configuration reference: server, rate limits, whitelist / blacklist, event purging, relay metadata, and the hot-reload system.
- **[docker/readme.md](docker/readme.md)** — Docker and docker-compose deployment.

## 📦 Building on the client library (developers)

- **[client-library-guide.md](client-library-guide.md)** — the importable `client/core` engine: the outbox model, automatic routing, streaming fetches, the pluggable Signer / Logger / RelayListStore seams, and runnable examples.
- **[api.md](api.md)** — the relay's HTTP API (OpenAPI / Swagger UI), served live by every relay at `/api/docs`.

## 🔧 Contributing

- **[development/readme.md](development/readme.md)** — development environment, the Docker build system, code standards, and the release process.
- **[../tests/readme.md](../tests/readme.md)** — the integration test suite and test-environment tooling.

---

The API reference is generated from the code — every running relay serves its own Swagger UI at `<relay>/api/docs` (raw spec at `/api/docs/openapi.json`).

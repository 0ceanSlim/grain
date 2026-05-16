# API documentation

GRAIN's HTTP API is documented via OpenAPI, generated from `// @` annotations on the handlers (see `swag` in the root `Makefile`) and served by the running relay.

- **Swagger UI:** `<relay>/api/docs`
- **OpenAPI spec (JSON):** `<relay>/api/docs/openapi.json`

The previous hand-written reference that lived here drifted from the code; the served spec is the source of truth. To regenerate the spec locally after editing handler annotations, run `make generate` from the repo root.

The NIP-86 relay management endpoint (`POST /` with `Content-Type: application/nostr+json+rpc`, gated by NIP-98 HTTP Auth) is grouped under the `nip86` tag in the UI.

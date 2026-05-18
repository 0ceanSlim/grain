# Root developer-facing Makefile. Test orchestration lives under
# tests/Makefile; this one is for codegen and a single-container
# dev relay you can poke at in a browser.
#
# Targets:
#   generate   — regenerate the OpenAPI spec from swag annotations.
#                Run this after adding or editing any `// @...` doc
#                comments on HTTP handlers; the build embeds the
#                generated JSON via //go:embed in main.go, so a
#                stale spec means a stale Swagger UI.
#   dev-up     — build from local source and start a single fresh
#                relay on :8181. No persistent volume, so every
#                `dev-down && dev-up` is a clean first-run (good
#                for exercising /setup). For the multi-scenario
#                integration fleet, use `cd tests && make test`.
#   dev-down   — stop and remove the dev container.
#   dev-logs   — tail the dev container's logs.
#   dev-rebuild — rebuild after code changes + restart, in one shot.

DEV_COMPOSE = docker compose -f docs/docker/docker-compose.dev.yml

.PHONY: generate dev-up dev-down dev-logs dev-rebuild

# `--parseDependency --parseInternal` makes swag follow imports into
# our own internal packages so response struct fields resolve. Output
# is restricted to JSON via `-ot json` — main.go embeds swagger.json
# only, and the otherwise-generated docs.go would land in a Go
# package directory and pull in swaggo/swag's API surface, which we
# don't actually consume at runtime.
generate:
	swag init --parseDependency --parseInternal -g main.go -o docs/openapi -ot json

# Bring up a single fresh dev relay built from the working tree.
# Healthcheck in the Dockerfile gates readiness so the echoed URL
# is actually serving by the time it's printed.
dev-up:
	@echo "Building grain from local source (first run takes 3-6 min)..."
	@$(DEV_COMPOSE) up -d --build
	@echo ""
	@echo "Dev relay starting at http://localhost:8181"
	@echo "First-run flow: visit / → red banner → click → /setup → claim."
	@echo "Logs: make dev-logs   Stop: make dev-down"

dev-down:
	@$(DEV_COMPOSE) down -v --remove-orphans

dev-logs:
	@$(DEV_COMPOSE) logs -f --tail=100

# Rebuild after editing source. Equivalent to `dev-down && dev-up`
# but a single command since the iteration loop is "edit → rebuild → reload".
dev-rebuild:
	@$(DEV_COMPOSE) down -v --remove-orphans
	@$(DEV_COMPOSE) up -d --build
	@echo "Dev relay restarted at http://localhost:8181"

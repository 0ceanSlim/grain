# Root developer-facing Makefile. Test orchestration lives under
# tests/Makefile; this one is for codegen and build helpers that
# touch the production binary's sources.
#
# Targets:
#   generate  — regenerate the OpenAPI spec from swag annotations.
#               Run this after adding or editing any `// @...` doc
#               comments on HTTP handlers; the build embeds the
#               generated JSON via //go:embed in main.go, so a stale
#               spec means a stale Swagger UI.

.PHONY: generate

# `--parseDependency --parseInternal` makes swag follow imports into
# our own internal packages so response struct fields resolve. Output
# lives in docs/openapi/, which is gitignored (the spec is a build
# artifact). Only swagger.json is embedded; the other two outputs
# (swagger.yaml, docs.go) are harmless leftovers from swag.
generate:
	swag init --parseDependency --parseInternal -g main.go -o docs/openapi

// Package main is the GRAIN relay entry point.
//
// The swag annotations on this file (@title, @version, etc.) populate
// the top-level info block of the generated OpenAPI document. The
// per-route annotations live on the individual handlers — search the
// codebase for `@Router` to find them.
//
// @title           GRAIN Relay API
// @version         0.7
// @description     HTTP API for grain — Nostr relay and client tooling. Includes read-only relay configuration endpoints, key utilities, event publishing/query helpers, and (gated behind NIP-98) the NIP-86 relay management endpoint.
// @license.name    MIT
// @license.url     https://github.com/0ceanslim/grain/blob/main/LICENSE
// @BasePath        /
// @securityDefinitions.apikey  NostrAuth
// @in                          header
// @name                        Authorization
// @description                 NIP-98 HTTP Auth. Value is `Nostr <base64-encoded-kind-27235-event>` signed by the relay owner.
package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/0ceanslim/grain/client"
	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/config/datadir"
	"github.com/0ceanslim/grain/server"
	"github.com/0ceanslim/grain/server/api/docs"
)

//go:embed docs/examples/*
var embeddedExamples embed.FS

//go:embed www/*
var embeddedWWW embed.FS

// embeddedOpenAPI is the swag-generated OpenAPI document. The file is
// produced by `make generate` (or the equivalent step in CI) and
// rebuilt on every build — it's intentionally gitignored. If the file
// is missing the build fails with "pattern docs/openapi/swagger.json:
// no matching files found", which is the right signal: a binary
// without the spec would serve a broken /api/docs page.
//
//go:embed docs/openapi/swagger.json
var embeddedOpenAPI []byte

// Version information - these will be set during build
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	// Set version information in server package
	server.SetVersionInfo(Version, BuildTime, GitCommit)

	// Handle command-line arguments first (--version, --help, --config-help)
	if server.HandleArgs() {
		return // Exit if a command-line flag was handled
	}

	// Resolve and prepare data directory
	dir := datadir.Resolve(parseDataDirFlag())
	if err := datadir.EnsureExists(dir); err != nil {
		fmt.Printf("Failed to create data directory %s: %v\n", dir, err)
		os.Exit(1)
	}
	config.SetDataDir(dir)

	// Set embedded filesystems
	config.SetEmbeddedExamples(embeddedExamples)
	client.SetEmbeddedWWW(embeddedWWW)
	docs.SetSpec(embeddedOpenAPI)

	// Handle --import flag: import events from JSONL file and exit
	if importFile := parseImportFlag(); importFile != "" {
		if err := server.ImportEvents(importFile); err != nil {
			fmt.Printf("Import failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Handle --delete / --delete-file flags: physically remove events and exit.
	// No signature check — shell access is the authorization boundary, same
	// trust model as --import.
	if ids := parseDeleteFlags(); len(ids) > 0 {
		if err := server.DeleteEvents(ids); err != nil {
			fmt.Printf("Delete failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Start the server
	if err := server.Run(); err != nil {
		fmt.Printf("Application failed: %v\n", err)
		os.Exit(1)
	}
}

// parseDataDirFlag extracts --data-dir value from os.Args, if present.
func parseDataDirFlag() string {
	for i, arg := range os.Args {
		if arg == "--data-dir" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

// parseImportFlag extracts --import value from os.Args, if present.
func parseImportFlag() string {
	for i, arg := range os.Args {
		if arg == "--import" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

// parseDeleteFlags collects ids from --delete <id> (may be repeated) and
// --delete-file <path> (one hex id per line, # comments). Returns nil if
// neither flag is present.
func parseDeleteFlags() []string {
	var ids []string
	for i, arg := range os.Args {
		switch arg {
		case "--delete":
			if i+1 < len(os.Args) {
				ids = append(ids, os.Args[i+1])
			}
		case "--delete-file":
			if i+1 < len(os.Args) {
				fileIDs, err := server.ReadDeleteFile(os.Args[i+1])
				if err != nil {
					fmt.Printf("Failed to read delete file: %v\n", err)
					os.Exit(1)
				}
				ids = append(ids, fileIDs...)
			}
		}
	}
	return ids
}

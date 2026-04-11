package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/0ceanslim/grain/client"
	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/config/datadir"
	"github.com/0ceanslim/grain/server"
)

//go:embed docs/examples/*
var embeddedExamples embed.FS

//go:embed www/*
var embeddedWWW embed.FS

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

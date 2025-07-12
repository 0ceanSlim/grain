package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server"
)

//go:embed docs/examples/*
var embeddedExamples embed.FS

// Version information - these will be set during build
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	// Set version information in server package
	server.SetVersionInfo(Version, BuildTime, GitCommit)

	// Handle command-line arguments first
	if server.HandleArgs() {
		return // Exit if a command-line flag was handled
	}

	// Set embedded examples for configuration system
	config.SetEmbeddedExamples(embeddedExamples)

	// Start the server
	if err := server.Run(); err != nil {
		fmt.Printf("Application failed: %v\n", err)
		os.Exit(1)
	}
}

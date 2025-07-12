package server

import (
	"fmt"
	"os"
	"runtime"
)

// Version information - these will be set during build via main package
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// SetVersionInfo allows main package to set version information
func SetVersionInfo(version, buildTime, gitCommit string) {
	Version = version
	BuildTime = buildTime
	GitCommit = gitCommit
}

// HandleArgs processes command-line arguments and returns true if the program should exit
func HandleArgs() bool {
	if len(os.Args) <= 1 {
		return false // No arguments, continue with normal startup
	}

	switch os.Args[1] {
	case "--version", "-v":
		printVersion()
		return true
	case "--help", "-h":
		printHelp()
		return true
	case "--config-help":
		printConfigHelp()
		return true
	default:
		// Check for unknown flags
		if len(os.Args[1]) > 0 && os.Args[1][0] == '-' {
			fmt.Printf("Unknown flag: %s\n", os.Args[1])
			fmt.Println("Use --help for usage information")
			os.Exit(1)
		}
	}

	return false // Continue with normal startup
}

// printVersion displays version information
func printVersion() {
	fmt.Printf("GRAIN %s\n", Version)
	fmt.Printf("Go Relay Architecture for Implementing Nostr\n")
	fmt.Printf("\nBuild Information:\n")
	fmt.Printf("  Version:    %s\n", Version)
	fmt.Printf("  Build Time: %s\n", BuildTime)
	fmt.Printf("  Git Commit: %s\n", GitCommit)
	fmt.Printf("  Go Version: %s\n", runtime.Version())
	fmt.Printf("  Platform:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

// printHelp displays usage information
func printHelp() {
	fmt.Printf("GRAIN - Go Relay Architecture for Implementing Nostr\n\n")
	fmt.Printf("Usage:\n")
	fmt.Printf("  grain [options]\n\n")
	fmt.Printf("Options:\n")
	fmt.Printf("  --version, -v        Show version information\n")
	fmt.Printf("  --help, -h           Show this help message\n")
	fmt.Printf("  --config-help        Show configuration file information\n\n")
	fmt.Printf("Environment Variables:\n")
	fmt.Printf("  MONGO_URI           Override MongoDB connection string\n")
	fmt.Printf("  SERVER_PORT         Override server port (just the number, e.g., 8080)\n")
	fmt.Printf("  LOG_LEVEL           Override log level (debug, info, warn, error)\n")
	fmt.Printf("  GRAIN_ENV           Set environment type (dev, prod, test)\n\n")
	fmt.Printf("Configuration:\n")
	fmt.Printf("  GRAIN uses YAML configuration files that are automatically created\n")
	fmt.Printf("  on first run if they don't exist:\n")
	fmt.Printf("    - config.yml        Main server configuration\n")
	fmt.Printf("    - whitelist.yml     User and content allowlists\n")
	fmt.Printf("    - blacklist.yml     User and content blocklists\n")
	fmt.Printf("    - relay_metadata.json  Public relay information (NIP-11)\n\n")
	fmt.Printf("For detailed configuration documentation, use --config-help\n")
	fmt.Printf("\nDocumentation:\n")
	fmt.Printf("  https://github.com/0ceanslim/grain\n")
}

// printConfigHelp displays configuration information
func printConfigHelp() {
	fmt.Printf("GRAIN Configuration Help\n\n")
	fmt.Printf("Configuration Files:\n")
	fmt.Printf("  config.yml          - Main server configuration\n")
	fmt.Printf("  whitelist.yml       - User and content allowlists\n")
	fmt.Printf("  blacklist.yml       - User and content blocklists\n")
	fmt.Printf("  relay_metadata.json - Public relay information (NIP-11)\n\n")
	fmt.Printf("File Creation:\n")
	fmt.Printf("  GRAIN automatically creates default configuration files from\n")
	fmt.Printf("  embedded examples on first run if they don't exist.\n\n")
	fmt.Printf("Hot Reload:\n")
	fmt.Printf("  Configuration files support hot-reload. Changes are detected\n")
	fmt.Printf("  automatically and the server restarts with new settings.\n\n")
	fmt.Printf("Environment Override:\n")
	fmt.Printf("  Key settings can be overridden with environment variables:\n")
	fmt.Printf("    MONGO_URI     - MongoDB connection string\n")
	fmt.Printf("    SERVER_PORT   - Server port number\n")
	fmt.Printf("    LOG_LEVEL     - Logging level\n")
	fmt.Printf("    GRAIN_ENV     - Environment type\n\n")
	fmt.Printf("Working Directory:\n")
	fmt.Printf("  GRAIN looks for configuration files in the current working directory.\n")
	fmt.Printf("  Make sure to run GRAIN from a directory where you want the config\n")
	fmt.Printf("  files to be created and stored.\n\n")
	fmt.Printf("Documentation:\n")
	fmt.Printf("  For comprehensive configuration documentation, see:\n")
	fmt.Printf("  https://github.com/0ceanslim/grain/blob/main/docs/configuration.md\n")
}

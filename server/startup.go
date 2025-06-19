package server

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0ceanslim/grain/client"
	"github.com/0ceanslim/grain/config"
	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/db/mongo"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
	"github.com/0ceanslim/grain/server/utils/userSync"

	"golang.org/x/net/websocket"
)

// Run starts the GRAIN relay server with configuration management and graceful shutdown
func Run() error {
	// Ensure required configuration files exist
	if err := ensureConfigFiles(); err != nil {
		return fmt.Errorf("failed to ensure config files: %w", err)
	}

	// Load initial configuration and setup logging
	cfg, err := config.LoadConfig("config.yml")
	if err != nil {
		return fmt.Errorf("failed to load initial config: %w", err)
	}
	log.InitializeLoggers(cfg)

	// Setup configuration file watchers and signal handlers
	restartChan := make(chan struct{}, 1) // Buffered channel to prevent blocking
	signalChan := make(chan os.Signal, 1)
	
	startConfigWatchers(restartChan)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	log.Main().Info("GRAIN relay server starting")

	// Main server lifecycle loop
	for {
		// Create shutdown channel for this instance
		shutdownChan := make(chan struct{})
		
		// Start server instance in goroutine
		go func() {
			runServerInstance(shutdownChan, restartChan, signalChan)
		}()

		// Wait for restart or shutdown signal
		select {
		case <-restartChan:
			log.Main().Info("Restarting server due to configuration change")
			close(shutdownChan) // Signal server instance to shutdown
			time.Sleep(3 * time.Second) // Brief pause before restart
			
			// Reset configurations to allow fresh loading
			resetConfigurations()
			continue
		case <-signalChan:
			log.Main().Info("Shutting down server gracefully")
			close(shutdownChan) // Signal server instance to shutdown
			time.Sleep(1 * time.Second) // Allow cleanup time
			return nil
		}
	}
}

// ensureConfigFiles creates default configuration files if they don't exist
func ensureConfigFiles() error {
	configFiles := map[string]string{
		"config.yml":           "www/static/examples/config.example.yml",
		"whitelist.yml":        "www/static/examples/whitelist.example.yml",
		"blacklist.yml":        "www/static/examples/blacklist.example.yml",
		"relay_metadata.json":  "www/static/examples/relay_metadata.example.json",
	}

	for target, example := range configFiles {
		utils.EnsureFileExists(target, example)
	}

	return nil
}

// startConfigWatchers starts file watchers for configuration files
func startConfigWatchers(restartChan chan<- struct{}) {
	watchFiles := []string{
		"config.yml",
		"whitelist.yml", 
		"blacklist.yml",
		"relay_metadata.json",
	}

	for _, file := range watchFiles {
		go config.WatchConfigFile(file, restartChan)
	}
}

// runServerInstance runs a single server instance until shutdown signal
func runServerInstance(shutdownChan <-chan struct{}, restartChan <-chan struct{}, signalChan <-chan os.Signal) {
	// Load all configuration files
	cfg, err := loadAllConfigs()
	if err != nil {
		log.Main().Error("Failed to load configurations", "error", err)
		return
	}

	// Initialize database connection
	dbClient, err := mongo.InitDB(cfg)
	if err != nil {
		log.Main().Error("Failed to initialize database", "error", err, "uri", cfg.MongoDB.URI)
		return
	}
	defer func() {
		if dbClient != nil {
			mongo.DisconnectDB(dbClient)
		}
	}()

	// Initialize all subsystems
	if err := initializeSubsystems(cfg); err != nil {
		log.Main().Error("Failed to initialize subsystems", "error", err)
		return
	}

	// Setup HTTP server
	httpServer := setupHTTPServer(cfg)
	defer func() {
		log.Main().Debug("Closing HTTP server")
		httpServer.Close()
	}()

	// Start background services
	startBackgroundServices(cfg)

	// Wait for shutdown, restart, or signal
	select {
	case <-shutdownChan:
		log.Main().Debug("Server instance received shutdown signal")
	case <-restartChan:
		log.Main().Debug("Server instance received restart signal") 
		// Don't reset configs here - let main loop handle it
	case <-signalChan:
		log.Main().Debug("Server instance received OS signal")
	}
}

// loadAllConfigs loads all configuration files with error handling
func loadAllConfigs() (*cfgType.ServerConfig, error) {
	cfg, err := config.LoadConfig("config.yml")
	if err != nil {
		return nil, fmt.Errorf("failed to load server config: %w", err)
	}

	if _, err := config.LoadWhitelistConfig("whitelist.yml"); err != nil {
		log.Main().Error("Failed to load whitelist config", "error", err, "file", "whitelist.yml")
	}

	if _, err := config.LoadBlacklistConfig("blacklist.yml"); err != nil {
		log.Main().Error("Failed to load blacklist config", "error", err, "file", "blacklist.yml")
	}

	return cfg, nil
}

// initializeSubsystems sets up all server subsystems
func initializeSubsystems(cfg *cfgType.ServerConfig) error {
	log.Main().Debug("Initializing server subsystems")

	// Re-initialize logger with current configuration
	log.InitializeLoggers(cfg)
	
	// Set resource limits
	config.SetResourceLimit(&cfg.ResourceLimits)
	
	// Configure rate and size limiting
	config.SetRateLimit(cfg)
	config.SetSizeLimit(cfg)
	
	// Clear any temporary bans from previous instance
	config.ClearTemporaryBans()

	// Load relay metadata
	if err := utils.LoadRelayMetadataJSON(); err != nil {
		log.Main().Error("Failed to load relay metadata", "error", err, "file", "relay_metadata.json")
	}

	// Initialize pubkey cache system
	config.InitializePubkeyCache()

	// TODO: make these configurable. Change the dfdefault config for the client
	// package to the same defaults I put in the example config. 
	// Initialize client package (includes session manager and core client)
	appRelays := []string{
		"wss://relay.damus.io",
		"wss://nos.lol", 
		"wss://relay.nostr.band",
	}
	
	if err := client.InitializeClient(appRelays); err != nil {
		log.Main().Error("Failed to initialize client package", "error", err)
		return fmt.Errorf("client initialization failed: %w", err)
	}

	log.Main().Info("Server subsystems initialized successfully")
	return nil
}

// setupHTTPServer creates and starts the HTTP server
func setupHTTPServer(cfg *cfgType.ServerConfig) *http.Server {
	mux := initClient()
	
	server := &http.Server{
		Addr:         cfg.Server.Port,
		Handler:      mux,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	go func() {
		fmt.Printf("Server is running on http://localhost%s\n", cfg.Server.Port)
		log.Main().Info("HTTP server started", 
			"address", cfg.Server.Port,
			"read_timeout", cfg.Server.ReadTimeout,
			"write_timeout", cfg.Server.WriteTimeout,
			"idle_timeout", cfg.Server.IdleTimeout)
			
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Main().Error("HTTP server error", "error", err)
		}
	}()

	return server
}

// startBackgroundServices starts all background services
func startBackgroundServices(cfg *cfgType.ServerConfig) {
	log.Main().Debug("Starting background services")

	// Start client statistics monitoring
	go InitStatsMonitoring()

	// Start event purging service
	go mongo.ScheduleEventPurgingOptimized(cfg)

	// Start periodic user sync service
	go userSync.StartPeriodicUserSync(cfg)

	log.Main().Info("Background services started")
}

// resetConfigurations resets all configuration state for restart
func resetConfigurations() {
	config.ResetConfig()
	config.ResetWhitelistConfig()
	config.ResetBlacklistConfig()
}

// initClient initializes the HTTP application routes and middleware
func initClient() http.Handler {
	mux := http.NewServeMux()
	
	// Main route handles WebSocket upgrades, NIP-11 relay info, and web interface
	mux.HandleFunc("/", initRoot)

	// Register API endpoints only (no view routes)
	client.RegisterEndpoints(mux)

	return mux // Return the mux as the HTTP handler
}

// wsServer handles WebSocket connections for the Nostr relay protocol
var wsServer = &websocket.Server{
	Handshake: func(config *websocket.Config, r *http.Request) error {
		// Skip origin check for maximum compatibility
		return nil
	},
	Handler: websocket.Handler(ClientHandler),
}

// initRoot handles the root endpoint, routing between WebSocket, NIP-11, and web interface
func initRoot(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Header.Get("Upgrade") == "websocket":
		// Handle Nostr WebSocket connections
		wsServer.ServeHTTP(w, r)
	case r.Header.Get("Accept") == "application/nostr+json":
		// Handle NIP-11 relay information requests
		utils.RelayInfoHandler(w, r)
	case r.URL.Path == "/":
		// Serve the main application template
		data := client.PageData{
			Title: "🌾 grain",
		}
		client.RenderTemplate(w, data, "app.html")
	default:
		// Serve static files from www directory
		fileServer := http.FileServer(http.Dir("www"))
		http.StripPrefix("/", fileServer).ServeHTTP(w, r)
	}
}
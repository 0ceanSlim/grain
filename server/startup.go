package server

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/0ceanslim/grain/client"
	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/config"
	cfgType "github.com/0ceanslim/grain/config/types"
	relay "github.com/0ceanslim/grain/server/api"
	"github.com/0ceanslim/grain/server/db/nostrdb"
	"github.com/0ceanslim/grain/server/handlers"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"

	"golang.org/x/net/websocket"
)

// Run starts the GRAIN relay server with configuration management and graceful shutdown
func Run() error {
	// Ensure required configuration files exist
	if err := ensureConfigFiles(); err != nil {
		return fmt.Errorf("failed to ensure config files: %w", err)
	}

	// Load initial configuration and setup logging
	cfg, err := config.LoadConfig(config.ConfigPath("config.yml"))
	if err != nil {
		return fmt.Errorf("failed to load initial config: %w", err)
	}
	log.InitializeLoggers(cfg)

	utils.AuthRequiredProvider = func() bool {
		if c := config.GetConfig(); c != nil {
			return c.Auth.Required
		}
		return false
	}

	// Setup configuration file watchers and signal handlers
	restartChan := make(chan struct{}, 1) // Buffered channel to prevent blocking
	signalChan := make(chan os.Signal, 1)

	startConfigWatchers(restartChan)
	// Wire the NIP-86 grain_reloadconfig method's restart trigger
	// into the same channel the file watcher uses. Non-blocking
	// send: if a restart is already queued we don't need to enqueue
	// another one.
	config.SetTriggerRestart(func() {
		select {
		case restartChan <- struct{}{}:
		default:
		}
	})

	// Provide server-side counters to NIP-86 grain_stats_overview.
	// Hook pattern (instead of an import) — server already imports
	// server/api for the dispatcher, so the reverse direction
	// would cycle. Atomic loads keep the call cheap.
	startTime := time.Now()
	relay.SetServerStatsHook(func() relay.ServerStats {
		return relay.ServerStats{
			ActiveConnections: currentConnections.Load(),
			TotalMessagesSent: totalMessagesSent,
			UptimeSeconds:     int64(time.Since(startTime).Seconds()),
			Version:           Version,
			BuildTime:         BuildTime,
			GitCommit:         GitCommit,
		}
	})

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	log.Startup().Info("GRAIN relay server starting")

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
			log.Startup().Info("Restarting server due to configuration change")
			close(shutdownChan)         // Signal server instance to shutdown
			time.Sleep(3 * time.Second) // Brief pause before restart

			// Reset configurations to allow fresh loading
			resetConfigurations()
			continue
		case <-signalChan:
			log.Startup().Info("Shutting down server gracefully")
			close(shutdownChan)         // Signal server instance to shutdown
			time.Sleep(1 * time.Second) // Allow cleanup time
			return nil
		}
	}
}

// ensureConfigFiles creates default configuration files if they don't exist
func ensureConfigFiles() error {
	return config.EnsureAllConfigFiles()
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
		go config.WatchConfigFile(config.ConfigPath(file), restartChan)
	}
}

// runServerInstance runs a single server instance until shutdown signal
func runServerInstance(shutdownChan <-chan struct{}, restartChan <-chan struct{}, signalChan <-chan os.Signal) {
	// Load all configuration files
	cfg, err := loadAllConfigs()
	if err != nil {
		log.Startup().Error("Failed to load configurations", "error", err)
		return
	}

	// Initialize nostrdb database
	dbPath := cfg.Database.Path
	if dbPath == "" {
		dbPath = "data"
	}
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(config.GetDataDir(), dbPath)
	}
	mapSizeMB := cfg.Database.MapSizeMB
	if mapSizeMB <= 0 {
		mapSizeMB = 4096 // 4GB default
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		log.Startup().Error("Failed to create database directory", "path", dbPath, "error", err)
		return
	}

	db, err := nostrdb.Open(dbPath, mapSizeMB, 4)
	dbAvailable := err == nil
	if err != nil {
		log.Startup().Error("Failed to open nostrdb", "path", dbPath, "error", err)
	} else {
		nostrdb.SetGlobalDB(db)
	}
	defer func() {
		if db != nil {
			db.Close()
		}
	}()

	// Initialize all subsystems
	if err := initializeSubsystems(cfg); err != nil {
		log.Startup().Error("Failed to initialize subsystems", "error", err)
		return
	}

	// Setup HTTP server
	httpServer := setupHTTPServer(cfg)
	defer func() {
		log.Startup().Debug("Closing HTTP server")
		httpServer.Close()
	}()

	// Start background services (pass DB availability status)
	startBackgroundServices(cfg, dbAvailable)

	log.Startup().Info("Server instance started successfully",
		"database_available", dbAvailable,
		"port", cfg.Server.Port)

	// Wait for shutdown, restart, or signal
	select {
	case <-shutdownChan:
		log.Startup().Debug("Server instance received shutdown signal")
	case <-restartChan:
		log.Startup().Debug("Server instance received restart signal")
		// Don't reset configs here - let main loop handle it
	case <-signalChan:
		log.Startup().Debug("Server instance received OS signal")
	}
}

// loadAllConfigs loads all configuration files with error handling
func loadAllConfigs() (*cfgType.ServerConfig, error) {
	cfg, err := config.LoadConfig(config.ConfigPath("config.yml"))
	if err != nil {
		return nil, fmt.Errorf("failed to load server config: %w", err)
	}

	if _, err := config.LoadWhitelistConfig(config.ConfigPath("whitelist.yml")); err != nil {
		log.Startup().Error("Failed to load whitelist config", "error", err, "file", "whitelist.yml")
	}

	if _, err := config.LoadBlacklistConfig(config.ConfigPath("blacklist.yml")); err != nil {
		log.Startup().Error("Failed to load blacklist config", "error", err, "file", "blacklist.yml")
	}

	return cfg, nil
}

// initializeSubsystems sets up all server subsystems
func initializeSubsystems(cfg *cfgType.ServerConfig) error {
	log.Startup().Debug("Initializing server subsystems")

	// Resolve log file path relative to data directory
	if !filepath.IsAbs(cfg.Logging.File) {
		cfg.Logging.File = filepath.Join(config.GetDataDir(), cfg.Logging.File)
	}

	// Re-initialize logger with current configuration
	log.InitializeLoggers(cfg)

	// Set resource limits
	config.SetResourceLimit(&cfg.ResourceLimits)

	// Configure rate and size limiting
	config.SetRateLimit(cfg)
	config.SetSizeLimit(cfg)

	// Clear any temporary bans from previous instance
	config.ClearTemporaryBans()

	// Load relay metadata and wire the admin write path: NIP-86's
	// changerelay* methods need to write back through the same
	// atomic-write + watcher-suppression pipeline the config files
	// use, but server/utils can't import config (would cycle), so
	// startup installs hooks into the loader.
	metadataPath := config.ConfigPath("relay_metadata.json")
	utils.SetRelayMetadataWritePath(metadataPath)
	utils.SetAdminWriteHooks(config.AtomicWriteFile, config.SuppressWatcherFor)
	if err := utils.LoadRelayMetadata(metadataPath); err != nil {
		log.Startup().Error("Failed to load relay metadata", "error", err, "file", "relay_metadata.json")
	}

	// First-run owner provisioning. GRAIN_OWNER_PUBKEY is the
	// declarative path — wins over whatever the JSON says and is
	// re-applied every startup (never written to disk). If the var
	// isn't set and the metadata has no owner, log a WARN so the
	// operator knows to visit /setup (the page itself is the primary
	// signal — banner on every page — but the log line helps anyone
	// watching `docker logs` too).
	if envHex, ok := resolveOwnerEnv(); ok {
		utils.OverrideRelayOwnerInMemory(envHex)
		log.Startup().Info("Relay owner set from GRAIN_OWNER_PUBKEY", "pubkey", envHex)
	} else if utils.IsRelayUnowned() {
		log.Startup().Warn("Relay has no owner configured — visit /setup to claim ownership")
	}

	// Wire up real-time event broadcasting to active subscribers
	handlers.OnEventStored = BroadcastEvent

	// Initialize client package with server configuration. This must happen
	// BEFORE InitializePubkeyCache because the initial blacklist refresh
	// fetches per-author NIP-65 mute lists via the core client; without it
	// the first refresh produces an empty grouped-mutelist cache and the
	// dashboard sees zero mutelist entries until the next scheduled refresh
	// (mutelist_cache_refresh_minutes later — up to 30 min by default).
	if err := client.InitializeClient(cfg); err != nil {
		log.Startup().Error("Failed to initialize client package", "error", err)
		return fmt.Errorf("client initialization failed: %w", err)
	}

	// Initialize pubkey cache system (after client init — see comment above)
	config.InitializePubkeyCache()

	log.Startup().Info("Server subsystems initialized successfully")
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
		log.Startup().Info("HTTP server started",
			"address", cfg.Server.Port,
			"read_timeout", cfg.Server.ReadTimeout,
			"write_timeout", cfg.Server.WriteTimeout,
			"idle_timeout", cfg.Server.IdleTimeout)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Startup().Error("HTTP server error", "error", err)
		}
	}()

	return server
}

// startBackgroundServices starts all background services
func startBackgroundServices(cfg *cfgType.ServerConfig, dbAvailable bool) {
	log.Startup().Debug("Starting background services",
		"database_available", dbAvailable)

	// Always start these services regardless of DB availability
	// Start client statistics monitoring
	go InitStatsMonitoring()

	// Load IP blocklist (admin + sidecar) and start the temp-ban
	// expiry sweeper. See #62. Both safe to call before the DB is up.
	config.LoadIPBlocklist(cfg.Blacklist)
	config.StartIPBlocklistSweeper()

	// Only start DB-dependent services if database is available
	if dbAvailable {
		// Start event purging service
		db := nostrdb.GetDB()
		if db != nil {
			go db.ScheduleEventPurging(cfg, func() []string {
				pubkeyCache := config.GetPubkeyCache()
				return pubkeyCache.GetWhitelistedPubkeys()
			})

			// NIP-40: rebuild the in-memory expiration heap from
			// stored events, then start the sweeper. Bootstrap runs
			// in a goroutine so a multi-million-event scan doesn't
			// stall startup; the sweeper joins the heap as
			// bootstrap populates it. ctx is intentionally
			// background — the sweeper runs for the lifetime of
			// this server instance and is unwound by process exit.
			go func() {
				if err := db.BootstrapExpirations(); err != nil {
					log.Startup().Error("NIP-40 expiration bootstrap failed", "error", err)
				}
			}()
			go db.RunExpirationSweeper(context.Background())
		}

		log.Startup().Info("All background services started")
	} else {
		log.Startup().Warn("Database-dependent services disabled",
			"disabled_services", "event_purging")
		log.Startup().Info("Non-database background services started")
	}
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

	// Register PWA routes
	client.RegisterPWARoutes(mux)

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
	case r.Method == http.MethodPost && r.Header.Get("Content-Type") == relay.NIP86ContentType:
		// NIP-86 relay management API. Matched ahead of the WebSocket
		// branch because a malformed Upgrade header on a POST request
		// would otherwise swallow it. Auth and owner checks happen
		// inside HandleNIP86 via RequireOwner.
		relay.HandleNIP86(w, r)
	case r.Header.Get("Upgrade") == "websocket":
		// Pre-upgrade per-IP connection rate limit (#61). Rejecting here
		// avoids paying the WS-upgrade cost for connection-storm
		// attempts. Disabled when limit is 0.
		cfg := config.GetConfig()
		if cfg != nil {
			if !EnforceConnectionRateLimit(w, r, cfg.Server.ConnectionRateLimitPerIP) {
				return
			}
		}
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
	case strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/login") || strings.HasPrefix(r.URL.Path, "/logout"):
		// Let API and auth endpoints fall through to be handled by registered endpoints
		http.NotFound(w, r)
	case strings.HasPrefix(r.URL.Path, "/views/") || strings.HasPrefix(r.URL.Path, "/static/") || strings.HasPrefix(r.URL.Path, "/style/"):
		// Serve actual static files from embedded FS (CSS, JS, view templates, etc.)
		subFS, _ := fs.Sub(client.GetEmbeddedWWW(), "www")
		fileServer := http.FileServer(http.FS(subFS))
		http.StripPrefix("/", fileServer).ServeHTTP(w, r)
	default:
		// All other routes: serve main app template for frontend routing
		data := client.PageData{
			Title: "🌾 grain",
		}
		client.RenderTemplate(w, data, "app.html")
	}

}

// resolveOwnerEnv reads GRAIN_OWNER_PUBKEY and returns the
// lowercased-hex pubkey + ok=true if the var is set to a usable
// value. Accepts hex (64 chars) or npub (bech32 — routed through
// tools.DecodeNpub). Malformed values log a WARN and return ok=false
// so the relay still serves traffic, just unowned.
//
// Lives in startup.go (not server/utils) because it depends on the
// client/core/tools package, which utils intentionally doesn't pull in.
func resolveOwnerEnv() (string, bool) {
	raw := strings.TrimSpace(os.Getenv("GRAIN_OWNER_PUBKEY"))
	if raw == "" {
		return "", false
	}
	if strings.HasPrefix(raw, "npub") {
		hex, err := tools.DecodeNpub(raw)
		if err != nil {
			log.Startup().Warn("GRAIN_OWNER_PUBKEY: npub decode failed, ignoring", "error", err)
			return "", false
		}
		return hex, true
	}
	lower := strings.ToLower(raw)
	if len(lower) != 64 || !isHex(lower) {
		log.Startup().Warn("GRAIN_OWNER_PUBKEY: must be 64-char hex or npub, ignoring",
			"length", len(lower))
		return "", false
	}
	return lower, true
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

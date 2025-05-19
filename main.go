package main

import (
	"fmt"
	"log/slog"
	"strings"

	//"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	configTypes "github.com/0ceanslim/grain/config/types"
	relay "github.com/0ceanslim/grain/server"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/db/mongo"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/userSync"
	"github.com/0ceanslim/grain/web/api"
	"github.com/0ceanslim/grain/web/handlers"
	"github.com/0ceanslim/grain/web/middleware"
	"github.com/0ceanslim/grain/web/routes"

	"golang.org/x/net/websocket"
)

var log *slog.Logger

func main() {
	// Initial logger setup
	cfg, err := config.LoadConfig("config.yml")
	if err != nil {
		fmt.Printf("Failed to load initial config: %v\n", err)
		os.Exit(1) // Exit if initial config load fails
	}
	utils.InitializeLogger(cfg) // Initialize logger with initial config
	log = utils.GetLogger("main")

	utils.EnsureFileExists("config.yml", "app/static/examples/config.example.yml")
	utils.EnsureFileExists("whitelist.yml", "app/static/examples/whitelist.example.yml")
	utils.EnsureFileExists("blacklist.yml", "app/static/examples/blacklist.example.yml")
	utils.EnsureFileExists("relay_metadata.json", "app/static/examples/relay_metadata.example.json")

	restartChan := make(chan struct{})
	go config.WatchConfigFile("config.yml", restartChan)
	go config.WatchConfigFile("whitelist.yml", restartChan)
	go config.WatchConfigFile("blacklist.yml", restartChan)
	go config.WatchConfigFile("relay_metadata.json", restartChan)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	for {
		wg.Add(1)

		cfg, err := config.LoadConfig("config.yml")
		if err != nil {
			log.Error("Failed to load config", "error", err, "file", "config.yml")
		}

		_, err = config.LoadWhitelistConfig("whitelist.yml")
		if err != nil {
			log.Error("Failed to load whitelist config", "error", err, "file", "whitelist.yml")
		}

		_, err = config.LoadBlacklistConfig("blacklist.yml")
		if err != nil {
			log.Error("Failed to load blacklist config", "error", err, "file", "blacklist.yml")
		}

		client, err := mongo.InitDB(cfg)
		if err != nil {
			log.Error("Failed to initialize database", "error", err, "uri", cfg.MongoDB.URI)
		}

		// Re-initialize logger with new configuration
		utils.InitializeLogger(cfg)
		log = utils.GetLogger("main") // Update global logger
		config.SetResourceLimit(&cfg.ResourceLimits)
		config.SetRateLimit(cfg)
		config.SetSizeLimit(cfg)
		config.ClearTemporaryBans()

		err = utils.LoadRelayMetadataJSON()
		if err != nil {
			log.Error("Failed to load relay metadata", "error", err, "file", "relay_metadata.json")
		}

		mux := initApp()
		server := initRelay(cfg, mux, &wg)

		// Start event purging in the background.
		go mongo.ScheduleEventPurging(cfg)

		// Start periodic user sync in a goroutine
		go userSync.StartPeriodicUserSync(cfg)

		// Monitor for server restart or shutdown signals.
		select {
		case <-restartChan:
			log.Info("Restarting server...")
			config.ResetConfig()
			config.ResetWhitelistConfig()
			config.ResetBlacklistConfig()
			server.Close()
			wg.Wait()
			time.Sleep(3 * time.Second)
		case <-signalChan:
			log.Info("Shutting down server...")
			server.Close()
			mongo.DisconnectDB(client)
			wg.Wait()
			return
		}
	}
}

func initApp() http.Handler {
	mux := http.NewServeMux()
	// Listen for ws messages or upgrade to http
	mux.HandleFunc("/", initRoot)

	// Init API Routes
	api.RegisterAPIRoutes(mux)

	// Handlers for Frontend
	mux.HandleFunc("/do-login", handlers.LoginHandler)
	mux.HandleFunc("/logout", handlers.LogoutHandler) // Logout process
	mux.HandleFunc("/profile", routes.ProfileHandler)
	// Serve static directory and favicon
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("app/static"))))
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "app/static/img/favicon.ico")
	})
	// Wrap with middleware
	return middleware.UserMiddleware(mux)
}

var wsServer = &websocket.Server{
	Handshake: func(config *websocket.Config, r *http.Request) error {
		// Skip origin check
		return nil
	},
	Handler: websocket.Handler(relay.ClientHandler),
}

func initRoot(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") == "websocket" {
		wsServer.ServeHTTP(w, r)
	} else if r.Header.Get("Accept") == "application/nostr+json" {
		utils.RelayInfoHandler(w, r)
	} else {
		routes.IndexHandler(w, r)
	}
}

func initRelay(config *configTypes.ServerConfig, handler http.Handler, wg *sync.WaitGroup) *http.Server {
	server := &http.Server{
		Addr:         config.Server.Port,
		Handler:      handler,
		ReadTimeout:  time.Duration(config.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(config.Server.IdleTimeout) * time.Second,
	}

	go func() {
		defer wg.Done() // Notify that the server goroutine is done
		fmt.Printf("Server is running on http://localhost%s\n", config.Server.Port)
		log.Info(fmt.Sprintf("Server is running on http://localhost%s", strings.TrimSpace(config.Server.Port)))
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Error(err.Error())
		}
	}()

	return server
}

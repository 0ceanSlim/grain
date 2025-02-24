package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	configTypes "grain/config/types"
	relay "grain/server"

	//"grain/app/src/api"
	"grain/config"
	"grain/server/db/mongo"
	"grain/server/utils"
	"grain/server/utils/userSync"
	"grain/web/handlers"
	"grain/web/middleware"
	"grain/web/routes"

	"golang.org/x/net/websocket"
)

func main() {
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
			log.Fatal("Error loading config: ", err)
		}

		_, err = config.LoadWhitelistConfig("whitelist.yml")
		if err != nil {
			log.Fatal("Error loading whitelist config: ", err)
		}

		_, err = config.LoadBlacklistConfig("blacklist.yml")
		if err != nil {
			log.Fatal("Error loading blacklist config: ", err)
		}

		client, err := mongo.InitDB(cfg)
		if err != nil {
			log.Fatal("Error initializing database: ", err)
		}

		config.SetResourceLimit(&cfg.ResourceLimits)

		// Start event purging in the background.
		go mongo.ScheduleEventPurging(cfg)

		config.SetRateLimit(cfg)
		config.SetSizeLimit(cfg)
		config.ClearTemporaryBans()

		err = utils.LoadRelayMetadataJSON()
		if err != nil {
			log.Fatal("Failed to load relay metadata: ", err)
		}

		mux := initApp()
		server := initRelay(cfg, mux, &wg)

		// Start periodic user sync in a goroutine
		go userSync.StartPeriodicUserSync(cfg)

		// Monitor for server restart or shutdown signals.
		select {
		case <-restartChan:
			log.Println("Restarting server...")
			config.ResetConfig()
			config.ResetWhitelistConfig()
			config.ResetBlacklistConfig()
			server.Close()
			wg.Wait()
			time.Sleep(3 * time.Second)
		case <-signalChan:
			log.Println("Shutting down server...")
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
	Handler: websocket.Handler(relay.WebSocketHandler),
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
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			fmt.Println("Error starting server:", err)
		}
	}()

	return server
}

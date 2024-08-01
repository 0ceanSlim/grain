package main

import (
	"fmt"
	"grain/app"
	"grain/config"
	configTypes "grain/config/types"
	relay "grain/server"
	"grain/server/db"
	"grain/server/nip"
	"grain/server/utils"
	"log"
	"net/http"
	"time"

	"golang.org/x/net/websocket"
)

func main() {
	utils.EnsureFileExists("config.yml", "app/static/examples/config.example.yml")
	utils.EnsureFileExists("relay_metadata.json", "app/static/examples/relay_metadata.example.json")

	cfg, err := config.LoadConfig("config.yml")
	if err != nil {
		log.Fatal("Error loading config: ", err)
	}

	client, err := db.InitDB(cfg)
	if err != nil {
		log.Fatal("Error initializing database: ", err)
	}
	defer db.DisconnectDB(client)

	config.SetupRateLimiter(cfg)
	config.SetupSizeLimiter(cfg)

	err = nip.LoadRelayMetadataJSON()
	if err != nil {
		log.Fatal("Failed to load relay metadata: ", err)
	}

	mux := setupRoutes()

	startServer(cfg, mux)
}

func setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", utils.WithCORS(ListenAndServe))
	mux.Handle("/static/", utils.WithCORSHandler(http.StripPrefix("/static/", http.FileServer(http.Dir("app/static")))))
	mux.HandleFunc("/favicon.ico", utils.WithCORS(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "app/static/img/favicon.ico")
	}))
	return mux
}



func startServer(config *configTypes.ServerConfig, mux *http.ServeMux) {
	server := &http.Server{
		Addr:         config.Server.Port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	fmt.Printf("Server is running on http://localhost%s\n", config.Server.Port)
	err := server.ListenAndServe()
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
var wsServer = &websocket.Server{
    Handshake: func(config *websocket.Config, r *http.Request) error {
        // Skip origin check
        return nil
    },
    Handler: websocket.Handler(relay.WebSocketHandler),
}

func ListenAndServe(w http.ResponseWriter, r *http.Request) {
    if r.Header.Get("Upgrade") == "websocket" {
        wsServer.ServeHTTP(w, r)
    } else if r.Header.Get("Accept") == "application/nostr+json" {
        nip.RelayInfoHandler(w, r)
    } else {
        app.RootHandler(w, r)
    }
}
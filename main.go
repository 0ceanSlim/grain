package main

import (
	"fmt"
	"grain/app"
	"grain/config"
	configTypes "grain/config/types"
	relay "grain/server"
	"grain/server/db"
	"grain/server/utils"

	"log"
	"net/http"

	"golang.org/x/net/websocket"
)

func main() {
	utils.EnsureFileExists("config.yml", "config/config.example.yml")
	utils.EnsureFileExists("app/relay_metadata.json", "app/relay_metadata.example.json")

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

	err = app.LoadRelayMetadataJSON()
	if err != nil {
		log.Fatal("Failed to load relay metadata: ", err)
	}

	mux := setupRoutes()

	startServer(cfg, mux)
}

func setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", ListenAndServe)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/static/img/favicon.ico")
	})
	return mux
}

func startServer(config *configTypes.ServerConfig, mux *http.ServeMux) {
	fmt.Printf("Server is running on http://localhost%s\n", config.Server.Port)
	err := http.ListenAndServe(config.Server.Port, mux)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

func ListenAndServe(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") == "websocket" {
		websocket.Handler(func(ws *websocket.Conn) {
			relay.WebSocketHandler(ws)
		}).ServeHTTP(w, r)
	} else if r.Header.Get("Accept") == "application/nostr+json" {
		app.RelayInfoHandler(w, r)
	} else {
		app.RootHandler(w, r)
	}
}

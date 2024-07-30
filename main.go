package main

import (
	"fmt"
	"log"
	"net/http"

	"grain/config"
	"grain/relay"
	"grain/relay/db"
	"grain/web"

	"golang.org/x/net/websocket"
)

func main() {
	cfg, err := config.LoadConfiguration()
	if err != nil {
		log.Fatal("Error loading config: ", err)
	}

	err = db.InitializeDatabase(cfg)
	if err != nil {
		log.Fatal("Error initializing database: ", err)
	}
	defer db.DisconnectDB()

	config.SetupRateLimiter(cfg)
	config.SetupSizeLimiter(cfg)

	err = web.LoadRelayMetadataJSON()
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

func startServer(config *config.Config, mux *http.ServeMux) {
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
		web.RelayInfoHandler(w, r)
	} else {
		web.RootHandler(w, r)
	}
}

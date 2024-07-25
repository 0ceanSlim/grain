package main

import (
	"fmt"
	"log"
	"net/http"

	"grain/relay"
	"grain/relay/db"
	"grain/relay/handlers"
	"grain/relay/utils"
	"grain/web"

	"golang.org/x/net/websocket"
	"golang.org/x/time/rate"
)

var rl *utils.RateLimiter

func main() {
	// Load configuration
	config, err := utils.LoadConfig("config.yml")
	if err != nil {
		log.Fatal("Error loading config: ", err)
	}

	// Initialize MongoDB client
	_, err = db.InitDB(config.MongoDB.URI, config.MongoDB.Database)
	if err != nil {
		log.Fatal("Error initializing database: ", err)
	}
	defer db.DisconnectDB()

	// Initialize rate limiter
	rl = utils.NewRateLimiter(rate.Limit(config.RateLimit.EventLimit), config.RateLimit.EventBurst, rate.Limit(config.RateLimit.WsLimit), config.RateLimit.WsBurst)
	for _, kindLimit := range config.RateLimit.KindLimits {
		rl.AddKindLimit(kindLimit.Kind, rate.Limit(kindLimit.Limit), kindLimit.Burst)
	}

	handlers.SetRateLimiter(rl)

	// Create a new ServeMux
	mux := http.NewServeMux()

	// Handle the root path
	mux.HandleFunc("/", ListenAndServe)

	// Serve static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Serve the favicon
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/static/img/favicon.ico")
	})

	// Start the Relay
	fmt.Printf("Server is running on http://localhost%s\n", config.Server.Port)
	err = http.ListenAndServe(config.Server.Port, mux)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

// Listener serves both WebSocket and HTML
func ListenAndServe(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") == "websocket" {
		websocket.Handler(func(ws *websocket.Conn) {
			if !rl.AllowWs() {
				ws.Close()
				return
			}
			relay.WebSocketHandler(ws)
		}).ServeHTTP(w, r)
	} else {
		web.RootHandler(w, r)
	}
}

package main

import (
	"fmt"
	"log"
	"net/http"

	"grain/relay"
	"grain/relay/db"
	"grain/relay/utils"
	"grain/web"

	"golang.org/x/net/websocket"
	"golang.org/x/time/rate"
)

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

	// Initialize RateLimiter
	rateLimiter := utils.NewRateLimiter(rate.Limit(config.RateLimit.EventLimit), config.RateLimit.EventBurst, rate.Limit(config.RateLimit.WsLimit), config.RateLimit.WsBurst)

	for _, kindLimit := range config.RateLimit.KindLimits {
		rateLimiter.AddKindLimit(kindLimit.Kind, rate.Limit(kindLimit.Limit), kindLimit.Burst)
	}

	rateLimiter.AddCategoryLimit("regular", rate.Limit(config.RateLimit.CategoryLimits.Regular.Limit), config.RateLimit.CategoryLimits.Regular.Burst)
	rateLimiter.AddCategoryLimit("replaceable", rate.Limit(config.RateLimit.CategoryLimits.Replaceable.Limit), config.RateLimit.CategoryLimits.Replaceable.Burst)
	rateLimiter.AddCategoryLimit("parameterized_replaceable", rate.Limit(config.RateLimit.CategoryLimits.ParameterizedReplaceable.Limit), config.RateLimit.CategoryLimits.ParameterizedReplaceable.Burst)
	rateLimiter.AddCategoryLimit("ephemeral", rate.Limit(config.RateLimit.CategoryLimits.Ephemeral.Limit), config.RateLimit.CategoryLimits.Ephemeral.Burst)

	// Create a new ServeMux
	mux := http.NewServeMux()

	// Handle the root path
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") == "websocket" {
			websocket.Handler(func(ws *websocket.Conn) {
				relay.WebSocketHandler(ws, rateLimiter)
			}).ServeHTTP(w, r)
		} else {
			web.RootHandler(w, r)
		}
	})

	// Serve static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Serve the favicon
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/static/img/favicon.ico")
	})

	// Start the server
	fmt.Printf("Server is running on http://localhost%s\n", config.Server.Port)
	err = http.ListenAndServe(config.Server.Port, mux)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

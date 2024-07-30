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
	"golang.org/x/time/rate"
)

func main() {
	cfg, err := loadConfiguration()
	if err != nil {
		log.Fatal("Error loading config: ", err)
	}

	err = initializeDatabase(cfg)
	if err != nil {
		log.Fatal("Error initializing database: ", err)
	}
	defer db.DisconnectDB()

	setupRateLimiter(cfg)
	setupSizeLimiter(cfg)

	err = loadRelayMetadata()
	if err != nil {
		log.Fatal("Failed to load relay metadata: ", err)
	}

	mux := setupRoutes()

	startServer(cfg, mux)
}

func loadConfiguration() (*config.Config, error) {
	return config.LoadConfig("config.yml")
}

func initializeDatabase(config *config.Config) error {
	_, err := db.InitDB(config.MongoDB.URI, config.MongoDB.Database)
	return err
}

func setupRateLimiter(cfg *config.Config) {
	rateLimiter := config.NewRateLimiter(
		rate.Limit(cfg.RateLimit.WsLimit),
		cfg.RateLimit.WsBurst,
		rate.Limit(cfg.RateLimit.EventLimit),
		cfg.RateLimit.EventBurst,
		rate.Limit(cfg.RateLimit.ReqLimit),
		cfg.RateLimit.ReqBurst,
	)

	for _, kindLimit := range cfg.RateLimit.KindLimits {
		rateLimiter.AddKindLimit(kindLimit.Kind, rate.Limit(kindLimit.Limit), kindLimit.Burst)
	}

	for category, categoryLimit := range cfg.RateLimit.CategoryLimits {
		rateLimiter.AddCategoryLimit(category, rate.Limit(categoryLimit.Limit), categoryLimit.Burst)
	}

	config.SetRateLimiter(rateLimiter)
}

func setupSizeLimiter(cfg *config.Config) {
	sizeLimiter := config.NewSizeLimiter(cfg.RateLimit.MaxEventSize)
	for _, kindSizeLimit := range cfg.RateLimit.KindSizeLimits {
		sizeLimiter.AddKindSizeLimit(kindSizeLimit.Kind, kindSizeLimit.MaxSize)
	}

	config.SetSizeLimiter(sizeLimiter)
}

func loadRelayMetadata() error {
	return web.LoadRelayMetadata("relay_metadata.json")
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

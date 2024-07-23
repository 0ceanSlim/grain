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

	// Run the WebSocket server in a goroutine
	go func() {
		fmt.Printf("WebSocket server is running on ws://localhost%s\n", config.Relay.Port)
		err := http.ListenAndServe(config.Relay.Port, websocket.Handler(relay.Listener))
		if err != nil {
			fmt.Println("Error starting WebSocket server:", err)
		}
	}()

	// Run the HTTP server for serving static files and home page
	mux := http.NewServeMux()
	mux.HandleFunc("/", web.RootHandler)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	fmt.Printf("Http server is running on http://localhost%s\n", config.Web.Port)
	err = http.ListenAndServe(config.Web.Port, mux)
	if err != nil {
		fmt.Println("Error starting web server:", err)
	}
}

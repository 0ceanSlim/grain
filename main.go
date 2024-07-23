package main

import (
	"fmt"
	"log"
	"net/http"

	"grain/server"
	"grain/server/db"
	"grain/server/utils"

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

	// Start WebSocket server
	http.Handle("/", websocket.Handler(server.Listener))
	fmt.Println("WebSocket server started on", config.Server.Address)
	err = http.ListenAndServe(config.Server.Address, nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

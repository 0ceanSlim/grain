package main

import (
	"fmt"
	"log"
	"net/http"

	"grain/db"
	"grain/events"
	"grain/requests"
	"grain/utils"

	"golang.org/x/net/websocket"
)

func main() {
	// Load configuration
	config, err := utils.LoadConfig("config.yml")
	if err != nil {
		log.Fatal("Error loading config: ", err)
	}

	// Initialize MongoDB client
	client, err := db.InitDB(config.MongoDB.URI, config.MongoDB.Database)
	if err != nil {
		log.Fatal("Error initializing database: ", err)
	}
	defer db.DisconnectDB(client)

	// Initialize collections
	events.InitCollections(client, config.Collections.EventKind0, config.Collections.EventKind1)

	// Set the MongoDB collections in the requests package
	requests.SetCollections(events.GetCollections())

	// Start WebSocket server
	http.Handle("/", websocket.Handler(requests.Handler))
	fmt.Println("WebSocket server started on", config.Server.Address)
	err = http.ListenAndServe(config.Server.Address, nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

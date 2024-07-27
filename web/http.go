package web

import (
	"context"
	"encoding/json"
	"grain/relay/db"
	relay "grain/relay/types"
	"html/template"
	"net/http"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type PageData struct {
	Title  string
	Theme  string
	Events []relay.Event
}

func RootHandler(w http.ResponseWriter, r *http.Request) {
	// Fetch the top ten most recent events
	client := db.GetClient()
	events, err := FetchTopTenRecentEvents(client)
	if err != nil {
		http.Error(w, "Unable to fetch events", http.StatusInternalServerError)
		return
	}

	data := PageData{
		Title:  "GRAIN Relay",
		Events: events,
	}

	RenderTemplate(w, data, "index.html")
}

func RelayInfoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Accept") != "application/nostr+json" {
		http.Error(w, "Unsupported Media Type", http.StatusUnsupportedMediaType)
		return
	}

	w.Header().Set("Content-Type", "application/nostr+json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	json.NewEncoder(w).Encode(relayMetadata)
}

// Define the base directories for views and templates
const (
	viewsDir     = "web/views/"
	templatesDir = "web/views/templates/"
)

// Define the common layout templates filenames
var templateFiles = []string{
	"layout.html",
	"header.html",
	"footer.html",
}

// Initialize the common templates with full paths
var layout = PrependDir(templatesDir, templateFiles)

func RenderTemplate(w http.ResponseWriter, data PageData, view string) {
	// Append the specific template for the route
	templates := append(layout, viewsDir+view)

	// Parse all templates
	tmpl, err := template.ParseFiles(templates...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Execute the "layout" template
	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Helper function to prepend a directory path to a list of filenames
func PrependDir(dir string, files []string) []string {
	var fullPaths []string
	for _, file := range files {
		fullPaths = append(fullPaths, dir+file)
	}
	return fullPaths
}

// FetchTopTenRecentEvents queries the database and returns the top ten most recent events.
func FetchTopTenRecentEvents(client *mongo.Client) ([]relay.Event, error) {
	var results []relay.Event

	collection := client.Database("grain").Collection("events")
	filter := bson.D{}
	opts := options.Find().SetSort(bson.D{{Key: "createdat", Value: -1}}).SetLimit(10)

	cursor, err := collection.Find(context.TODO(), filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	for cursor.Next(context.TODO()) {
		var event relay.Event
		if err := cursor.Decode(&event); err != nil {
			return nil, err
		}
		results = append(results, event)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

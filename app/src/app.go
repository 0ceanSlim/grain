package app

import (
	"grain/server/db"
	relay "grain/server/types"
	"html/template"
	"net/http"
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

// Define the base directories for views and templates
const (
	viewsDir     = "app/views/"
	templatesDir = "app/views/templates/"
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



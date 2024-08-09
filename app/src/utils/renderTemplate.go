package utils

import (
	app "grain/app/src/types"
	"html/template"
	"net/http"
)

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

func RenderTemplate(w http.ResponseWriter, data app.PageData, view string) {
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




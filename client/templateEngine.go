package client

import (
	"html/template"
	"net/http"
	"path/filepath"
)

type PageData struct {
	Title      string
	Theme      string
	CustomData map[string]interface{}
}

// Define the base directories for views and templates
const (
	viewsDir     = "www/views/"
	templatesDir = "www/views/templates/"
)

// Define the common layout templates filenames
var templateFiles = []string{
	"layout.html",
	"header.html",
	"footer.html",
}

// Initialize the common templates with full paths
var layout = PrependDir(templatesDir, templateFiles)

// RenderTemplate renders a template with the standard layout
func RenderTemplate(w http.ResponseWriter, data PageData, view string) {
	// Add global data if needed (e.g., client-wide constants or configurations)
	if data.CustomData == nil {
		data.CustomData = make(map[string]interface{})
	}

	data.CustomData["appName"] = "grain client" // Example global data

	viewTemplate := filepath.Join(viewsDir, view)
	componentPattern := filepath.Join(viewsDir, "components", "*.html")
	componentTemplates, err := filepath.Glob(componentPattern)
	if err != nil {
		http.Error(w, "Error loading component templates: "+err.Error(), http.StatusInternalServerError)
		return
	}

	templates := append(layout, viewTemplate)
	templates = append(templates, componentTemplates...)

	tmpl, err := template.New("").Funcs(template.FuncMap{}).ParseFiles(templates...)
	if err != nil {
		http.Error(w, "Error parsing templates: "+err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
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

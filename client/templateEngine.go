package client

import (
	"html/template"
	"io/fs"
	"net/http"
	"path"
)

type PageData struct {
	Title string
	Theme string
}

// Define the base directories for views and templates within the embedded FS
const (
	viewsDir     = "www/views"
	templatesDir = "www/views/templates"
)

// Define the common layout template filenames
var templateFiles = []string{
	"layout.html",
	"header.html",
	"footer.html",
}

// layoutPatterns returns the full embedded FS paths for layout templates
func layoutPatterns() []string {
	var paths []string
	for _, file := range templateFiles {
		paths = append(paths, path.Join(templatesDir, file))
	}
	return paths
}

// RenderTemplate renders a template with the standard layout using the embedded FS
func RenderTemplate(w http.ResponseWriter, data PageData, view string) {
	viewTemplate := path.Join(viewsDir, view)
	componentPattern := path.Join(viewsDir, "components", "*.html")

	componentTemplates, err := fs.Glob(wwwFS, componentPattern)
	if err != nil {
		http.Error(w, "Error loading component templates: "+err.Error(), http.StatusInternalServerError)
		return
	}

	patterns := append(layoutPatterns(), viewTemplate)
	patterns = append(patterns, componentTemplates...)

	tmpl, err := template.New("").Funcs(template.FuncMap{}).ParseFS(wwwFS, patterns...)
	if err != nil {
		http.Error(w, "Error parsing templates: "+err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
	}
}

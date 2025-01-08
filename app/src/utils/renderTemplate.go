package utils

import (
	//"goFrame/src/types"
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

var loginLayout = PrependDir(templatesDir, []string{"login-layout.html", "footer.html"})

func RenderTemplate(w http.ResponseWriter, data PageData, view string, useLoginLayout bool) {
	viewTemplate := filepath.Join(viewsDir, view)
	componentPattern := filepath.Join(viewsDir, "components", "*.html")
	componentTemplates, err := filepath.Glob(componentPattern)
	if err != nil {
		http.Error(w, "Error loading component templates: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var templates []string
	if useLoginLayout {
		templates = append(loginLayout, viewTemplate)
	} else {
		templates = append(layout, viewTemplate)
	}
	templates = append(templates, componentTemplates...)

	tmpl, err := template.New("").Funcs(template.FuncMap{}).ParseFiles(templates...)
	if err != nil {
		http.Error(w, "Error parsing templates: "+err.Error(), http.StatusInternalServerError)
		return
	}

	layoutName := "layout"
	if useLoginLayout {
		layoutName = "login-layout"
	}
	err = tmpl.ExecuteTemplate(w, layoutName, data)
	if err != nil {
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
	}
}

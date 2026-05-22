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

// assetVersion is stamped onto static asset URLs (?v=<version>) in the
// layout so a new release gets a fresh URL the CDN / browser / service
// worker can't stale-serve. Set once at startup from the build version.
var assetVersion = "dev"

// SetAssetVersion wires the build version in so {{assetVersion}} in the
// templates resolves to it. Called from main once the version is known.
func SetAssetVersion(v string) {
	if v != "" {
		assetVersion = v
	}
}

// baseTemplateFuncs are the helpers every layout-rendered page needs.
// All three parse sites (RenderTemplate, renderAdmin, renderSetup) must
// register these or ParseFS fails on the {{assetVersion}} reference in
// layout.html.
var baseTemplateFuncs = template.FuncMap{
	"assetVersion": func() string { return assetVersion },
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

	tmpl, err := template.New("").Funcs(baseTemplateFuncs).ParseFS(wwwFS, patterns...)
	if err != nil {
		http.Error(w, "Error parsing templates: "+err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
	}
}

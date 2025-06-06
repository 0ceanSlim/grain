package routes

import (
	"net/http"

	"github.com/0ceanslim/grain/client"
)

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		fileServer := http.FileServer(http.Dir("www"))
		http.StripPrefix("/", fileServer).ServeHTTP(w, r)
		return
	}

	data := client.PageData{
		Title:      "🏠",
	}

	client.RenderTemplate(w, data, "index.html")
}

package routes

import (
	"net/http"

	"github.com/0ceanslim/grain/client"
	"github.com/0ceanslim/grain/client/middleware"
)

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		fileServer := http.FileServer(http.Dir("www"))
		http.StripPrefix("/", fileServer).ServeHTTP(w, r)
		return
	}
	userData := middleware.GetUserFromContext(r.Context())

	data := client.PageData{
		Title:      "🏠",
		CustomData: userData,
	}

	client.RenderTemplate(w, data, "index.html")
}

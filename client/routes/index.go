package routes

import (
	"net/http"

	"github.com/0ceanslim/grain/client/middleware"
	"github.com/0ceanslim/grain/client/utils"
)

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		fileServer := http.FileServer(http.Dir("www"))
		http.StripPrefix("/", fileServer).ServeHTTP(w, r)
		return
	}
	userData := middleware.GetUserFromContext(r.Context())

	data := utils.PageData{
		Title:      "🏠",
		CustomData: userData,
	}

	utils.RenderTemplate(w, data, "index.html")
}

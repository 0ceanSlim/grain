package routes

import (
	"net/http"

	"github.com/0ceanslim/grain/client"
	"github.com/0ceanslim/grain/client/middleware"
)

func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	userData := middleware.GetUserFromContext(r.Context())

	data := client.PageData{
		Title:      "👤",
		CustomData: userData,
	}

	client.RenderTemplate(w, data, "profile.html")
}

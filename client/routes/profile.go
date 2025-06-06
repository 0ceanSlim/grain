package routes

import (
	"net/http"

	"github.com/0ceanslim/grain/client"
)

func ProfileHandler(w http.ResponseWriter, r *http.Request) {

	data := client.PageData{
		Title:      "👤",
	}

	client.RenderTemplate(w, data, "profile.html")
}

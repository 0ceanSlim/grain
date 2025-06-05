package routes

import (
	"net/http"

	"github.com/0ceanslim/grain/client/middleware"
	"github.com/0ceanslim/grain/client/utils"
)

func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	userData := middleware.GetUserFromContext(r.Context())

	data := utils.PageData{
		Title:      "👤",
		CustomData: userData,
	}

	utils.RenderTemplate(w, data, "profile.html")
}

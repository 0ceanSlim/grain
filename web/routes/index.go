package routes

import (
	"net/http"

	"github.com/0ceanslim/grain/web/middleware"
	"github.com/0ceanslim/grain/web/utils"
)

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	userData := middleware.GetUserFromContext(r.Context())

	data := utils.PageData{
		Title:      "GRAIN Dashboard",
		CustomData: userData,
	}

	utils.RenderTemplate(w, data, "index.html", false)
}

package routes

import (
	"grain/web/middleware"
	"grain/web/utils"
	"net/http"
)

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	userData := middleware.GetUserFromContext(r.Context())

	data := utils.PageData{
		Title:      "GRAIN Dashboard",
		CustomData: userData,
	}

	utils.RenderTemplate(w, data, "index.html", false)
}

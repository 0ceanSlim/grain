package routes

import (
	"grain/app/src/middleware"
	"grain/app/src/utils"

	"net/http"
)

func ImportEvents(w http.ResponseWriter, r *http.Request) {
	userData := middleware.GetUserFromContext(r.Context())

	data := utils.PageData{
		Title:      "Import Events",
		CustomData: userData,
	}

	utils.RenderTemplate(w, data, "importEvents.html", false)
}

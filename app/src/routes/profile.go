package routes

import (
	"grain/app/src/middleware"
	"grain/app/src/utils"
	"net/http"
)

func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	userData := middleware.GetUserFromContext(r.Context())

	data := utils.PageData{
		Title:      "nostr Profile",
		CustomData: userData,
	}

	utils.RenderTemplate(w, data, "profile.html", false)
}

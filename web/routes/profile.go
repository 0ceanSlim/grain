package routes

import (
	"net/http"

	"github.com/0ceanslim/grain/web/middleware"
	"github.com/0ceanslim/grain/web/utils"
)

func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	userData := middleware.GetUserFromContext(r.Context())

	data := utils.PageData{
		Title:      "nostr Profile",
		CustomData: userData,
	}

	utils.RenderTemplate(w, data, "profile.html", false)
}

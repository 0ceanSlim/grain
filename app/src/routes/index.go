package routes

import (
	"grain/app/src/handlers"
	"grain/app/src/utils"
	"net/http"
)

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := handlers.User.Get(r, "session-name")

	publicKey := session.Values["publicKey"]
	displayName := session.Values["displayName"]
	picture := session.Values["picture"]
	about := session.Values["about"]

	data := utils.PageData{
		Title: "GRAIN Dashboard",
		CustomData: map[string]interface{}{
			"publicKey":   publicKey,
			"displayName": displayName,
			"picture":     picture,
			"about":       about,
		},
	}

	utils.RenderTemplate(w, data, "index.html", false)
}

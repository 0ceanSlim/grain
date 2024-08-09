package routes

import (
	app "grain/app/src/types"
	"grain/app/src/utils"

	"net/http"
)

func ImportEvents(w http.ResponseWriter, r *http.Request) {
	data := app.PageData{
		Title: "Import Events",
	}

	// Call RenderTemplate with the specific template for this route
	utils.RenderTemplate(w, data, "importEvents.html")
}

package routes

import (
	app "grain/app/src/types"
	"grain/app/src/utils"
	"net/http"
)

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	
	data := app.PageData{
		Title:  "GRAIN Dashboard",

	}

	utils.RenderTemplate(w, data, "index.html")
}
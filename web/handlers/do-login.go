package handlers

import (
	"fmt"

	"github.com/0ceanslim/grain/config"

	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/sessions"
)

// TODO NEED TO MAKE THIS SECRET KEY DYNAMICALLY FOR EACH SESSION
var User = sessions.NewCookieStore([]byte("your-secret-key"))

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	publicKey := r.FormValue("publicKey")
	if publicKey == "" {
		http.Error(w, "Public key missing", http.StatusBadRequest)
		return
	}

	cfg, err := config.LoadConfig("config.yml")
	if err != nil {
		log.Printf("Failed to load config: %v\n", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	relays := []string{fmt.Sprintf("ws://localhost%s", cfg.Server.Port)}

	// Fetch user metadata
	userContent, err := FetchUserMetadata(publicKey, relays)
	if err != nil {
		log.Printf("Error fetching metadata for %s: %v\n", publicKey, err)
		http.Error(w, "Failed to fetch user metadata from relay", http.StatusInternalServerError)
		return
	}

	// **🚀 Fix the panic issue**
	if userContent == nil {
		log.Printf("User metadata for %s is nil\n", publicKey)
		http.Error(w, "Relay responded but no Kind 0 metadata found. You may not be whitelisted or your Kind 0 is not synced. Try using a client to write your Kind 0 to this relay.", http.StatusNotFound)
		return
	}

	// **🚀 Prevent accessing nil fields**
	if userContent.DisplayName == "" {
		log.Printf("Kind 0 metadata missing for %s\n", publicKey)
		http.Error(w, "Kind 0 metadata not found. Try writing your Kind 0 event to this relay.", http.StatusNotFound)
		return
	}

	// Save metadata to session
	session, _ := User.Get(r, "session-name")
	session.Values["publicKey"] = publicKey
	session.Values["displayName"] = userContent.DisplayName
	session.Values["picture"] = userContent.Picture
	session.Values["about"] = userContent.About

	if err := session.Save(r, w); err != nil {
		log.Printf("Failed to save session: %v\n", err)
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	// Respond with metadata
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"displayName": userContent.DisplayName,
		"picture":     userContent.Picture,
		"about":       userContent.About,
	})
}

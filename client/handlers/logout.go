package handlers

import (
	"log"
	"net/http"
)

// Assuming User is defined elsewhere, like in login.go
// var User = sessions.NewCookieStore([]byte("your-secret-key"))

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("LogoutHandler called")

	// Retrieve the session
	session, _ := User.Get(r, "session-name")

	// Clear session values
	session.Values = map[interface{}]interface{}{}
	session.Options.MaxAge = -1 // This will delete the session cookie

	// Save the session to commit the changes
	if err := session.Save(r, w); err != nil {
		log.Printf("Failed to clear session: %v\n", err)
		http.Error(w, "Failed to logout", http.StatusInternalServerError)
		return
	}

	log.Println("Session cleared successfully")

	// Redirect to the root ("/")
	http.Redirect(w, r, "/", http.StatusSeeOther)
	log.Println("Redirecting to / after logout")
}

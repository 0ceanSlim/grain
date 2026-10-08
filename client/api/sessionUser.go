package api

import (
	"net/http"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/session"
)

// sessionUser returns the logged-in user's per-user client context, writing
// the error response and returning nil when there isn't one.
func sessionUser(w http.ResponseWriter, r *http.Request) *core.UserContext {
	sess := session.SessionMgr.GetCurrentUser(r)
	if sess == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return nil
	}
	uc := connection.UserFor(sess.PublicKey)
	if uc == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return nil
	}
	return uc
}

// optionalSessionUser returns the logged-in user's per-user client context, or
// nil for an anonymous request. Requests made through it run as that user.
func optionalSessionUser(r *http.Request) *core.UserContext {
	sess := session.SessionMgr.GetCurrentUser(r)
	if sess == nil {
		return nil
	}
	return connection.UserFor(sess.PublicKey)
}

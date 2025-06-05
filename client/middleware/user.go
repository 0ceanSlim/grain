package middleware

import (
	"context"
	"net/http"

	"github.com/0ceanslim/grain/client/auth"
)

type contextKey string

const UserContextKey contextKey = "user"

func UserMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := auth.User.Get(r, "session-name")

		userData := map[string]interface{}{
			"publicKey":   session.Values["publicKey"],
			"displayName": session.Values["displayName"],
			"picture":     session.Values["picture"],
			"about":       session.Values["about"],
		}

		// Store user data in context
		ctx := context.WithValue(r.Context(), UserContextKey, userData)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUserFromContext(ctx context.Context) map[string]interface{} {
	userData, ok := ctx.Value(UserContextKey).(map[string]interface{})
	if !ok {
		return nil
	}
	return userData
}

package utils

import (
	"net/http"
	"strings"
)

func GetClientIP(r *http.Request) string {
	// Try X-Forwarded-For header first
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			// No per-request debug line here: GetClientIP runs on every HTTP
			// request and the line added ~26k records/hour with nothing
			// actionable (RC diagnosis §C).
			return strings.TrimSpace(ips[0])
		}
	}

	// Fall back to RemoteAddr
	remoteAddr := r.RemoteAddr
	var clientIP string

	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		clientIP = remoteAddr[:idx]
	} else {
		clientIP = remoteAddr
	}

	return clientIP
}

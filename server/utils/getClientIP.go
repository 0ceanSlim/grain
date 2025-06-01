package utils

import (
	"net/http"
	"strings"

	"github.com/0ceanslim/grain/server/utils/log"
)

func GetClientIP(r *http.Request) string {
    // Try X-Forwarded-For header first
    xff := r.Header.Get("X-Forwarded-For")
    if xff != "" {
        ips := strings.Split(xff, ",")
        if len(ips) > 0 {
            clientIP := strings.TrimSpace(ips[0])
            log.Util().Debug("Client IP determined from X-Forwarded-For", 
                "ip", clientIP, 
                "original_header", xff)
            return clientIP
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
    
    log.Util().Debug("Client IP determined from RemoteAddr", 
        "ip", clientIP, 
        "remote_addr", remoteAddr)
    return clientIP
}
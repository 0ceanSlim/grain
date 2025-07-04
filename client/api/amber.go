package api

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/server/utils/log"
)

// AmberCallbackData represents the callback data from Amber
type AmberCallbackData struct {
	Event     string `json:"event"`
	PublicKey string `json:"public_key"`
	Error     string `json:"error,omitempty"`
}

// pubKeyRegex validates Nostr public keys (64 hex characters)
var pubKeyRegex = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

// HandleAmberCallback processes callbacks from Amber app
func HandleAmberCallback(w http.ResponseWriter, r *http.Request) {
	log.Auth().Debug("amber callback received",
		"method", r.Method,
		"url", r.URL.String(),
		"user_agent", r.Header.Get("User-Agent"))

	// Parse query parameters
	eventParam := r.URL.Query().Get("event")
	if eventParam == "" {
		log.ClientAPI().Error("amber callback missing event parameter")
		renderAmberError(w, "Missing event data from Amber")
		return
	}

	// Extract public key from event parameter
	publicKey, err := extractPublicKeyFromAmber(eventParam)
	if err != nil {
		log.ClientAPI().Error("failed to extract public key from amber response",
			"event", eventParam,
			"error", err)
		renderAmberError(w, "Invalid response from Amber: "+err.Error())
		return
	}

	log.ClientAPI().Info("amber callback processed successfully",
		"public_key", publicKey[:16]+"...")

	// Create session
	sessionRequest := session.SessionInitRequest{
		PublicKey:     publicKey,
		RequestedMode: session.WriteMode,
		SigningMethod: session.AmberSigning,
	}

	_, err = session.CreateUserSession(w, sessionRequest)
	if err != nil {
		log.ClientAPI().Error("failed to create amber session",
			"public_key", publicKey[:16]+"...",
			"error", err)
		renderAmberError(w, "Failed to create session")
		return
	}

	log.ClientAPI().Info("amber session created successfully",
		"public_key", publicKey[:16]+"...")

	// Set session cookie (already handled by CreateUserSession)

	// Render success page with auto-redirect
	renderAmberSuccess(w, publicKey)
}

// extractPublicKeyFromAmber extracts and validates public key from Amber response
func extractPublicKeyFromAmber(eventParam string) (string, error) {
	// Handle compressed response (starts with "Signer1")
	if strings.HasPrefix(eventParam, "Signer1") {
		return "", fmt.Errorf("compressed Amber responses not supported")
	}

	// For get_public_key, event parameter should contain the public key directly
	publicKey := strings.TrimSpace(eventParam)

	// Validate public key format
	if !pubKeyRegex.MatchString(publicKey) {
		return "", fmt.Errorf("invalid public key format from Amber")
	}

	return publicKey, nil
}

// renderAmberSuccess renders the success page for Amber callback
func renderAmberSuccess(w http.ResponseWriter, publicKey string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	// Render success page that stores result and communicates back to main window
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Amber Login Success</title>
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, sans-serif; 
            margin: 0; 
            padding: 20px; 
            background: #1a1a1a; 
            color: white; 
            text-align: center; 
        }
        .success { color: #4ade80; margin: 20px 0; }
        .loading { color: #64748b; }
    </style>
</head>
<body>
    <div class="success">
        <h2>✅ Amber Login Successful!</h2>
        <p>Connected successfully. Returning to application...</p>
    </div>
    <div class="loading">
        <p>Please wait...</p>
    </div>
    
    <script>
        // Store the result in localStorage for the main window to pick up
        const amberResult = {
            success: true,
            publicKey: '` + publicKey + `',
            timestamp: Date.now()
        };
        
        try {
            localStorage.setItem('amber_callback_result', JSON.stringify(amberResult));
            console.log('Stored Amber success result in localStorage');
        } catch (error) {
            console.error('Failed to store Amber result:', error);
        }
        
        // Try to communicate with parent window if available
        if (window.opener && !window.opener.closed) {
            try {
                window.opener.postMessage({
                    type: 'amber_success',
                    publicKey: '` + publicKey + `'
                }, window.location.origin);
                console.log('Sent success message to opener window');
            } catch (error) {
                console.error('Failed to send message to opener:', error);
            }
        }
        
        // Return to main page after short delay
        setTimeout(() => {
            try {
                if (window.opener && !window.opener.closed) {
                    // Close popup and return to opener
                    window.close();
                } else {
                    // Redirect to main page with success indicator
                    window.location.href = '/?amber_login=success';
                }
            } catch (error) {
                console.error('Failed to navigate:', error);
                // Fallback: just go to main page
                window.location.href = '/';
            }
        }, 1500);
    </script>
</body>
</html>`

	w.Write([]byte(html))
}

// renderAmberError renders the error page for Amber callback
func renderAmberError(w http.ResponseWriter, errorMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)

	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Amber Login Error</title>
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, sans-serif; 
            margin: 0; 
            padding: 20px; 
            background: #1a1a1a; 
            color: white; 
            text-align: center; 
        }
        .error { color: #ef4444; margin: 20px 0; }
        .retry { margin-top: 20px; }
        .retry a { color: #3b82f6; text-decoration: none; }
    </style>
</head>
<body>
    <div class="error">
        <h2>❌ Amber Login Failed</h2>
        <p>` + errorMsg + `</p>
    </div>
    <div class="retry">
        <a href="/">← Return to login</a>
    </div>
    
    <script>
        if (window.opener) {
            // We're in a popup, send error to parent
            window.opener.postMessage({
                type: 'amber_error',
                error: '` + errorMsg + `'
            }, window.location.origin);
            setTimeout(() => window.close(), 3000);
        }
    </script>
</body>
</html>`

	w.Write([]byte(html))
}
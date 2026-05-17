package api

import (
	"net/http"

	"github.com/0ceanslim/grain/server/utils/log"
)

// HandleAmberCallback is the redirect target for NIP-55 (Amber).
//
// Mill's <nostr-signer> opens `nostrsigner://...?callbackUrl=...`;
// Amber bounces back with `?event=<pubkey-or-signed-event>&error=`
// on that URL. The page rendered here loads the bundled mill script
// so its boot-time hook (see nip55.js around line 17) captures the
// query params, stores them under `mill:amber:result` in
// localStorage, and notifies the opener window via postMessage.
// `MILL.deliverAmberCallback({autoClose:true})` finishes the
// handshake and closes the popup.
//
// The server intentionally does NOT create the session here. The
// pubkey funnels back into mill in the opener window; the bridge
// (www/static/js/mill-bridge.js) is what POSTs /api/v1/auth/login
// with the resolved method and pubkey. Keeping session creation in
// one path (the bridge) means there's a single contract with the
// session manager regardless of signing method.
//
// @Summary      Amber NIP-55 callback
// @Description  Landing page hit by the Amber signer app. Loads mill, which forwards the result to the opener window and closes the popup. Session creation lives in the front-end mill bridge, not here.
// @Tags         client-auth
// @Param        event  query     string  false  "Amber response: pubkey hex (get_public_key) or signed event JSON"
// @Param        error  query     string  false  "Amber error message, if user rejected"
// @Produce      html
// @Success      200  {string}  string  "HTML bridge page"
// @Router       /api/v1/auth/amber-callback [get]
func HandleAmberCallback(w http.ResponseWriter, r *http.Request) {
	log.Auth().Debug("amber callback received",
		"method", r.Method,
		"url", r.URL.String(),
		"user_agent", r.Header.Get("User-Agent"))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(amberBridgeHTML))
}

// amberBridgeHTML is the static page Amber lands on. Mill's script
// snapshots the URL query params into localStorage on load (so even
// if the page reloads the result is recoverable) and
// MILL.deliverAmberCallback wraps that with autoClose so the popup
// closes itself after handing off.
//
// Styled inline (no token system) because:
//   - the popup is short-lived
//   - it can't reach into the parent's CSS variables (different window)
//   - keeping it tiny means no extra fetch, no layout-shift on a slow
//     network — the user just sees a brief "returning to grain" before
//     the popup closes
const amberBridgeHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Returning to grain</title>
  <style>
    body { font-family: system-ui, -apple-system, sans-serif; margin: 0; padding: 24px; background: #0d0f0c; color: #e8f0e4; text-align: center; }
    .msg { margin-top: 40px; }
    .msg h2 { font-weight: 600; }
    .msg p { color: #8ea882; }
  </style>
</head>
<body>
  <div class="msg">
    <h2>🌾 returning to grain&hellip;</h2>
    <p>You can close this window if it doesn't close on its own.</p>
  </div>
  <script src="/static/mill/mill.umd.min.js"></script>
  <script>
    // Mill's nip55.js boot hook has already snapshot the query
    // params to localStorage and posted to the opener by this point.
    // deliverAmberCallback re-emits and arranges the close.
    if (window.MILL && typeof window.MILL.deliverAmberCallback === "function") {
      window.MILL.deliverAmberCallback({ autoClose: true });
    } else {
      setTimeout(function () { window.close(); }, 500);
    }
  </script>
</body>
</html>`

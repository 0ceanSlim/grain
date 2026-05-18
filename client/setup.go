// First-run owner provisioning. /setup is the click-through claim
// page for operators who haven't set GRAIN_OWNER_PUBKEY in their env.
//
// Security model (deliberately tiny): first POST to /setup while the
// relay is unowned wins. A signed NIP-98 envelope would add nothing —
// before ownership exists the relay has no notion of "which pubkeys
// may claim." The cost of losing the race is "redeploy a fresh
// relay" so the loud red banner + the post-claim "claimed by <npub>"
// page are the operator's safety net. See plans/floating-imagining-tome.md.
package client

import (
	"encoding/json"
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// SetupPageData feeds setup.html (claim form) and setup-claimed.html
// (already-owned info panel). OwnerNpub is empty in the unowned
// state; populated with the bech32 form when rendering the claimed
// panel so the operator can eyeball whether the claimant is them.
type SetupPageData struct {
	Title     string
	OwnerHex  string
	OwnerNpub string
}

// HandleSetup renders the first-run claim page (GET) or processes a
// claim attempt (POST). See package doc for the threat-model rationale.
func HandleSetup(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleSetupGet(w, r)
	case http.MethodPost:
		handleSetupPost(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleSetupGet(w http.ResponseWriter, r *http.Request) {
	data := SetupPageData{Title: "🌾 grain — setup"}
	if utils.IsRelayUnowned() {
		renderSetup(w, data, "setup.html")
		return
	}
	data.OwnerHex = utils.GetRelayOwnerPubkey()
	if npub, err := tools.EncodePubkey(data.OwnerHex); err == nil {
		data.OwnerNpub = npub
	}
	renderSetup(w, data, "setup-claimed.html")
}

// setupClaimRequest is what setup.js POSTs after mill returns a
// signer. The pubkey is the signer's claimed identity. We don't
// verify a signature — see package doc.
type setupClaimRequest struct {
	Pubkey string `json:"pubkey"`
}

func handleSetupPost(w http.ResponseWriter, r *http.Request) {
	var req setupClaimRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	hexPub := strings.ToLower(strings.TrimSpace(req.Pubkey))
	if len(hexPub) != 64 || !isLowerHex(hexPub) {
		http.Error(w, "pubkey must be 64-char hex", http.StatusBadRequest)
		return
	}

	if err := utils.SetRelayOwner(hexPub); err != nil {
		if errors.Is(err, utils.ErrOwnerAlreadySet) {
			// Race lost (or operator double-clicked). Return the
			// current claimant so the JS can swap to the
			// already-claimed panel without a second round-trip.
			current := utils.GetRelayOwnerPubkey()
			npub, _ := tools.EncodePubkey(current)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":      "already claimed",
				"owner_hex":  current,
				"owner_npub": npub,
			})
			return
		}
		log.ClientAPI().Error("SetRelayOwner failed", "error", err)
		http.Error(w, "claim failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.ClientAPI().Info("Relay ownership claimed via /setup",
		"client_ip", utils.GetClientIP(r),
		"pubkey", hexPub)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"ok":       "true",
		"redirect": "/admin",
	})
}

func isLowerHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

// renderSetup parses the named view template against the shared
// layout. Mirrors renderAdmin in admin.go — separate from
// RenderTemplate so we can pass a typed SetupPageData (PageData is
// too narrow for OwnerHex/OwnerNpub).
func renderSetup(w http.ResponseWriter, data SetupPageData, view string) {
	viewTemplate := path.Join(viewsDir, view)
	componentPattern := path.Join(viewsDir, "components", "*.html")
	componentTemplates, err := fs.Glob(wwwFS, componentPattern)
	if err != nil {
		http.Error(w, "Error loading component templates: "+err.Error(), http.StatusInternalServerError)
		return
	}
	patterns := append(layoutPatterns(), viewTemplate)
	patterns = append(patterns, componentTemplates...)
	tmpl, err := template.New("").Funcs(template.FuncMap{}).ParseFS(wwwFS, patterns...)
	if err != nil {
		http.Error(w, "Error parsing templates: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
	}
}

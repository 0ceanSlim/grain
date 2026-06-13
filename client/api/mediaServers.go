package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/server/utils/log"
)

// MediaServerEntry is a resolved media server annotated with grain's static
// capability metadata (when the server is one grain knows).
type MediaServerEntry struct {
	URL       string `json:"url"`
	Kind      string `json:"kind"`                // "blossom" | "nip96"
	Primary   bool   `json:"primary"`             // first in its list (the upload default)
	Name      string `json:"name,omitempty"`      // display name, when known
	Cost      string `json:"cost,omitempty"`      // "free" | "paid" | "" (unknown)
	Retention string `json:"retention,omitempty"` // "permanent" | "ephemeral" | "" (unknown)
	Mirror    bool   `json:"mirror"`              // accepts BUD-04 /mirror
	Note      string `json:"note,omitempty"`
	CTA       string `json:"cta,omitempty"`
	Known     bool   `json:"known"` // grain has capability metadata for this URL
}

// UserMediaServersResponse is a user's resolved Blossom + NIP-96 media-server
// lists. HasAny drives the "set up media servers" prompt on upload.
type UserMediaServersResponse struct {
	Pubkey  string             `json:"pubkey"`
	HasAny  bool               `json:"hasAny"`
	Blossom []MediaServerEntry `json:"blossom"`
	NIP96   []MediaServerEntry `json:"nip96"`
}

// GetUserMediaServersHandler resolves a user's Blossom (kind 10063) and NIP-96
// (kind 10096) media-server lists and annotates each entry with grain's static
// capability metadata. Defaults to the session user when no pubkey is given.
//
// @Summary      Get a user's media servers
// @Description  Resolve a user's Blossom (kind 10063) and NIP-96 (kind 10096) media-server lists, annotated with grain's capability metadata. Defaults to the session user.
// @Tags         client
// @Produce      json
// @Param        pubkey  query     string  false  "Hex pubkey (defaults to the session user)"
// @Success      200     {object}  UserMediaServersResponse
// @Failure      401     {string}  string  "Authentication required or pubkey parameter needed"
// @Router       /api/v1/user/media-servers [get]
func GetUserMediaServersHandler(w http.ResponseWriter, r *http.Request) {
	pubkey := r.URL.Query().Get("pubkey")
	if pubkey == "" {
		sess := session.SessionMgr.GetCurrentUser(r)
		if sess == nil {
			http.Error(w, "Authentication required or pubkey parameter needed", http.StatusUnauthorized)
			return
		}
		pubkey = sess.PublicKey
	}

	coreClient := connection.GetCoreClient()
	if coreClient == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}

	media := coreClient.ResolveMediaServers(pubkey)
	resp := UserMediaServersResponse{
		Pubkey:  pubkey,
		HasAny:  media.HasAny(),
		Blossom: annotateMediaServers(media.Blossom, core.MediaKindBlossom),
		NIP96:   annotateMediaServers(media.NIP96, core.MediaKindNIP96),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.ClientAPI().Error("Failed to encode media servers response", "error", err)
	}
}

// annotateMediaServers pairs each resolved URL with grain's capability metadata,
// flagging the first entry as the primary (the default upload target).
func annotateMediaServers(urls []string, kind string) []MediaServerEntry {
	entries := make([]MediaServerEntry, 0, len(urls))
	for i, u := range urls {
		entry := MediaServerEntry{URL: u, Kind: kind, Primary: i == 0}
		if info, ok := core.LookupMediaServerInfo(u); ok {
			entry.Name = info.Name
			entry.Cost = info.Cost
			entry.Retention = info.Retention
			entry.Mirror = info.Mirror
			entry.Note = info.Note
			entry.CTA = info.CTA
			entry.Known = true
		}
		entries = append(entries, entry)
	}
	return entries
}

// MediaServersBuildRequest carries the desired ordered server list for one
// media-server list kind.
type MediaServersBuildRequest struct {
	Kind    int      `json:"kind"`    // 10063 (Blossom) or 10096 (NIP-96)
	Servers []string `json:"servers"` // ordered base URLs, primary first
}

// BuildMediaServersHandler assembles an UNSIGNED media-server list event for the
// session user, preserving any non-server tags on their existing list, and
// returns it for the browser to sign. Publish the signed event via
// /api/v1/events/publish.
//
// @Summary      Build a media-server list event
// @Description  Assemble an unsigned Blossom (10063) or NIP-96 (10096) server-list event for the session user, preserving non-server tags, and return it to sign client-side.
// @Tags         client
// @Accept       json
// @Produce      json
// @Success      200  {object}  nostr.Event
// @Failure      400  {string}  string  "Invalid request"
// @Failure      401  {string}  string  "Authentication required"
// @Router       /api/v1/user/media-servers/build [post]
func BuildMediaServersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess := session.SessionMgr.GetCurrentUser(r)
	if sess == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var req MediaServersBuildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	coreClient := connection.GetCoreClient()
	if coreClient == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}

	// Fetch the user's existing list so non-server tags survive the rewrite.
	existing := coreClient.FetchMediaServerList(sess.PublicKey, req.Kind)
	unsigned, err := core.AssembleMediaServerEvent(existing, req.Kind, sess.PublicKey, req.Servers)
	if err != nil {
		log.ClientAPI().Warn("Media-server build failed", "pubkey", sess.PublicKey, "kind", req.Kind, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(unsigned); err != nil {
		log.ClientAPI().Error("Failed to encode built media-server event", "error", err)
	}
}

// GetSuggestedMediaServersHandler returns grain's curated quick-add media-server
// suggestions (Blossom preferred, NIP-96 legacy fallback) for the settings page.
//
// @Summary      Get suggested media servers
// @Description  grain's curated quick-add media-server suggestions, Blossom preferred with NIP-96 as a legacy fallback.
// @Tags         client
// @Produce      json
// @Success      200  {array}  core.MediaServerInfo
// @Router       /api/v1/media-servers/suggested [get]
func GetSuggestedMediaServersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(core.SuggestedMediaServers()); err != nil {
		log.ClientAPI().Error("Failed to encode suggested media servers", "error", err)
	}
}

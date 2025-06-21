package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/0ceanslim/grain/client/auth"
	"github.com/0ceanslim/grain/client/core"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// PublishEventRequest represents the request structure for publishing events
type PublishEventRequest struct {
	Kind       int         `json:"kind"`
	Content    string      `json:"content"`
	Tags       [][]string  `json:"tags,omitempty"`
	PrivateKey string      `json:"privateKey,omitempty"`
	Relays     []string    `json:"relays,omitempty"`
}

// PublishEventResponse represents the response structure for publishing events
type PublishEventResponse struct {
	Success    bool                     `json:"success"`
	EventID    string                   `json:"eventId,omitempty"`
	Event      *nostr.Event            `json:"event,omitempty"`
	Results    []core.BroadcastResult  `json:"results"`
	Summary    core.BroadcastSummary   `json:"summary"`
	Error      string                  `json:"error,omitempty"`
}

// PublishEventHandler handles event publishing requests
func PublishEventHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current session
	session := auth.EnhancedSessionMgr.GetCurrentUser(r)
	if session == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Parse request
	var req PublishEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Util().Error("Failed to parse publish request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get core client
	coreClient := auth.GetCoreClient()
	if coreClient == nil {
		log.Util().Error("Core client not available")
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}

	// Create event signer
	var signer *core.EventSigner
	var err error
	
	if req.PrivateKey != "" {
		// Use provided private key
		signer, err = core.NewEventSigner(req.PrivateKey)
		if err != nil {
			log.Util().Error("Invalid private key", "error", err)
			sendEventResponse(w, PublishEventResponse{
				Success: false,
				Error:   "Invalid private key",
			})
			return
		}
	} else {
		// This would need browser extension integration in a real web app
		sendEventResponse(w, PublishEventResponse{
			Success: false,
			Error:   "Private key or browser extension required",
		})
		return
	}

	// Build event
	eventBuilder := core.NewEventBuilder(req.Kind).Content(req.Content)
	
	// Add tags if provided
	for _, tag := range req.Tags {
		if len(tag) > 0 {
			eventBuilder.Tag(tag[0], tag[1:]...)
		}
	}

	// Build, sign, and publish
	event, results, err := core.PublishEvent(coreClient, signer, eventBuilder, req.Relays)
	if err != nil {
		log.Util().Error("Failed to publish event", "error", err)
		sendEventResponse(w, PublishEventResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Create response
	summary := core.SummarizeBroadcast(results)
	response := PublishEventResponse{
		Success: summary.Successful > 0,
		EventID: event.ID,
		Event:   event,
		Results: results,
		Summary: summary,
	}

	if summary.Successful == 0 {
		response.Error = "Failed to publish to any relays"
	}

	log.Util().Info("Event published", 
		"event_id", event.ID,
		"kind", event.Kind,
		"successful_relays", summary.Successful,
		"total_relays", summary.TotalRelays)

	sendEventResponse(w, response)
}

// sendEventResponse sends a JSON response
func sendEventResponse(w http.ResponseWriter, response PublishEventResponse) {
	w.Header().Set("Content-Type", "application/json")
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Util().Error("Failed to encode response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GetUserProfileHandler fetches user profile using core client
func GetUserProfileHandler(w http.ResponseWriter, r *http.Request) {
	// Get pubkey from query parameter
	pubkey := r.URL.Query().Get("pubkey")
	if pubkey == "" {
		// Use current session pubkey if not provided
		session := auth.EnhancedSessionMgr.GetCurrentUser(r)
		if session == nil {
			http.Error(w, "Authentication required or pubkey parameter needed", http.StatusUnauthorized)
			return
		}
		pubkey = session.PublicKey
	}

	// Get core client
	coreClient := auth.GetCoreClient()
	if coreClient == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}

	// Fetch profile
	profile, err := coreClient.GetUserProfile(pubkey, nil)
	if err != nil {
		log.Util().Error("Failed to fetch user profile", "pubkey", pubkey, "error", err)
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(profile); err != nil {
		log.Util().Error("Failed to encode profile response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GetUserRelaysHandler fetches user relay list using core client
func GetUserRelaysHandler(w http.ResponseWriter, r *http.Request) {
	// Get pubkey from query parameter
	pubkey := r.URL.Query().Get("pubkey")
	if pubkey == "" {
		// Use current session pubkey if not provided
		session := auth.EnhancedSessionMgr.GetCurrentUser(r)
		if session == nil {
			http.Error(w, "Authentication required or pubkey parameter needed", http.StatusUnauthorized)
			return
		}
		pubkey = session.PublicKey
	}

	// Get core client
	coreClient := auth.GetCoreClient()
	if coreClient == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}

	// Fetch relays
	relays, err := coreClient.GetUserRelays(pubkey)
	if err != nil {
		log.Util().Error("Failed to fetch user relays", "pubkey", pubkey, "error", err)
		http.Error(w, "Relays not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(relays); err != nil {
		log.Util().Error("Failed to encode relays response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// QueryEventsHandler handles event querying using core client
func QueryEventsHandler(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters into filters
	filters, err := parseFiltersFromQuery(r)
	if err != nil {
		log.Util().Error("Failed to parse query filters", "error", err)
		http.Error(w, "Invalid query parameters", http.StatusBadRequest)
		return
	}

	// Get core client
	coreClient := auth.GetCoreClient()
	if coreClient == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}

	// Create subscription to fetch events
	sub, err := coreClient.Subscribe(filters, nil)
	if err != nil {
		log.Util().Error("Failed to create subscription", "error", err)
		http.Error(w, "Query failed", http.StatusInternalServerError)
		return
	}
	defer sub.Close()

	// Collect events with timeout
	events := make([]*nostr.Event, 0)
	timeout := time.After(10 * time.Second)

	for {
		select {
		case event := <-sub.Events:
			events = append(events, event)
		case <-sub.Done:
			// Subscription completed (EOSE received)
			goto sendResponse
		case <-timeout:
			log.Util().Debug("Query timeout reached", "event_count", len(events))
			goto sendResponse
		}
	}

sendResponse:
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"events": events,
		"count":  len(events),
	}); err != nil {
		log.Util().Error("Failed to encode query response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// parseFiltersFromQuery converts HTTP query parameters to Nostr filters
func parseFiltersFromQuery(r *http.Request) ([]nostr.Filter, error) {
	query := r.URL.Query()
	
	filter := nostr.Filter{}
	
	// Parse authors
	if authors := query["authors"]; len(authors) > 0 {
		filter.Authors = authors
	}
	
	// Parse kinds
	if kindStrs := query["kinds"]; len(kindStrs) > 0 {
		kinds := make([]int, 0, len(kindStrs))
		for _, kindStr := range kindStrs {
			if kind, err := strconv.Atoi(kindStr); err == nil {
				kinds = append(kinds, kind)
			}
		}
		if len(kinds) > 0 {
			filter.Kinds = kinds
		}
	}
	
	// Parse limit
	if limitStr := query.Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filter.Limit = &limit
		}
	}
	
	// Parse IDs
	if ids := query["ids"]; len(ids) > 0 {
		filter.IDs = ids
	}
	
	return []nostr.Filter{filter}, nil
}
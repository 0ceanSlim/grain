package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/session"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// PublishEventRequest represents the request structure for publishing events
type PublishEventRequest struct {
	Kind       int        `json:"kind"`
	Content    string     `json:"content"`
	Tags       [][]string `json:"tags,omitempty"`
	PrivateKey string     `json:"privateKey,omitempty"`
	Relays     []string   `json:"relays,omitempty"`
}

// PublishEventResponse represents the response structure for publishing events
type PublishEventResponse struct {
	Success bool                   `json:"success"`
	EventID string                 `json:"eventId,omitempty"`
	Event   *nostr.Event           `json:"event,omitempty"`
	Results []core.BroadcastResult `json:"results"`
	Summary core.BroadcastSummary  `json:"summary"`
	Error   string                 `json:"error,omitempty"`
}

// PublishEventHandler handles event publishing requests
func PublishEventHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current session
	session := session.SessionMgr.GetCurrentUser(r)
	if session == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Parse request
	var req PublishEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.ClientAPI().Error("Failed to parse publish request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get core client
	coreClient := connection.GetCoreClient()
	if coreClient == nil {
		log.ClientAPI().Error("Core client not available")
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
			log.ClientAPI().Error("Invalid private key", "error", err)
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
		log.ClientAPI().Error("Failed to publish event", "error", err)
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

	log.ClientAPI().Info("Event published",
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
		log.ClientAPI().Error("Failed to encode response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GetUserProfileHandler fetches user profile using core client
func GetUserProfileHandler(w http.ResponseWriter, r *http.Request) {
	// Get pubkey from query parameter
	pubkey := r.URL.Query().Get("pubkey")
	if pubkey == "" {
		// Use current session pubkey if not provided
		session := session.SessionMgr.GetCurrentUser(r)
		if session == nil {
			http.Error(w, "Authentication required or pubkey parameter needed", http.StatusUnauthorized)
			return
		}
		pubkey = session.PublicKey
	}

	// Get core client
	coreClient := connection.GetCoreClient()
	if coreClient == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}

	// Fetch profile
	profile, err := coreClient.GetUserProfile(pubkey, nil)
	if err != nil {
		log.ClientAPI().Error("Failed to fetch user profile", "pubkey", pubkey, "error", err)
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(profile); err != nil {
		log.ClientAPI().Error("Failed to encode profile response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GetUserRelaysHandler fetches user relay list using core client
func GetUserRelaysHandler(w http.ResponseWriter, r *http.Request) {
	// Get pubkey from query parameter
	pubkey := r.URL.Query().Get("pubkey")
	if pubkey == "" {
		// Use current session pubkey if not provided
		session := session.SessionMgr.GetCurrentUser(r)
		if session == nil {
			http.Error(w, "Authentication required or pubkey parameter needed", http.StatusUnauthorized)
			return
		}
		pubkey = session.PublicKey
	}

	// Get core client
	coreClient := connection.GetCoreClient()
	if coreClient == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}

	// Fetch relays
	relays, err := coreClient.GetUserRelays(pubkey)
	if err != nil {
		log.ClientAPI().Error("Failed to fetch user relays", "pubkey", pubkey, "error", err)
		http.Error(w, "Relays not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(relays); err != nil {
		log.ClientAPI().Error("Failed to encode relays response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// QueryEventsHandler handles event querying using core client
func QueryEventsHandler(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters into filters
	filters, err := parseFiltersFromQuery(r)
	if err != nil {
		log.ClientAPI().Error("Failed to parse query filters", "error", err)
		http.Error(w, "Invalid query parameters", http.StatusBadRequest)
		return
	}

	log.ClientAPI().Debug("Query request",
		"filters", len(filters),
		"client_ip", r.RemoteAddr)

	// Get core client
	coreClient := connection.GetCoreClient()
	if coreClient == nil {
		log.ClientAPI().Error("Core client not available")
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}

	// Ensure relay connections are established before querying
	if err := connection.EnsureRelayConnections(); err != nil {
		log.ClientAPI().Error("Failed to ensure relay connections", "error", err)
		http.Error(w, "No relay connections available", http.StatusServiceUnavailable)
		return
	}

	// Create subscription to fetch events
	sub, err := coreClient.Subscribe(filters, nil)
	if err != nil {
		log.ClientAPI().Error("Failed to create subscription", "error", err)
		http.Error(w, "Query failed", http.StatusInternalServerError)
		return
	}
	defer sub.Close()

	// Use map for deduplication by event ID
	eventMap := make(map[string]*nostr.Event)
	timeout := time.After(8 * time.Second)

	// Get the limit from the first filter (if any)
	var requestedLimit int
	if len(filters) > 0 && filters[0].Limit != nil {
		requestedLimit = *filters[0].Limit
		log.ClientAPI().Debug("Query limit set", "limit", requestedLimit)
	} else {
		requestedLimit = 500 // Default max
		log.ClientAPI().Debug("No limit specified, using default", "limit", requestedLimit)
	}

	log.ClientAPI().Debug("Starting event collection with deduplication",
		"timeout_seconds", 8,
		"requested_limit", requestedLimit)

	for {
		select {
		case event := <-sub.Events:
			// Only add if we haven't seen this event ID before
			if _, exists := eventMap[event.ID]; !exists {
				eventMap[event.ID] = event
				log.ClientAPI().Debug("Added unique event",
					"event_id", event.ID,
					"kind", event.Kind,
					"unique_count", len(eventMap))

				// Stop collecting if we have enough unique events
				if len(eventMap) >= requestedLimit {
					log.ClientAPI().Debug("Reached requested limit", "unique_count", len(eventMap))
					goto sendResponse
				}
			} else {
				log.ClientAPI().Debug("Skipped duplicate event",
					"event_id", event.ID,
					"unique_count", len(eventMap))
			}

		case <-sub.Done:
			// Subscription completed (EOSE received from all relays)
			log.ClientAPI().Debug("Subscription completed (EOSE)", "unique_count", len(eventMap))
			goto sendResponse

		case <-timeout:
			log.ClientAPI().Debug("Query timeout reached", "unique_count", len(eventMap))
			goto sendResponse
		}
	}

sendResponse:
	// Convert map to slice
	events := make([]*nostr.Event, 0, len(eventMap))
	for _, event := range eventMap {
		events = append(events, event)
	}

	// Sort events by created_at (newest first) and then by ID for deterministic ordering
	sort.Slice(events, func(i, j int) bool {
		if events[i].CreatedAt == events[j].CreatedAt {
			// If timestamps are equal, sort by ID (lexicographically)
			return events[i].ID < events[j].ID
		}
		// Sort by timestamp, newest first
		return events[i].CreatedAt > events[j].CreatedAt
	})

	// Apply limit after sorting (in case we collected more than needed)
	if len(events) > requestedLimit {
		events = events[:requestedLimit]
		log.ClientAPI().Debug("Trimmed events to limit",
			"trimmed_to", len(events),
			"requested_limit", requestedLimit)
	}

	log.ClientAPI().Info("Query completed",
		"unique_events", len(events),
		"requested_limit", requestedLimit,
		"client_ip", r.RemoteAddr)

	// Check if this is a single event query by ID and handle 404
	if len(filters) == 1 && len(filters[0].IDs) == 1 && len(events) == 0 {
		// Single event query that returned no results
		log.ClientAPI().Debug("Single event not found", "event_id", filters[0].IDs[0])
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	// Set headers and send response
	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"events": events,
		"count":  len(events),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.ClientAPI().Error("Failed to encode query response", "error", err)
		return
	}
}

// parseFiltersFromQuery converts HTTP query parameters to Nostr filters
func parseFiltersFromQuery(r *http.Request) ([]nostr.Filter, error) {
	query := r.URL.Query()

	filter := nostr.Filter{}

	// Parse authors
	if authors := query["authors"]; len(authors) > 0 {
		filter.Authors = authors
		log.ClientAPI().Debug("Parsed authors", "authors", authors)
	}

	// Parse kinds
	if kindStrs := query["kinds"]; len(kindStrs) > 0 {
		kinds := make([]int, 0, len(kindStrs))
		for _, kindStr := range kindStrs {
			if kind, err := strconv.Atoi(kindStr); err == nil {
				kinds = append(kinds, kind)
			} else {
				log.ClientAPI().Warn("Invalid kind parameter", "kind", kindStr)
			}
		}
		if len(kinds) > 0 {
			filter.Kinds = kinds
			log.ClientAPI().Debug("Parsed kinds", "kinds", kinds)
		}
	}

	// Parse limit
	if limitStr := query.Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filter.Limit = &limit
			log.ClientAPI().Debug("Parsed limit", "limit", limit)
		} else {
			log.ClientAPI().Warn("Invalid limit parameter", "limit", limitStr)
		}
	}

	// Parse IDs
	if ids := query["ids"]; len(ids) > 0 {
		filter.IDs = ids
		log.ClientAPI().Debug("Parsed IDs", "ids", ids)
	}

	// Parse since timestamp
	if sinceStr := query.Get("since"); sinceStr != "" {
		if since, err := strconv.ParseInt(sinceStr, 10, 64); err == nil {
			sinceTime := time.Unix(since, 0)
			filter.Since = &sinceTime
			log.ClientAPI().Debug("Parsed since", "since", since, "time", sinceTime.Format(time.RFC3339))
		} else {
			log.ClientAPI().Warn("Invalid since parameter", "since", sinceStr)
		}
	}

	// Parse until timestamp
	if untilStr := query.Get("until"); untilStr != "" {
		if until, err := strconv.ParseInt(untilStr, 10, 64); err == nil {
			untilTime := time.Unix(until, 0)
			filter.Until = &untilTime
			log.ClientAPI().Debug("Parsed until", "until", until, "time", untilTime.Format(time.RFC3339))
		} else {
			log.ClientAPI().Warn("Invalid until parameter", "until", untilStr)
		}
	}

	log.ClientAPI().Debug("Created filter",
		"authors_count", len(filter.Authors),
		"kinds_count", len(filter.Kinds),
		"ids_count", len(filter.IDs),
		"has_limit", filter.Limit != nil,
		"has_since", filter.Since != nil,
		"has_until", filter.Until != nil)

	return []nostr.Filter{filter}, nil
}

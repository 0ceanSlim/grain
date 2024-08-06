package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"

	"grain/server/db"
	relay "grain/server/types"

	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/net/websocket"
)

type ResultData struct {
	Success bool
	Message string
	Count   int
}

func ImportEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	pubkey := r.FormValue("pubkey")
	relayUrls := r.FormValue("relayUrls")
	urls := strings.Split(relayUrls, ",")

	var totalEvents int
	var errorMessage string

	for _, url := range urls {
		events, err := fetchEventsFromRelay(pubkey, url)
		if err != nil {
			log.Printf("Error fetching events from relay %s: %v", url, err)
			errorMessage = fmt.Sprintf("Error fetching events from relay %s", url)
			renderResult(w, false, errorMessage, 0)
			return
		}

		err = storeEvents(events)
		if err != nil {
			log.Printf("Error storing events: %v", err)
			errorMessage = "Error storing events"
			renderResult(w, false, errorMessage, 0)
			return
		}

		totalEvents += len(events)
	}

	renderResult(w, true, "Events imported successfully", totalEvents)
}

func renderResult(w http.ResponseWriter, success bool, message string, count int) {
	tmpl, err := template.New("result").Parse(`
		{{ if .Success }}
		<p class="text-green-500">Successfully inserted {{ .Count }} events.</p>
		{{ else }}
		<p class="text-red-500">Failed to import events: {{ .Message }}</p>
		{{ end }}
	`)
	if err != nil {
		http.Error(w, "Error generating result", http.StatusInternalServerError)
		return
	}

	data := ResultData{
		Success: success,
		Message: message,
		Count:   count,
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Error rendering result", http.StatusInternalServerError)
	}
}

func fetchEventsFromRelay(pubkey, relayUrl string) ([]relay.Event, error) {
	log.Printf("Connecting to relay: %s", relayUrl)
	conn, err := websocket.Dial(relayUrl, "", "http://localhost/")
	if err != nil {
		log.Printf("Error connecting to relay %s: %v", relayUrl, err)
		return nil, err
	}
	defer conn.Close()
	log.Printf("Connected to relay: %s", relayUrl)

	reqMessage := fmt.Sprintf(`["REQ", "import-sub", {"authors": ["%s"]}]`, pubkey)
	log.Printf("Sending request: %s", reqMessage)
	if _, err := conn.Write([]byte(reqMessage)); err != nil {
		log.Printf("Error sending request to relay %s: %v", relayUrl, err)
		return nil, err
	}

	var events []relay.Event
	for {
		var msg []byte
		if err := websocket.Message.Receive(conn, &msg); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error receiving message from relay %s: %v", relayUrl, err)
			return nil, err
		}

		log.Printf("Received message: %s", string(msg))

		var response []interface{}
		if err := json.Unmarshal(msg, &response); err != nil {
			log.Printf("Error unmarshaling message from relay %s: %v", relayUrl, err)
			return nil, err
		}

		if response[0] == "EOSE" {
			log.Printf("Received EOSE message from relay %s, closing connection", relayUrl)
			break
		}

		if response[0] == "EVENT" {
			eventData, err := json.Marshal(response[2]) // Change index from 1 to 2
			if err != nil {
				log.Printf("Error marshaling event data from relay %s: %v", relayUrl, err)
				continue
			}
			var event relay.Event
			if err := json.Unmarshal(eventData, &event); err != nil {
				log.Printf("Error unmarshaling event data from relay %s: %v", relayUrl, err)
				continue
			}
			events = append(events, event)
		}
	}

	log.Printf("Fetched %d events from relay %s", len(events), relayUrl)
	return events, nil
}

func storeEvents(events []relay.Event) error {
	for _, event := range events {
		collection := db.GetCollection(event.Kind)
		_, err := collection.InsertOne(context.TODO(), event)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				log.Printf("Duplicate event ID: %s for event kind: %d", event.ID, event.Kind)
			} else {
				log.Printf("Error inserting event with ID: %s for event kind: %d: %v", event.ID, event.Kind, err)
				return err
			}
		} else {
			log.Printf("Successfully inserted event with ID: %s for event kind: %d", event.ID, event.Kind)
		}
	}
	return nil
}

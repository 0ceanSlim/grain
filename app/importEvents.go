package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"grain/server/db"
	relay "grain/server/types"

	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/net/websocket"
)

func ImportEventsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	pubkey := r.FormValue("pubkey")
	relayUrls := r.FormValue("relayUrls")
	urls := strings.Split(relayUrls, ",")

	for _, url := range urls {
		events, err := fetchEventsFromRelay(pubkey, url)
		if err != nil {
			log.Printf("Error fetching events from relay %s: %v", url, err)
			http.Error(w, fmt.Sprintf("Error fetching events from relay %s", url), http.StatusInternalServerError)
			return
		}

		err = storeEvents(events)
		if err != nil {
			log.Printf("Error storing events: %v", err)
			http.Error(w, "Error storing events", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
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
		if err != nil && !mongo.IsDuplicateKeyError(err) {
			log.Printf("Error inserting event into MongoDB: %v", err)
			return err
		}
	}
	return nil
}
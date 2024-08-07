package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"grain/config"

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

	totalEventsChan := make(chan int)
	errorChan := make(chan error)

	go func() {
		var totalEvents int
		var err error

		for _, url := range urls {
			var events []map[string]interface{}
			events, err = fetchEventsFromRelay(pubkey, url)
			if err != nil {
				errorChan <- fmt.Errorf("error fetching events from relay %s: %w", url, err)
				return
			}

			err = sendEventsToRelay(events)
			if err != nil {
				errorChan <- fmt.Errorf("error sending events to relay: %w", err)
				return
			}

			totalEvents += len(events)
		}

		totalEventsChan <- totalEvents
	}()

	select {
	case totalEvents := <-totalEventsChan:
		renderResult(w, true, "Events imported successfully", totalEvents)
	case err := <-errorChan:
		renderResult(w, false, err.Error(), 0)
	case <-time.After(5 * time.Minute):
		renderResult(w, false, "Timeout importing events", 0)
	}
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

func fetchEventsFromRelay(pubkey, relayUrl string) ([]map[string]interface{}, error) {
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

	var events []map[string]interface{}
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
			eventData, ok := response[2].(map[string]interface{})
			if !ok {
				log.Printf("Invalid event data format from relay %s", relayUrl)
				continue
			}
			events = append(events, eventData)
		}
	}

	log.Printf("Fetched %d events from relay %s", len(events), relayUrl)
	return events, nil
}

func sendEventsToRelay(events []map[string]interface{}) error {
	// Use the configuration to get the port
	cfg, err := config.LoadConfig("config.yml")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	relayUrl := fmt.Sprintf("ws://localhost%s", cfg.Server.Port)

	batchSize := 5 // Reduce the batch size to avoid connection issues
	for i := 0; i < len(events); i += batchSize {
		end := i + batchSize
		if end > len(events) {
			end = len(events)
		}
		batch := events[i:end]

		if err := sendBatchToRelay(batch, relayUrl); err != nil {
			return err
		}
	}

	return nil
}

func sendBatchToRelay(events []map[string]interface{}, relayUrl string) error {
	log.Printf("Connecting to local relay: %s", relayUrl)
	conn, err := websocket.Dial(relayUrl, "", "http://localhost/")
	if err != nil {
		log.Printf("Error connecting to local relay: %v", err)
		return err
	}
	defer conn.Close()
	log.Printf("Connected to local relay: %s", relayUrl)

	for _, event := range events {
		eventMessage := []interface{}{"EVENT", event}
		eventMessageBytes, err := json.Marshal(eventMessage)
		if err != nil {
			log.Printf("Error marshaling event message: %v", err)
			return err
		}

		if _, err := conn.Write(eventMessageBytes); err != nil {
			log.Printf("Error sending event message to local relay: %v", err)
			return err
		}
		log.Printf("Sent event to local relay: %s", event["id"])
	}

	// Wait for a short period to avoid overloading the relay server
	time.Sleep(100 * time.Millisecond)

	return nil
}

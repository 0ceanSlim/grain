package events

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/websocket"
)

type Event struct {
	ID        string     `json:"id"`
	PubKey    string     `json:"pubkey"`
	CreatedAt int64      `json:"created_at"`
	Kind      int        `json:"kind"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
	Sig       string     `json:"sig"`
}

var (
	client      *mongo.Client
	collections = make(map[int]*mongo.Collection)
)

func SetClient(mongoClient *mongo.Client) {
	client = mongoClient
}

func GetCollection(kind int) *mongo.Collection {
	if collection, exists := collections[kind]; exists {
		return collection
	}
	collectionName := fmt.Sprintf("event-kind%d", kind)
	collection := client.Database("grain").Collection(collectionName)
	collections[kind] = collection
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	_, err := collection.Indexes().CreateOne(context.TODO(), indexModel)
	if err != nil {
		fmt.Printf("Failed to create index on %s: %v\n", collectionName, err)
	}
	return collection
}

func HandleEvent(ctx context.Context, evt Event, ws *websocket.Conn) {
	if !CheckSignature(evt) {
		sendOKResponse(ws, evt.ID, false, "invalid: signature verification failed")
		return
	}

	collection := GetCollection(evt.Kind)

	var err error
	switch evt.Kind {
	case 0:
		err = HandleEventKind0(ctx, evt, collection)
	case 1:
		err = HandleEventKind1(ctx, evt, collection)
	default:
		err = HandleUnknownEvent(ctx, evt, collection)
	}

	if err != nil {
		sendOKResponse(ws, evt.ID, false, fmt.Sprintf("error: %v", err))
		return
	}

	sendOKResponse(ws, evt.ID, true, "")
}

func sendOKResponse(ws *websocket.Conn, eventID string, status bool, message string) {
	response := []interface{}{"OK", eventID, status, message}
	responseBytes, _ := json.Marshal(response)
	websocket.Message.Send(ws, string(responseBytes))
}

func SerializeEvent(evt Event) []byte {
	eventData := []interface{}{
		0,
		evt.PubKey,
		evt.CreatedAt,
		evt.Kind,
		evt.Tags,
		evt.Content,
	}
	serializedEvent, _ := json.Marshal(eventData)
	return serializedEvent
}

func CheckSignature(evt Event) bool {
	serializedEvent := SerializeEvent(evt)
	hash := sha256.Sum256(serializedEvent)
	eventID := hex.EncodeToString(hash[:])
	if eventID != evt.ID {
		log.Printf("Invalid ID: expected %s, got %s\n", eventID, evt.ID)
		return false
	}

	sigBytes, err := hex.DecodeString(evt.Sig)
	if err != nil {
		log.Printf("Error decoding signature: %v\n", err)
		return false
	}

	sig, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		log.Printf("Error parsing signature: %v\n", err)
		return false
	}

	pubKeyBytes, err := hex.DecodeString(evt.PubKey)
	if err != nil {
		log.Printf("Error decoding public key: %v\n", err)
		return false
	}

	var pubKey *btcec.PublicKey
	if len(pubKeyBytes) == 32 {
		// Handle 32-byte public key (x-coordinate only)
		pubKey, err = btcec.ParsePubKey(append([]byte{0x02}, pubKeyBytes...))
	} else {
		// Handle standard compressed or uncompressed public key
		pubKey, err = btcec.ParsePubKey(pubKeyBytes)
	}
	if err != nil {
		log.Printf("Error parsing public key: %v\n", err)
		return false
	}

	verified := sig.Verify(hash[:], pubKey)
	if !verified {
		log.Printf("Signature verification failed for event ID: %s\n", evt.ID)
	}

	return verified
}

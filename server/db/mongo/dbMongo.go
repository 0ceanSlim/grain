package mongo

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils/log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var client *mongo.Client
var collections = make(map[int]*mongo.Collection)
var databaseName string // Store the database name globally

// SetGlobalClient safely sets the global MongoDB client (used for reconnection)
func SetGlobalClient(newClient *mongo.Client, dbName string) error {
	client = newClient
	databaseName = dbName

	// Clear existing collection cache
	collections = make(map[int]*mongo.Collection)

	// Ensure indexes on reconnection
	if err := EnsureIndexes(client, databaseName); err != nil {
		log.Mongo().Warn("Error ensuring indexes after reconnection", "error", err)
	}

	log.Mongo().Info("Global MongoDB client updated successfully", "database", dbName)
	return nil
}

// GetClient returns the MongoDB client
func GetClient() *mongo.Client {
	return client
}

// GetDatabaseName returns the database name from config
func GetDatabaseName() string {
	return databaseName
}

// InitDB establishes a connection to MongoDB with retry logic and graceful error handling
func InitDB(cfg *cfgType.ServerConfig) (*mongo.Client, error) {
	return InitDBWithRetry(cfg, 5, 2*time.Second, 30*time.Second)
}

// InitializeDatabase attempts to connect to MongoDB with graceful fallback
func InitializeDatabase(cfg *cfgType.ServerConfig) (*mongo.Client, bool) {
	log.Mongo().Info("Initializing database connection")

	// Attempt to connect to MongoDB
	dbClient, err := InitDB(cfg)
	if err != nil {
		// Check if this is a connection error we can recover from
		if isConnectionError(err) {
			log.Mongo().Warn("MongoDB unavailable, starting in degraded mode",
				"error", err,
				"recovery_info", "MongoDB health monitoring will attempt reconnection")

			// Server can still start without database
			return nil, false
		} else {
			// For non-connection errors (like auth failures), log as error but still continue
			log.Mongo().Error("MongoDB initialization failed, starting without database",
				"error", err,
				"error_type", fmt.Sprintf("%T", err))
			return nil, false
		}
	}

	log.Mongo().Info("Database connection established successfully")
	return dbClient, true
}

// StartMongoHealthMonitor runs background MongoDB health monitoring and reconnection
func StartMongoHealthMonitor(cfg *cfgType.ServerConfig) {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	log.Mongo().Info("MongoDB health monitor started",
		"check_interval_sec", 30)

	for range ticker.C {
		// Skip if MongoDB is already connected
		if GetClient() != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			if IsClientHealthy(ctx) {
				cancel()
				log.Mongo().Debug("MongoDB health monitor: connection healthy, stopping monitor")
				return
			}
			cancel()
		}

		// Attempt silent connection to check if MongoDB is back
		testClient, err := InitDBSilent(cfg)
		if err != nil {
			log.Mongo().Debug("MongoDB still unavailable",
				"error_type", fmt.Sprintf("%T", err))
			continue
		}

		// MongoDB is back!
		log.Mongo().Info("MongoDB connection restored, reinitializing database")

		// Set the global client
		if err := SetGlobalClient(testClient, cfg.MongoDB.Database); err != nil {
			log.Mongo().Error("Failed to set global MongoDB client", "error", err)
			DisconnectDB(testClient)
			continue
		}

		log.Mongo().Info("MongoDB health monitor completed recovery, stopping monitor")
		return
	}
}

// InitDBWithRetry establishes a MongoDB connection with configurable retry logic
func InitDBWithRetry(cfg *cfgType.ServerConfig, maxRetries int, baseDelay, timeout time.Duration) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	clientOptions := options.Client().ApplyURI(cfg.MongoDB.URI)
	// Set connection pool options for better reliability
	clientOptions.SetMaxPoolSize(20)
	clientOptions.SetMinPoolSize(5)
	clientOptions.SetMaxConnIdleTime(30 * time.Second)
	clientOptions.SetServerSelectionTimeout(10 * time.Second)

	log.Mongo().Info("Attempting to connect to MongoDB",
		"uri", sanitizeURIForLog(cfg.MongoDB.URI),
		"database", cfg.MongoDB.Database,
		"max_retries", maxRetries,
		"timeout_sec", int(timeout.Seconds()))

	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Mongo().Debug("MongoDB connection attempt",
			"attempt", attempt,
			"max_retries", maxRetries)

		// Create client
		mongoClient, err := mongo.Connect(ctx, clientOptions)
		if err != nil {
			lastErr = err
			log.Mongo().Warn("Failed to create MongoDB client",
				"attempt", attempt,
				"error", err,
				"error_type", fmt.Sprintf("%T", err))

			if attempt < maxRetries {
				delay := calculateBackoffDelay(baseDelay, attempt)
				log.Mongo().Debug("Retrying MongoDB connection",
					"retry_in_sec", int(delay.Seconds()),
					"next_attempt", attempt+1)
				time.Sleep(delay)
				continue
			}
			break
		}

		// Test connection with ping
		pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
		err = mongoClient.Ping(pingCtx, nil)
		pingCancel()

		if err != nil {
			lastErr = err
			log.Mongo().Warn("MongoDB ping failed",
				"attempt", attempt,
				"error", err,
				"error_type", fmt.Sprintf("%T", err))

			// Close the client before retrying
			disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
			mongoClient.Disconnect(disconnectCtx)
			disconnectCancel()

			if attempt < maxRetries {
				delay := calculateBackoffDelay(baseDelay, attempt)
				log.Mongo().Debug("Retrying MongoDB connection after ping failure",
					"retry_in_sec", int(delay.Seconds()),
					"next_attempt", attempt+1)
				time.Sleep(delay)
				continue
			}
			break
		}

		// Success!
		client = mongoClient
		databaseName = cfg.MongoDB.Database

		log.Mongo().Info("Connected to MongoDB successfully",
			"attempt", attempt,
			"database", cfg.MongoDB.Database)

		// Ensure indexes on successful connection
		if err := EnsureIndexes(client, databaseName); err != nil {
			log.Mongo().Warn("Error ensuring indexes during connection", "error", err)
		}

		return client, nil
	}

	// All attempts failed
	log.Mongo().Error("Failed to connect to MongoDB after all retry attempts",
		"max_retries", maxRetries,
		"total_time_sec", int(timeout.Seconds()),
		"last_error", lastErr,
		"uri", sanitizeURIForLog(cfg.MongoDB.URI))

	return nil, fmt.Errorf("failed to connect to MongoDB after %d attempts: %w", maxRetries, lastErr)
}

// calculateBackoffDelay implements exponential backoff with jitter
func calculateBackoffDelay(baseDelay time.Duration, attempt int) time.Duration {
	// Exponential backoff: baseDelay * 2^(attempt-1)
	delay := baseDelay * time.Duration(1<<uint(attempt-1))

	// Cap at 30 seconds maximum
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}

	// Add jitter (±25% randomization)
	jitter := time.Duration(float64(delay) * (0.5 + rand.Float64()*0.5))

	return jitter
}

// sanitizeURIForLog removes credentials from MongoDB URI for safe logging
func sanitizeURIForLog(uri string) string {
	// Pattern to match MongoDB URIs with credentials
	re := regexp.MustCompile(`mongodb(\+srv)?://[^:]+:[^@]+@`)
	return re.ReplaceAllString(uri, "mongodb$1://[HIDDEN]@")
}

// InitDBSilent attempts to connect to MongoDB without logging errors
// Useful for health checks where failures are expected
func InitDBSilent(cfg *cfgType.ServerConfig) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(cfg.MongoDB.URI)
	clientOptions.SetServerSelectionTimeout(3 * time.Second)

	mongoClient, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	err = mongoClient.Ping(ctx, nil)
	if err != nil {
		mongoClient.Disconnect(ctx)
		return nil, err
	}

	return mongoClient, nil
}

func GetCollection(kind int) *mongo.Collection {
	collectionName := fmt.Sprintf("event-kind%d", kind)

	// Check if we already have this collection cached
	if collection, exists := collections[kind]; exists {
		return collection
	}

	// Check if client is available
	client := GetClient()
	if client == nil {
		log.Mongo().Warn("MongoDB client not available for collection access",
			"collection", collectionName,
			"kind", kind)
		return nil
	}

	// Create and cache the collection
	collection := client.Database(databaseName).Collection(collectionName)
	collections[kind] = collection

	log.Mongo().Debug("Collection cached",
		"collection", collectionName,
		"kind", kind)

	return collection
}

// isConnectionError checks if an error is related to MongoDB connection issues
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	connectionErrors := []string{
		"client is disconnected",
		"connection reset",
		"connection refused",
		"no reachable servers",
		"topology is closed",
		"context deadline exceeded",
		"network is unreachable",
		"server selection error",
		"server selection timeout",
	}

	for _, connErr := range connectionErrors {
		if strings.Contains(errStr, connErr) {
			return true
		}
	}

	return false
}

// IsClientHealthy checks if the MongoDB client is available and connected
func IsClientHealthy(ctx context.Context) bool {
	client := GetClient()
	if client == nil {
		return false
	}

	// Quick ping to verify connection
	err := client.Ping(ctx, nil)
	return err == nil
}

// DisconnectDB safely disconnects from MongoDB
func DisconnectDB(client *mongo.Client) {
	if client == nil {
		log.Mongo().Warn("Attempted to disconnect nil MongoDB client")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Disconnect(ctx)
	if err != nil {
		log.Mongo().Error("Error disconnecting from MongoDB", "error", err)
	} else {
		log.Mongo().Info("Disconnected from MongoDB successfully")
	}
}

// EnsureIndexes creates necessary indexes on all collections
func EnsureIndexes(client *mongo.Client, databaseName string) error {
	log.Mongo().Info("Ensuring indexes for all collections", "database", databaseName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collections, err := client.Database(databaseName).ListCollectionNames(ctx, bson.D{})
	if err != nil {
		log.Mongo().Error("Error listing collections", "error", err)
		return fmt.Errorf("error listing collections: %v", err)
	}

	log.Mongo().Debug("Found collections", "count", len(collections))

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.M{"id": 1},
			Options: options.Index().SetUnique(true).SetName("unique_id_index"),
		},
		{
			Keys:    bson.M{"pubkey": 1},
			Options: options.Index().SetName("pubkey_index"),
		},
		{
			Keys:    bson.M{"kind": 1},
			Options: options.Index().SetName("kind_index"),
		},
		{
			Keys:    bson.M{"created_at": -1},
			Options: options.Index().SetName("created_at_index"),
		},
	}

	indexStats := map[string]int{
		"processed": 0,
		"skipped":   0,
		"errors":    0,
	}

	for _, collectionName := range collections {
		collection := client.Database(databaseName).Collection(collectionName)
		indexStats["processed"]++

		for _, index := range indexes {
			_, err := collection.Indexes().CreateOne(ctx, index)
			if err != nil {
				if strings.Contains(err.Error(), "IndexKeySpecsConflict") ||
					strings.Contains(err.Error(), "already exists") {
					indexStats["skipped"]++
					// Log at debug level since this is expected behavior
					log.Mongo().Debug("Index already exists with different options, skipping",
						"collection", collectionName,
						"index_keys", fmt.Sprintf("%v", index.Keys),
						"error_type", "index_conflict")
				} else {
					indexStats["errors"]++
					// Log unexpected errors at error level
					log.Mongo().Error("Failed to create index",
						"collection", collectionName,
						"index_keys", fmt.Sprintf("%v", index.Keys),
						"error", err)
				}
			} else {
				log.Mongo().Debug("Index created successfully",
					"collection", collectionName,
					"index_keys", fmt.Sprintf("%v", index.Keys))
			}
		}
	}

	log.Mongo().Info("Index creation completed",
		"collections_processed", indexStats["processed"],
		"indexes_skipped", indexStats["skipped"],
		"errors", indexStats["errors"])

	return nil
}

package mongo

import (
	"context"
	"fmt"
	"log/slog"

	relay "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DiagnoseQueryIssues performs comprehensive database query diagnostics
func DiagnoseQueryIssues(filters []relay.Filter, client *mongo.Client, databaseName string) {
	diagLog := utils.GetLogger("mongo-diag")
	
	diagLog.Info("Starting query diagnostics", 
		"database", databaseName,
		"filter_count", len(filters))

	// Step 1: Check database and collections exist
	collections, err := client.Database(databaseName).ListCollectionNames(context.TODO(), bson.M{})
	if err != nil {
		diagLog.Error("Failed to list collections", "error", err)
		return
	}
	
	diagLog.Info("Database collections found", 
		"database", databaseName,
		"collection_count", len(collections),
		"collections", collections)

	// Step 2: Check document counts per collection
	var totalDocs int64
	for _, collectionName := range collections {
		collection := client.Database(databaseName).Collection(collectionName)
		count, err := collection.CountDocuments(context.TODO(), bson.M{})
		if err != nil {
			diagLog.Error("Failed to count documents", 
				"collection", collectionName, 
				"error", err)
			continue
		}
		
		diagLog.Info("Collection document count", 
			"collection", collectionName, 
			"count", count)
		totalDocs += count
		
		// Sample a few documents to check structure
		if count > 0 {
			sampleDocs(collection, collectionName, diagLog)
		}
	}
	
	diagLog.Info("Total documents across all collections", "total", totalDocs)

	// Step 3: Analyze the specific filters being applied
	for i, filter := range filters {
		diagLog.Info("Analyzing filter", 
			"filter_index", i,
			"ids_count", len(filter.IDs),
			"authors_count", len(filter.Authors), 
			"kinds_count", len(filter.Kinds),
			"tags_count", len(filter.Tags),
			"has_since", filter.Since != nil,
			"has_until", filter.Until != nil,
			"has_limit", filter.Limit != nil)
			
		// Log specific filter values
		if len(filter.IDs) > 0 {
			diagLog.Debug("Filter IDs", "ids", filter.IDs[:min(5, len(filter.IDs))])
		}
		if len(filter.Authors) > 0 {
			diagLog.Debug("Filter authors", "authors", filter.Authors[:min(3, len(filter.Authors))])
		}
		if len(filter.Kinds) > 0 {
			diagLog.Debug("Filter kinds", "kinds", filter.Kinds)
		}
		if filter.Limit != nil {
			diagLog.Debug("Filter limit", "limit", *filter.Limit)
		}
	}

	// Step 4: Test the actual MongoDB query construction
	testQueryConstruction(filters, collections, client, databaseName, diagLog)
}

// sampleDocs retrieves and logs sample documents from a collection
func sampleDocs(collection *mongo.Collection, collectionName string, diagLog *slog.Logger) {
	// Get first document
	cursor, err := collection.Find(context.TODO(), bson.M{}, options.Find().SetLimit(1))
	if err != nil {
		diagLog.Error("Failed to sample documents", 
			"collection", collectionName, 
			"error", err)
		return
	}
	defer cursor.Close(context.TODO())

	for cursor.Next(context.TODO()) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			diagLog.Error("Failed to decode sample document", 
				"collection", collectionName, 
				"error", err)
			continue
		}
		
		// Log key fields
		diagLog.Debug("Sample document structure", 
			"collection", collectionName,
			"id", doc["id"],
			"pubkey", doc["pubkey"],
			"kind", doc["kind"],
			"created_at", doc["created_at"],
			"has_tags", doc["tags"] != nil,
			"has_content", doc["content"] != nil)
	}
}

// testQueryConstruction tests the MongoDB query generation logic
func testQueryConstruction(filters []relay.Filter, collections []string, client *mongo.Client, databaseName string, diagLog *slog.Logger) {
	var combinedFilters []bson.M

	// Replicate the exact query construction logic from QueryEvents
	for _, filter := range filters {
		filterBson := bson.M{}

		if len(filter.IDs) > 0 {
			filterBson["id"] = bson.M{"$in": filter.IDs}
		}
		if len(filter.Authors) > 0 {
			filterBson["pubkey"] = bson.M{"$in": filter.Authors}
		}
		if len(filter.Kinds) > 0 {
			filterBson["kind"] = bson.M{"$in": filter.Kinds}
		}
		
		// Tag filtering logic
		if filter.Tags != nil {
			for key, values := range filter.Tags {
				if len(values) > 0 && len(key) > 0 {
					tagKey := key
					if tagKey[0] == '#' {
						tagKey = tagKey[1:]
					}
					
					filterBson["tags"] = bson.M{
						"$elemMatch": bson.M{
							"0": tagKey,
							"1": bson.M{"$in": values},
						},
					}
				}
			}
		}
		
		if filter.Since != nil {
			filterBson["created_at"] = bson.M{"$gte": *filter.Since}
		}
		if filter.Until != nil {
			if filterBson["created_at"] == nil {
				filterBson["created_at"] = bson.M{"$lte": *filter.Until}
			} else {
				filterBson["created_at"].(bson.M)["$lte"] = *filter.Until
			}
		}

		combinedFilters = append(combinedFilters, filterBson)
		
		diagLog.Debug("Generated MongoDB filter", 
			"filter_index", len(combinedFilters)-1,
			"bson_filter", fmt.Sprintf("%+v", filterBson))
	}

	// Test the combined query
	query := bson.M{}
	if len(combinedFilters) > 0 {
		query["$or"] = combinedFilters
	}
	
	diagLog.Info("Final MongoDB query", 
		"query", fmt.Sprintf("%+v", query),
		"is_empty_query", len(query) == 0)

	// Test query on each collection individually
	for _, collectionName := range collections {
		if !isEventCollection(collectionName) {
			continue
		}
		
		collection := client.Database(databaseName).Collection(collectionName)
		
		// Test with empty query first
		emptyCount, err := collection.CountDocuments(context.TODO(), bson.M{})
		if err != nil {
			diagLog.Error("Failed to count with empty query", 
				"collection", collectionName, 
				"error", err)
			continue
		}
		
		// Test with constructed query
		queryCount, err := collection.CountDocuments(context.TODO(), query)
		if err != nil {
			diagLog.Error("Failed to count with constructed query", 
				"collection", collectionName, 
				"error", err)
			continue
		}
		
		diagLog.Info("Query test results", 
			"collection", collectionName,
			"total_docs", emptyCount,
			"matching_query", queryCount,
			"query_matches", queryCount > 0)
			
		// If there's a mismatch, let's understand why
		if emptyCount > 0 && queryCount == 0 {
			diagnoseQueryMismatch(collection, query, collectionName, diagLog)
		}
	}
}

// diagnoseQueryMismatch investigates why a query returns no results when documents exist
func diagnoseQueryMismatch(collection *mongo.Collection, query bson.M, collectionName string, diagLog *slog.Logger) {
	diagLog.Warn("Query mismatch detected - investigating", "collection", collectionName)
	
	// Get sample documents to compare against query
	cursor, err := collection.Find(context.TODO(), bson.M{}, options.Find().SetLimit(3))
	if err != nil {
		diagLog.Error("Failed to get sample docs for mismatch analysis", "error", err)
		return
	}
	defer cursor.Close(context.TODO())

	sampleCount := 0
	for cursor.Next(context.TODO()) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		
		sampleCount++
		diagLog.Debug("Sample document for mismatch analysis", 
			"collection", collectionName,
			"sample_num", sampleCount,
			"id", doc["id"],
			"pubkey", doc["pubkey"], 
			"kind", doc["kind"],
			"created_at_type", fmt.Sprintf("%T", doc["created_at"]),
			"created_at_value", doc["created_at"],
			"tags_type", fmt.Sprintf("%T", doc["tags"]))
			
		// Test if this specific document would match our query
		testSingleDoc(collection, doc, query, diagLog)
	}
}

// testSingleDoc tests if a specific document matches the query
func testSingleDoc(collection *mongo.Collection, doc bson.M, originalQuery bson.M, diagLog *slog.Logger) {
	if id, ok := doc["id"].(string); ok {
		// Test if we can find this specific document with our query
		modifiedQuery := bson.M{
			"$and": []bson.M{
				{"id": id}, // Must match this specific document
				originalQuery, // AND our original query
			},
		}
		
		count, err := collection.CountDocuments(context.TODO(), modifiedQuery)
		if err != nil {
			diagLog.Error("Failed to test single document", "error", err)
			return
		}
		
		diagLog.Debug("Single document query test", 
			"document_id", id,
			"matches_query", count > 0,
			"test_query", fmt.Sprintf("%+v", modifiedQuery))
	}
}

// isEventCollection checks if a collection name represents an event collection
func isEventCollection(name string) bool {
	return len(name) > 11 && name[:11] == "event-kind"
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
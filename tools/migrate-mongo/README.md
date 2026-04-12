# MongoDB to Grain Migration Tool

Exports events from a grain MongoDB database to JSONL format for import into grain's nostrdb.

## Build
```
cd tools/migrate-mongo
go build -o migrate-mongo .
```

## Usage
```
./migrate-mongo --uri "mongodb://localhost:27017" --database grain --output events.jsonl
```

## Then import into grain
```
grain --import events.jsonl
```

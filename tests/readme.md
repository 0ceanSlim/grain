# GRAIN Testing

Integration test suite for GRAIN relay using Docker test environment.

**Note: All commands in this README should be run from the `tests/` directory.**

## Quick Start

Run the full test suite with interactive cleanup:

```bash
make test
```

This will:

1. Start Docker test environment (GRAIN + MongoDB)
2. Wait for services to be ready
3. Run all integration tests
4. Prompt whether to stop environment and collect logs, or keep the test enviornment up

## Manual Testing

### Step-by-step testing:

```bash
# Navigate to tests directory first
cd tests/

# 1. Start test environment
make test-start

# 2. Run all tests (can repeat multiple times)
make test-run

# 3. Run specific test function
make test-single TEST=TestBasicConnection

# 4. Run all tests in a file
make test-file FILE=relay_test.go

# 5. Stop environment and collect logs
make test-stop
```

## Test Structure

```
tests/                     # Run all commands from this directory
├── Makefile               # Test commands
├── README.md              # This file
├── helpers.go             # Test utilities
├── logs/                  # Generated test logs and results
├── docker/                # Test environment
│   ├── Dockerfile         # Test container build
│   └── docker-compose.yml # Test services
└── integration/          # Integration tests
    ├── relay_test.go      # Core relay functionality
    ├── websocket_test.go  # WebSocket connection tests
    └── api_test.go        # HTTP API tests
```

## Available Commands

| Command                          | Description                                  |
| -------------------------------- | -------------------------------------------- |
| `make test`                      | Start environment, run tests, prompt cleanup |
| `make test-start`                | Start test environment only                  |
| `make test-run`                  | Full automated cycle (start, test, stop)     |
| `make test-single TEST=TestName` | Run specific test function by name           |
| `make test-file   TEST=fie.go  ` | Run all tests in a single go file            |
| `make test-stop`                 | Stop environment and collect logs            |
| `make test-clean-logs`           | Remove all log files                         |
| `make help`                      | Show all available commands                  |

**Note on test names**: The `TEST=` parameter takes the actual Go test function name (e.g., `TestRelayBasics`, `TestWebSocketConnection`, `TestEventPublishing`).

## Test Environment

The test environment uses:

- **GRAIN**: `localhost:8181`
- **MongoDB**: `localhost:27017`
- **Environment**: Ephemeral (no data persistence)
- **Containers**: `grain-test-relay`, `grain-test-mongo`

## Logs and Results

Test results and logs are automatically saved to the `logs/` directory with timestamps:

- `test-results-YYYYMMDD_HHMMSS.log` - Full test run output
- `test-TestName-YYYYMMDD_HHMMSS.log` - Individual test output
- `grain-YYYYMMDD_HHMMSS.log` - Container logs
- `debug-YYYYMMDD_HHMMSS.log` - Application debug logs

Use `make test-clean-logs` to remove all log files when needed.

## Debugging

### Viewing Container Logs

While the test environment is running, you can monitor logs in real-time:

```bash
# View GRAIN logs
cd docker && docker-compose logs -f grain

# View MongoDB logs
cd docker && docker-compose logs -f mongo
```

### Viewiing GRAIN debug.log

GRAIN writes detailed logs to `debug.log` inside the container:

```bash
# View debug log in real-time
cd docker && docker-compose exec grain tail -f debug.log
```

### Editing Configuration During Testing

**Option 1: Edit files inside the container**

```bash
# Access the container shell
cd docker && docker-compose exec grain sh

# Edit configuration files using vi
vi config.yml
vi whitelist.yml
vi blacklist.yml
vi relay_metadata.json

# To edit and save in vi:
# Press 'i' to enter insert mode, make changes
# Press 'Esc', then type ':wq' and Enter to save
```

**Option 2: Edit locally (recommended)**

```bash
# Copy config files from container to local directory
cd docker && docker-compose cp grain:/app/config.yml ./config.yml
cd docker && docker-compose cp grain:/app/whitelist.yml ./whitelist.yml
cd docker && docker-compose cp grain:/app/blacklist.yml ./blacklist.yml
cd docker && docker-compose cp grain:/app/relay_metadata.json ./relay_metadata.json

# Edit locally with your preferred editor
nano config.yml    # or code config.yml, vim config.yml, etc.

# Copy edited files back to container
cd docker && docker-compose cp ./config.yml grain:/app/config.yml
cd docker && docker-compose cp ./whitelist.yml grain:/app/whitelist.yml
cd docker && docker-compose cp ./blacklist.yml grain:/app/blacklist.yml
cd docker && docker-compose cp ./relay_metadata.json grain:/app/relay_metadata.json
```

**Note**: GRAIN watches configuration files and automatically restarts when changes are saved. No need to restart the container manually.

### Database Operations

```bash
# Access MongoDB shell
cd docker && docker-compose exec mongo mongosh grain

# View database stats
cd docker && docker-compose exec mongo mongosh --eval "use grain; db.stats()"

# Backup database during testing
cd docker && docker-compose exec mongo mongodump --db grain --archive | gzip > "../logs/test-backup-$(date +%Y%m%d_%H%M%S).gz"
```

## Writing Tests

Integration tests should:

- Test against `http://localhost:8181` (relay endpoint)
- Test against `ws://localhost:8181` (WebSocket endpoint)
- Use `helpers.go` utilities for common test operations
- Clean up any test data created

Example test structure:

```go
func TestBasicConnection(t *testing.T) {
	client := tests.NewTestClient(t)
	defer client.Close()

	t.Log("✅ Successfully connected to relay")
}
```

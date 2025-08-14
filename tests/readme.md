# GRAIN Testing

Integration test suite for GRAIN relay using Docker test environment.

**Note: All commands in this README should be run from the `tests/` directory.**

## Quick Start

Run the full test suite with automatic cleanup:

```bash
make test
```

This will:

1. Start Docker test environment (GRAIN + MongoDB)
2. Wait for services to be ready
3. Run all integration tests
4. **Automatically stop environment and collect logs**

For development work where you want to keep the environment running:

```bash
make test-interactive
```

This provides the old behavior with prompts to keep the environment up.

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
├── integration/           # Integration tests
│   ├── relay_test.go      # Core relay functionality
│   ├── websocket_test.go  # WebSocket connection tests
│   └── api_test.go        # HTTP API tests
└── review/                # Code quality tests
    └── codeQuality_test.go # Code review and standards
```

## Available Commands

| Command                          | Description                                          |
| -------------------------------- | ---------------------------------------------------- |
| `make test`                      | **Complete test cycle** (start, test, stop, cleanup) |
| `make test-interactive`          | Interactive mode (keeps environment running)         |
| `make test-all`                  | Run integration + code review tests with cleanup     |
| `make test-review`               | Run code quality review tests only                   |
| `make test-start`                | Start test environment only                          |
| `make test-run`                  | Run integration tests only                           |
| `make test-single TEST=TestName` | Run specific test function by name                   |
| `make test-file FILE=file.go`    | Run all tests in a single go file                    |
| `make test-stop`                 | Stop environment and collect logs                    |
| `make test-clean-logs`           | Remove all log files                                 |
| `make help`                      | Show all available commands                          |

### Test Modes

**Production/CI Mode** (automatic cleanup):

```bash
make test      # Complete integration tests
make test-all  # Integration + code review tests
```

**Development Mode** (interactive):

```bash
make test-interactive  # Keeps environment up for multiple test runs
```

**Manual Mode** (step-by-step):

```bash
make test-start        # Start environment
make test-run          # Run tests
make test-single TEST=TestName  # Run specific tests
make test-stop         # Clean up when done
```

**Note on test names**: The `TEST=` parameter takes the actual Go test function name (e.g., `TestRelayBasics`, `TestWebSocketConnection`, `TestEventPublishing`).

## Test Types

### Integration Tests

Located in `integration/` directory:

- **relay_test.go** - Core relay functionality and connections
- **websocket_test.go** - WebSocket protocol testing
- **api_test.go** - HTTP API endpoint testing

### Code Quality Tests

Located in `review/` directory:

- **codeQuality_test.go** - Code formatting, linting, and standards
- Automatically run as part of `make test-all`
- Can be run independently with `make test-review`

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
- `test-file-filename-YYYYMMDD_HHMMSS.log` - File-specific test output
- `review-YYYYMMDD_HHMMSS.log` - Code review test results
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

### Viewing GRAIN debug.log

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

## Cross-Platform Compatibility

This test suite works on:

- **Windows** (PowerShell, Command Prompt, Git Bash)
- **macOS** (Terminal, iTerm2)
- **Linux** (Bash, Zsh)

All Makefile commands are cross-platform compatible and handle path differences automatically.

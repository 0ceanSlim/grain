# Development Guide

Developer documentation for building, testing, and releasing GRAIN.

## Development Environment

### Prerequisites

- **Go 1.21+** - [Download Go](https://go.dev/)
- **MongoDB** - [Install MongoDB Community](https://www.mongodb.com/docs/manual/administration/install-community/)
- **Air** (optional) - Live reload during development
  ```bash
  go install github.com/cosmtrek/air@latest
  ```

### Setup

1. **Clone and setup**

   ```bash
   git clone https://github.com/0ceanslim/grain.git
   cd grain
   go mod download
   ```

2. **Start MongoDB**

   ```bash
   # Ubuntu/Debian
   sudo systemctl start mongod

   # macOS with Homebrew
   brew services start mongodb-community

   # Windows
   net start MongoDB
   ```

3. **Run GRAIN**

   ```bash
   # Standard run
   go run .

   # Or with live reload
   air
   ```

## Live Development with Air

Air provides automatic recompilation and restart when source files change.

### Usage

```bash
# Start development server with live reload
air

# Air will watch for changes and automatically:
# - Rebuild the binary
# - Restart the server
# - Preserve configuration hot-reload
```

### Air Benefits

- **Instant feedback** - Changes reflected immediately
- **Configuration preservation** - Hot-reload still works
- **Error handling** - Compilation errors shown in terminal
- **Process management** - Clean shutdown and restart

## Testing

GRAIN includes a comprehensive integration test suite using Docker.

### Quick Test Run

```bash
cd tests/
make test
```

### Test Development

See the [Testing Documentation](../tests/README.md) for:

- Writing integration tests
- Using the test environment
- Running specific tests
- Debugging test failures
- Managing test logs

### Test Environment

- **Isolated containers** - Clean MongoDB and GRAIN instances
- **Realistic scenarios** - WebSocket, HTTP, and database testing
- **Automated cleanup** - No test artifacts left behind
- **Comprehensive logging** - Full test output and application logs

## Building Releases

### Release Target

Use the Makefile for standardized builds:

```bash
# Create release packages
make release
```

### Release Process

```bash
# 1. Version the release
make version VERSION=v1.2.3

# 2. Build all files for release
make release

# 3. Generate checksums
make checksums

# 4. Create GitHub release
make release-github
```

### Build Artifacts

Builds produce:

- **Binary executables** for Linux, Windows, macOS
- **Archive packages** (.tar.gz, .zip)
- **SHA256 checksums** for verification
- **Release metadata** with version info

## Code Standards

### Go Conventions

- **Standard library first** - Avoid unnecessary dependencies
- **Structured logging** - Use log levels and key-value pairs
- **Minimal database calls** - Prefer Nostr REQ over database queries when possible
- **Clear documentation** - Descriptive but not verbose
- **Error handling** - Proper error wrapping and context

### Logging Guidelines

```go
// Good: Structured logging with context
log.Event().Info("Event processed successfully",
    "event_id", evt.ID,
    "kind", evt.Kind,
    "pubkey", evt.PubKey)

// Bad: Unstructured logging
log.Printf("Event %s processed", evt.ID)
```

### Database Best Practices

- **Nostr-first approach** - Use REQ/EVENT messages when possible
- **Efficient queries** - Leverage MongoDB indexing
- **Minimal writes** - Batch operations where appropriate
- **Proper validation** - Check data integrity before database operations

## Configuration Management

### Hot Reload

GRAIN supports configuration hot-reload:

- **File watching** - Automatic detection of config changes
- **Graceful restart** - Seamless configuration updates
- **Validation** - Invalid configs rejected with clear errors

### Configuration Files

- `config.yml` - Server, database, and rate limiting
- `whitelist.yml` - User and content filtering
- `blacklist.yml` - Ban policies and escalation
- `relay_metadata.json` - Public relay information (NIP-11)

## Performance Considerations

### Memory Management

- **Connection limits** - Prevent memory exhaustion
- **Cache strategies** - Efficient pubkey and content caching
- **Garbage collection** - Minimize allocation pressure
- **Resource monitoring** - Track memory and CPU usage

## Debugging

### Development Tools

```bash
# View live logs with pretty printing
tail -f debug.log

# Monitor MongoDB operations
mongosh --eval "db.setLogLevel(2)"
```

### Common Issues

- **MongoDB connection** - Check URI and database accessibility
- **WebSocket errors** - Verify client compatibility and message format
- **Configuration syntax** - YAML parsing errors and validation
- **Rate limiting** - Client requests exceeding configured limits

## Contributing

### Development Workflow

1. **Fork and branch** from `main`
2. **Make changes** with tests
3. **Run test suite** `cd tests && make test`
4. **Submit pull request** with clear description
5. **Address feedback** and iterate

### Code Review

- **Functionality** - Does it work as intended?
- **Performance** - Is it efficient and scalable?
- **Security** - Are there any security implications?
- **Documentation** - Is the code well-documented?
- **Tests** - Are there appropriate tests?

## Resources

- **Nostr Protocol** - [NIPs Repository](https://github.com/nostr-protocol/nips)
- **MongoDB Docs** - [MongoDB Manual](https://www.mongodb.com/docs/manual/)
- **Go Documentation** - [Go Language Docs](https://golang.org/doc/)
- **GRAIN Repository** - [GitHub](https://github.com/0ceanslim/grain)
- **Issue Tracker** - [GitHub Issues](https://github.com/0ceanslim/grain/issues)

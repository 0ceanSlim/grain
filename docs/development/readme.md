# Development Guide

Developer documentation for building, testing, and releasing GRAIN.

## Development Environment

### Prerequisites

- **Docker** - [Install Docker Desktop](https://www.docker.com/products/docker-desktop/)
- **Go 1.21+** (optional) - [Download Go](https://go.dev/) - Only needed for local development
- **MongoDB** (optional) - [Install MongoDB Community](https://www.mongodb.com/docs/manual/administration/install-community/) - Only needed for local development
- **Air** (optional) - Live reload during development
  ```bash
  go install github.com/cosmtrek/air@latest
  ```

### Setup

1. **Clone and setup**

   ```bash
   git clone https://github.com/0ceanslim/grain.git
   cd grain
   ```

2. **For local development** (optional):

   ```bash
   # Download dependencies
   go mod download

   # Start MongoDB
   # Ubuntu/Debian: sudo systemctl start mongod
   # macOS: brew services start mongodb-community
   # Windows: net start MongoDB

   # Run GRAIN locally
   go run .

   # Or with live reload
   air
   ```

## Building Releases

### Docker-based Build System

GRAIN uses Docker containers for consistent, platform-independent builds. Only Docker is required.

```bash
# Navigate to development directory
cd docs/development

# Build release for all platforms
make release
```

This will:

- **Run tests first** - Build stops if tests fail
- **Cross-compile** for Linux, macOS, Windows (x64 and ARM)
- **Bundle www assets** with each binary
- **Create archives** (.tar.gz for Unix, .zip for Windows)
- **Generate checksums** for verification
- **Output to** `build/dist/` directory

### Available Commands

```bash
make help      # Show all available commands
make test      # Run tests only
make release   # Run tests, then build all platforms
make clean     # Clean build artifacts
```

### Build Artifacts

After `make release`, you'll find in `build/dist/`:

- `grain-linux-amd64.tar.gz` - Linux 64-bit
- `grain-linux-arm64.tar.gz` - Linux ARM (Raspberry Pi, etc.)
- `grain-darwin-amd64.tar.gz` - macOS Intel
- `grain-darwin-arm64.tar.gz` - macOS Apple Silicon
- `grain-windows-amd64.zip` - Windows 64-bit
- `checksums.txt` - SHA256 verification hashes

Each archive contains:

- Binary executable (`grain` or `grain.exe`)
- Complete `www/` directory with web assets

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

## Release Process

The new automated build system simplifies releases:

1. **Build everything**: `make release`
2. **Test binaries**: Extract and test key platforms from `build/dist/`
3. **Create GitHub release**: Manually create release on GitHub
4. **Upload artifacts**: Drag and drop all files from `build/dist/`
5. **Write release notes**: Document changes and features manually
6. **Publish**: When ready

### Benefits of Docker Build System

✅ **Platform Independent** - Works on Windows, macOS, Linux  
✅ **Consistent Environment** - Same Go version, same tools everywhere  
✅ **No Local Dependencies** - Only Docker required  
✅ **Reproducible Builds** - Same container = same results  
✅ **Easy CI/CD** - Same process for local and automated builds

## Code Standards

### Go Conventions

- **Standard library first** - Avoid unnecessary dependencies
- **Structured logging** - Use log levels and key-value pairs
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

## Debugging

### Development Tools

```bash
# View live logs with pretty printing
tail -f debug.log

# Monitor MongoDB operations (if running locally)
mongosh --eval "db.setLogLevel(2)"
```

### Common Issues

- **Docker not running** - Ensure Docker Desktop is started
- **MongoDB connection** - Check URI and database accessibility (local dev)
- **WebSocket errors** - Verify client compatibility and message format
- **Configuration syntax** - YAML parsing errors and validation
- **Rate limiting** - Client requests exceeding configured limits

## Contributing

### Development Workflow

1. **Fork and branch** from `main`
2. **Make changes** with tests
3. **Run test suite** `cd tests && make test`
4. **Build and test** `cd docs/development && make release`
5. **Submit pull request** with clear description
6. **Address feedback** and iterate

### Code Review

- **Functionality** - Does it work as intended?
- **Performance** - Is it efficient and scalable?
- **Security** - Are there any security implications?
- **Documentation** - Is the code well-documented?
- **Tests** - Are there appropriate tests?
- **Build** - Does `make release` complete successfully?

## Resources

- **Nostr Protocol** - [NIPs Repository](https://github.com/nostr-protocol/nips)
- **MongoDB Docs** - [MongoDB Manual](https://www.mongodb.com/docs/manual/)
- **Go Documentation** - [Go Language Docs](https://golang.org/doc/)
- **Docker Documentation** - [Docker Docs](https://docs.docker.com/)
- **GRAIN Repository** - [GitHub](https://github.com/0ceanslim/grain)
- **Issue Tracker** - [GitHub Issues](https://github.com/0ceanslim/grain/issues)

# Development Guide

Developer documentation for building, testing, and releasing GRAIN.

## Table of Contents

- [Development Guide](#development-guide)
  - [Table of Contents](#table-of-contents)
  - [Development Environment](#development-environment)
    - [Prerequisites](#prerequisites)
    - [Setup](#setup)
  - [Version Management](#version-management)
    - [Version Detection Logic](#version-detection-logic)
    - [Version Commands](#version-commands)
    - [Version Workflow Examples](#version-workflow-examples)
    - [Semantic Versioning](#semantic-versioning)
  - [Building Releases](#building-releases)
    - [Docker-based Build System](#docker-based-build-system)
    - [Available Commands](#available-commands)
    - [Build Types](#build-types)
    - [Build Artifacts](#build-artifacts)
    - [Version in Binary](#version-in-binary)
  - [Testing](#testing)
    - [Quick Test Run](#quick-test-run)
    - [Test Development](#test-development)
    - [Test Environment](#test-environment)
  - [Release Process](#release-process)
    - [Complete Release Workflow](#complete-release-workflow)
      - [1. Development Phase](#1-development-phase)
      - [2. Pre-release Testing](#2-pre-release-testing)
      - [3. Release Preparation](#3-release-preparation)
      - [4. Create Release](#4-create-release)
      - [5. Publish Release](#5-publish-release)
    - [Release Checklist](#release-checklist)
    - [Benefits of Docker Build System](#benefits-of-docker-build-system)
  - [Code Standards](#code-standards)
    - [Go Conventions](#go-conventions)
    - [Pre-commit](#pre-commit)
    - [Logging Guidelines](#logging-guidelines)
  - [Debugging](#debugging)
    - [Development Tools](#development-tools)
    - [Common Issues](#common-issues)
    - [Troubleshooting Version Issues](#troubleshooting-version-issues)
  - [Contributing](#contributing)
    - [Development Workflow](#development-workflow)
    - [Code Review](#code-review)
    - [Release Guidelines](#release-guidelines)
  - [Resources](#resources)

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

## Version Management

GRAIN uses semantic versioning with automatic version detection from Git tags, plus support for development builds.

### Version Detection Logic

The build system automatically determines the version using this priority:

1. **Environment Variable**: `VERSION=v1.2.3 make release`
2. **Exact Git Tag**: If current commit has a tag (e.g., `v1.2.3`)
3. **Development Version**: `v1.2.3-dev.5` (5 commits since last tag)
4. **Fallback**: `v0.0.0-dev` (no tags found)

### Version Commands

```bash
make version        # Show current version info
make tag VERSION=v1.3.0  # Create new version tag
make prepare-release     # Check if workspace is clean for release
```

### Version Workflow Examples

**Development Builds**:

```bash
# Auto-detect version from git
make release
# Output: v1.2.0-dev.3 (3 commits since v1.2.0 tag)
```

**Release Builds**:

```bash
# Create and push a tag
git tag -a v1.3.0 -m "Release v1.3.0"
git push origin v1.3.0

# Build the release
make release
# Output: v1.3.0 (exact tag match)
```

**Custom Version**:

```bash
# Force a specific version
make release VERSION=v2.0.0-beta.1
```

### Semantic Versioning

GRAIN follows a relaxed approach to [Semantic Versioning](https://semver.org/) suitable for a hobby project:

- **Major** (`v1.0.0`): Reserved for future stable release (no timeline planned)
- **Minor** (`v0.5.0`): New features and significant improvements
- **Patch** (`v0.4.1`): Bug fixes, small improvements, and maintenance

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
make help           # Show all available commands
make version        # Show version information
make test           # Run tests only
make release        # Run tests, then build all platforms
make dev-release    # Quick build without tests
make quick          # Alias for dev-release
make clean          # Clean build artifacts and containers
make prepare-release # Check if workspace is ready for release
```

### Build Types

**Full Release Build**:

```bash
make release
```

- Runs tests first
- Builds all platforms
- Uses proper version detection
- Creates production artifacts

**Development Build**:

```bash
make dev-release
# or
make quick
```

- Skips tests for speed
- Adds `-dev` suffix to version
- Faster iteration during development

**Clean Build**:

```bash
make clean release
```

- Removes all build artifacts
- Rebuilds Docker image
- Full clean build

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

### Version in Binary

The built binary includes version information accessible via:

```bash
# Show version
grain --version
```

Output:

```
GRAIN v1.2.0-dev.3
Go Relay Architecture for Implementing Nostr

Build Information:
  Version:    v1.2.0-dev.3
  Build Time: 2024-07-12T14:30:45Z
  Git Commit: abc1234
  Go Version: go1.23.0
  Platform:   linux/amd64
```

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

### Complete Release Workflow

#### 1. Development Phase

```bash
# Work on features
git commit -m "Add new feature"

# Build and test during development
make dev-release
# Version: v1.2.0-dev.3
```

#### 2. Pre-release Testing

```bash
# Create pre-release tag
git tag -a v0.4.1-pre-release -m "Pre-Release v0.4.1"

# Build pre-release
make release
# Version: v1.3.0-beta.1
```

#### 3. Release Preparation

```bash
# Ensure clean workspace
make prepare-release

# Final full build and test
make release
```

#### 4. Create Release

```bash
# Create release tag
git tag -a v0.4.1 -m "Release v0.4.1"

# Build final release
make release
# Version: v0.4.1
```

#### 5. Publish Release

1. **Test binaries**: Extract and test key platforms from `build/dist/`
2. **Create GitHub release**: Manually create release on GitHub
3. **Upload artifacts**: Drag and drop all files from `build/dist/`
4. **Write release notes**: Document changes and features
5. **Publish**: When ready

### Release Checklist

- [ ] All tests passing (`make test`)
- [ ] Clean workspace (`make prepare-release`)
- [ ] Version tag created and pushed
- [ ] Release build successful (`make release`)
- [ ] Artifacts tested on target platforms
- [ ] GitHub release created with artifacts
- [ ] Release notes written
- [ ] Release published

### Benefits of Docker Build System

✅ **Platform Independent** - Works on Windows, macOS, Linux  
✅ **Consistent Environment** - Same Go version, same tools everywhere  
✅ **No Local Dependencies** - Only Docker required  
✅ **Reproducible Builds** - Same container = same results  
✅ **Easy CI/CD** - Same process for local and automated builds  
✅ **Automatic Versioning** - Smart version detection from Git

## Code Standards

### Go Conventions

- **Standard library first** - Avoid unnecessary dependencies
- **Structured logging** - Use log levels and key-value pairs
- **Clear documentation** - Descriptive but not verbose
- **Error handling** - Proper error wrapping and context

### Pre-commit

You can use this pre-commit script `(.git/hook/pre-commit)` to ensure your code is properly formatted.

```bash
#!/bin/bash

set -e

echo "🔧 Running pre-commit checks..."

# Store initial status
INITIAL_STATUS=$(git status --porcelain)

# Format code
echo "📝 Formatting Go code..."
go fmt ./...
gofmt -s -w .

# Install goimports if not present and format imports
if ! command -v goimports &> /dev/null; then
    echo "📦 Installing goimports..."
    go install golang.org/x/tools/cmd/goimports@latest
fi
goimports -w .

# Tidy modules
echo "🧹 Tidying Go modules..."
go mod tidy
go mod verify

# Check if anything changed
CURRENT_STATUS=$(git status --porcelain)

if [ "$INITIAL_STATUS" != "$CURRENT_STATUS" ]; then
    echo "✨ Code was formatted/tidied. Staging changes..."

    # Stage all changes
    git add .

    # Show what changed
    echo "📋 Changes made:"
    git diff --cached --name-only

    echo "✅ Changes staged and ready for commit."
else
    echo "✅ No formatting or module changes needed."
fi

echo "🎉 Pre-commit checks completed!"
```

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

# Check current version info
make version

# Quick development builds
make quick
```

### Common Issues

- **Docker not running** - Ensure Docker Desktop is started
- **MongoDB connection** - Check URI and database accessibility (local dev)
- **WebSocket errors** - Verify client compatibility and message format
- **Configuration syntax** - YAML parsing errors and validation
- **Rate limiting** - Client requests exceeding configured limits
- **Version not detected** - Check git status and tags (`git tag --list`)
- **Build container conflicts** - Use `make clean` to reset

### Troubleshooting Version Issues

**Version Not Detected**:

```bash
# Check git status
git status
git tag --list

# Create initial tag if none exist
git tag -a v0.1.0 -m "Initial version"
```

**Wrong Version Detected**:

```bash
# Force specific version
make release VERSION=v1.2.3

# Or check git tag issues
git describe --tags --abbrev=0
```

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
- **Versioning** - Are version tags appropriate for the changes?

### Release Guidelines

- **Always tag releases**: Use `git tag` for proper version detection
- **Clean releases**: Use `make prepare-release` to check workspace
- **Test before tagging**: Run `make release` to verify build works
- **Consistent naming**: Use `v` prefix for tags (`v1.2.3`, not `1.2.3`)
- **Development builds**: Use `make quick` for faster iteration

## Resources

- **Nostr Protocol** - [NIPs Repository](https://github.com/nostr-protocol/nips)
- **MongoDB Docs** - [MongoDB Manual](https://www.mongodb.com/docs/manual/)
- **Go Documentation** - [Go Language Docs](https://golang.org/doc/)
- **Docker Documentation** - [Docker Docs](https://docs.docker.com/)
- **Semantic Versioning** - [SemVer Specification](https://semver.org/)
- **GRAIN Repository** - [GitHub](https://github.com/0ceanslim/grain)
- **Issue Tracker** - [GitHub Issues](https://github.com/0ceanslim/grain/issues)

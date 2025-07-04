#!/bin/bash
# GRAIN Docker Build Script
# Runs inside the build container

set -e

# Configuration
APP_NAME="grain"
VERSION="${VERSION:-v0.0.0}"
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS="-w -s -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}"

# Target platforms
PLATFORMS=(
    "linux/amd64"
    "linux/arm64" 
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}GRAIN Docker Build${NC}"
echo "Version: ${VERSION}"
echo "Build Time: ${BUILD_TIME}"
echo "Git Commit: ${GIT_COMMIT}"
echo ""

# Verify we're in the right directory
if [ ! -f "go.mod" ] || [ ! -d "www" ]; then
    echo -e "${RED}Error: Must run from project root with go.mod and www/ directory${NC}"
    exit 1
fi

# Create output directories
mkdir -p /output/dist
mkdir -p /tmp/build

echo -e "${YELLOW}Building for all platforms...${NC}"

# Build for each platform
for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -ra PLATFORM_SPLIT <<< "$platform"
    GOOS="${PLATFORM_SPLIT[0]}"
    GOARCH="${PLATFORM_SPLIT[1]}"
    
    echo -e "  Building ${GOOS}/${GOARCH}..."
    
    # Set binary name
    BINARY="${APP_NAME}"
    if [ "$GOOS" = "windows" ]; then
        BINARY="${APP_NAME}.exe"
    fi
    
    # Archive name
    ARCHIVE="${APP_NAME}-${GOOS}-${GOARCH}"
    
    # Build binary with VCS disabled to avoid Git issues in Docker
    echo -n "    Compiling... "
    if GOOS=$GOOS GOARCH=$GOARCH go build -buildvcs=false -ldflags="$LDFLAGS" -o "/tmp/build/$BINARY" .; then
        echo -e "${GREEN}✓${NC}"
    else
        echo -e "${RED}✗${NC}"
        exit 1
    fi
    
    # Create archive directory
    ARCHIVE_DIR="/tmp/build/$ARCHIVE"
    mkdir -p "$ARCHIVE_DIR"
    
    # Copy files to archive (only binary and www folder)
    echo -n "    Packaging... "
    cp "/tmp/build/$BINARY" "$ARCHIVE_DIR/"
    cp -r www "$ARCHIVE_DIR/"
    
    # Create archive
    cd /tmp/build
    if [ "$GOOS" = "windows" ]; then
        zip -rq "/output/dist/${ARCHIVE}.zip" "$ARCHIVE"
        echo -e "${GREEN}✓ ${ARCHIVE}.zip${NC}"
    else
        tar -czf "/output/dist/${ARCHIVE}.tar.gz" "$ARCHIVE"
        echo -e "${GREEN}✓ ${ARCHIVE}.tar.gz${NC}"
    fi
    
    # Cleanup
    rm -rf "$ARCHIVE_DIR" "/tmp/build/$BINARY"
    cd /app
done

# Generate checksums
echo -e "${YELLOW}Generating checksums...${NC}"
cd /output/dist
for file in *.tar.gz *.zip; do
    if [ -f "$file" ]; then
        sha256sum "$file" >> checksums.txt
    fi
done

echo -e "${GREEN}Build completed!${NC}"
echo ""
echo -e "${BLUE}Release artifacts:${NC}"
ls -la /output/dist/
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Test the binaries from build/dist/"
echo "2. Create GitHub release manually"
echo "3. Upload files from build/dist/"
echo "4. Write release notes"
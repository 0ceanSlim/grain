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

# Dependency URLs and versions
HTMX_VERSION="2.0.4"
HYPERSCRIPT_VERSION="0.9.14"
HTMX_URL="https://unpkg.com/htmx.org@${HTMX_VERSION}/dist/htmx.min.js"
HYPERSCRIPT_URL="https://unpkg.com/hyperscript.org@${HYPERSCRIPT_VERSION}/dist/_hyperscript.min.js"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Download frontend dependencies
download_dependencies() {
    echo -e "${YELLOW}Downloading frontend dependencies...${NC}"
    
    # Create directories for bundled assets - JS only, CSS stays in style
    mkdir -p www/static/js
    
    # Download HTMX
    echo -n "  Downloading HTMX ${HTMX_VERSION}... "
    if wget -q -O www/static/js/htmx.min.js "$HTMX_URL"; then
        echo -e "${GREEN}✓${NC}"
    else
        echo -e "${RED}✗${NC}"
        echo -e "${RED}Failed to download HTMX from: $HTMX_URL${NC}"
        exit 1
    fi
    
    # Download Hyperscript
    echo -n "  Downloading Hyperscript ${HYPERSCRIPT_VERSION}... "
    if wget -q -O www/static/js/hyperscript.min.js "$HYPERSCRIPT_URL"; then
        echo -e "${GREEN}✓${NC}"
    else
        echo -e "${RED}✗${NC}"
        echo -e "${RED}Failed to download Hyperscript from: $HYPERSCRIPT_URL${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}Dependencies downloaded successfully${NC}"
}

# Build TailwindCSS from source
build_css() {
    echo -e "${YELLOW}Building TailwindCSS v4...${NC}"
    
    # Verify TailwindCSS CLI is available
    echo -n "  Checking TailwindCSS CLI... "
    if ! command -v tailwindcss &> /dev/null; then
        echo -e "${RED}✗${NC}"
        echo -e "${RED}TailwindCSS CLI not found${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓${NC}"
    
    # Verify input file exists
    if [ ! -f "www/style/input.css" ]; then
        echo -e "${RED}TailwindCSS source file not found (input.css)${NC}"
        exit 1
    fi
    
    # Build CSS - TailwindCSS v4 with auto content detection
    echo -n "  Compiling TailwindCSS v4... "
    if (cd www/style && tailwindcss -i input.css -o tailwind.min.css --minify); then
        echo -e "${GREEN}✓${NC}"
    else
        echo -e "${RED}✗${NC}"
        echo -e "${RED}Failed to build TailwindCSS${NC}"
        exit 1
    fi
    
    # Verify the output file 
    echo -n "  Verifying CSS build... "
    CSS_SIZE=$(wc -c < www/style/tailwind.min.css)
    echo -e "${GREEN}✓ (${CSS_SIZE} bytes)${NC}"
    
    echo -e "${GREEN}TailwindCSS v4 built successfully${NC}"
}

# Replace CDN URLs with bundled assets in templates
replace_cdn_with_bundled() {
    echo -e "${YELLOW}Updating templates for bundled assets...${NC}"
    
    # Find and process layout.html template
    LAYOUT_FILE="www/views/templates/layout.html"
    if [ ! -f "$LAYOUT_FILE" ]; then
        echo -e "${RED}Layout template not found: $LAYOUT_FILE${NC}"
        exit 1
    fi
    
    echo -n "  Replacing CDN URLs... "
    
    # Replace TailwindCSS CDN with bundled CSS
    sed -i 's|<script src="https://cdn.tailwindcss.com"></script>|<link rel="stylesheet" href="/style/tailwind.min.css">|g' "$LAYOUT_FILE"
    
    # Replace Hyperscript CDN with bundled JS (simple single line pattern)
    sed -i 's|<script src="https://unpkg.com/hyperscript.org@[^"]*"></script>|<script src="/static/js/hyperscript.min.js"></script>|g' "$LAYOUT_FILE"
    
    # Replace HTMX CDN with bundled JS (specific pattern for your multi-line script)
    # Create a temporary file to work with
    cp "$LAYOUT_FILE" "${LAYOUT_FILE}.tmp"
    
    # Use perl for more reliable multi-line replacement
    perl -0pe 's|<script\s+src="https://unpkg\.com/htmx\.org@[^"]*"[^>]*>\s*</script>|<script src="/static/js/htmx.min.js"></script>|gs' "${LAYOUT_FILE}.tmp" > "$LAYOUT_FILE"
    
    # Clean up temp file
    rm "${LAYOUT_FILE}.tmp"
    
    echo -e "${GREEN}✓${NC}"
    echo -e "${GREEN}Templates updated for bundled assets${NC}"
}

# Bundle frontend assets for release
bundle_assets() {
    echo -e "${YELLOW}Bundling frontend assets...${NC}"
    
    # Download dependencies
    download_dependencies
    
    # Build CSS
    build_css
    
    # Update templates
    replace_cdn_with_bundled
    
    echo -e "${GREEN}Frontend assets bundled successfully${NC}"
}

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

# Bundle frontend assets first
bundle_assets

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
    
    # Copy files to archive (binary and www folder with bundled assets)
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
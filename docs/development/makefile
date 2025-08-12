# GRAIN Docker-based Build System
# Platform-independent builds using Docker containers

# Configuration
APP_NAME := grain
PROJECT_ROOT := ../..
BUILD_IMAGE := grain-builder

# Version Management
# 1. Check for VERSION env var first
# 2. Try to get from git tags
# 3. Fall back to default
# Windows-compatible version detection
# Version Management - Windows Compatible
# Version Management - Windows Compatible
ifdef VERSION
    BUILD_VERSION := $(VERSION)
else
    # Check if we're exactly on a tag
    GIT_TAG := $(shell git describe --tags --exact-match 2>nul)
    ifneq ($(GIT_TAG),)
        BUILD_VERSION := $(GIT_TAG)
    else
        # Get the latest tag
        GIT_VERSION := $(shell git describe --tags --abbrev=0 2>nul)
        ifneq ($(GIT_VERSION),)
            GIT_COMMITS := $(shell git rev-list $(GIT_VERSION)..HEAD --count 2>nul)
            ifneq ($(GIT_COMMITS),0)
                BUILD_VERSION := $(GIT_VERSION)-dev.$(GIT_COMMITS)
            else
                BUILD_VERSION := $(GIT_VERSION)
            endif
        else
            BUILD_VERSION := v0.0.0-dev
        endif
    endif
endif

# Colors for output (Windows compatible)
ifeq ($(OS),Windows_NT)
    # Windows - no colors, use echo
    RED := 
    GREEN := 
    YELLOW := 
    BLUE := 
    NC := 
else
    # Unix/Linux/macOS - use colors
    RED := \033[0;31m
    GREEN := \033[0;32m
    YELLOW := \033[1;33m
    BLUE := \033[0;34m
    NC := \033[0m
endif

.PHONY: help clean build-image release test info version check-docker

# Default target
all: release

help: ## Show available targets
	@echo "GRAIN Docker Build System"
	@echo "Platform-independent builds using Docker"
	@echo ""
	@echo "Available targets:"
	@echo "  help             Show this help message"
	@echo "  version          Show version information"
	@echo "  clean            Clean build artifacts"
	@echo "  check-docker     Check if Docker is available"
	@echo "  build-image      Build the Docker build image"
	@echo "  test             Run all tests (integration + code review)"
	@echo "  test-integration Run integration tests only"
	@echo "  test-review      Run code review tests only"
	@echo "  release          Build complete release using Docker"
	@echo "  dev-release      Quick release build for development"
	@echo "  tag              Create a new version tag"
	@echo "  info             Show build information"
	@echo "  prepare-release  Prepare for release (run tests, check git)"
	@echo "  quick            Quick development build (alias for dev-release)"
	@echo ""
	@echo "Testing Options:"
	@echo "  make test                       # Run all tests (integration + review)"
	@echo "  make test-integration           # Run integration tests only"
	@echo "  make test-review                # Run code review tests only"
	@echo ""
	@echo "Version Management:"
	@echo "  make release                    # Auto-detect version from git"
	@echo "  make release VERSION=v1.2.3    # Use specific version"
	@echo "  make version                    # Show current version info"
	@echo ""
	@echo "Current Version: $(BUILD_VERSION)"
	@echo ""
	@echo "Requirements:"
	@echo "  - Docker installed and running"
	@echo "  - No other dependencies needed"

version: ## Show version information
	@echo "$(BLUE)Version Information$(NC)"
	@echo "Build Version: $(BUILD_VERSION)"
	@echo "Git Tag:       (checking...)"
	@git describe --tags --exact-match 2>nul || echo "Git Tag: none"
	@echo "Latest Tag:    (checking...)"
	@git describe --tags --abbrev=0 2>nul || echo "Latest Tag: none"
	@echo "Git Commit:    (checking...)"
	@git rev-parse --short HEAD 2>nul || echo "Git Commit: unknown"
	@echo "Git Status:    (checking git status...)"
	@git status --porcelain 2>nul && echo "Has uncommitted changes" || echo "Status unknown"

clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
ifeq ($(OS),Windows_NT)
	@if exist "$(PROJECT_ROOT)\build" rmdir /s /q "$(PROJECT_ROOT)\build"
	@docker rm -f grain-build-temp 2>nul || echo >nul
	@docker rmi $(BUILD_IMAGE) 2>nul || echo >nul
else
	@rm -rf $(PROJECT_ROOT)/build
	@docker rm -f grain-build-temp 2>/dev/null || true
	@docker rmi $(BUILD_IMAGE) 2>/dev/null || true
endif
	@echo "$(GREEN)Clean completed$(NC)"

check-docker: ## Check if Docker is available
	@echo "$(YELLOW)Checking Docker...$(NC)"
	@docker --version > temp.txt 2>&1 && docker info > temp.txt 2>&1 && echo "$(GREEN)Docker is ready$(NC)" || (echo "$(RED)Error: Docker not available.$(NC)" && exit 1)
	@del temp.txt 2>nul || rm temp.txt 2>/dev/null || true

build-image: check-docker ## Build the Docker build image
	@echo "$(YELLOW)Building Docker build image...$(NC)"
	@cd $(PROJECT_ROOT) && docker build \
		--no-cache \
		-f docs/development/Dockerfile.build \
		-t $(BUILD_IMAGE) \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		.
	@echo "$(GREEN)Build image ready$(NC)"

test: check-docker ## Run all tests (integration + code review)
	@echo "$(YELLOW)Running all tests...$(NC)"
	@echo "$(BLUE)1/2: Running integration tests...$(NC)"
	@cd $(PROJECT_ROOT)/tests && $(MAKE) test-start test-run test-stop
	@echo "$(BLUE)2/2: Running code review tests...$(NC)"
	@cd $(PROJECT_ROOT)/tests && $(MAKE) test-review
	@echo "$(GREEN)All tests completed$(NC)"

test-integration: check-docker ## Run integration tests only
	@echo "$(YELLOW)Running integration tests...$(NC)"
	@cd $(PROJECT_ROOT)/tests && $(MAKE) test
	@echo "$(GREEN)Integration tests completed$(NC)"

test-review: ## Run code review tests only
	@echo "$(YELLOW)Running code review tests...$(NC)"
	@cd $(PROJECT_ROOT)/tests && $(MAKE) test-review
	@echo "$(GREEN)Code review tests completed$(NC)"

release: version test build-image ## Build complete release using Docker (runs tests first)
	@echo "$(BLUE)Starting Docker-based release build$(NC)"
	@echo "$(YELLOW)Building version: $(BUILD_VERSION)$(NC)"
	@echo ""
	@mkdir -p $(PROJECT_ROOT)/build/dist
	@echo "$(YELLOW)Cleaning up any existing build containers...$(NC)"
	@docker rm -f grain-build-temp 2>/dev/null || true
	@echo "$(YELLOW)Running build in container...$(NC)"
	@docker run --name grain-build-temp \
		-e VERSION=$(BUILD_VERSION) \
		$(BUILD_IMAGE)
	@echo "$(YELLOW)Copying artifacts from container...$(NC)"
	@docker cp grain-build-temp:/output/dist/. $(PROJECT_ROOT)/build/dist/
	@docker rm grain-build-temp
	@echo ""
	@echo "$(GREEN)Release build completed!$(NC)"
	@echo "$(YELLOW)Version: $(BUILD_VERSION)$(NC)"
	@echo ""
	@echo "$(BLUE)Artifacts in $(PROJECT_ROOT)/build/dist/:$(NC)"
	@ls -la $(PROJECT_ROOT)/build/dist/ 2>/dev/null || echo "No artifacts found"

# Development shortcuts
dev-release: ## Quick release build for development (skip tests)
	@echo "$(YELLOW)Development release (skipping tests)$(NC)"
	@$(MAKE) build-image
	@mkdir -p $(PROJECT_ROOT)/build/dist 2>nul || mkdir "$(PROJECT_ROOT)\build\dist" 2>nul || echo "Directory exists"
	@docker rm -f grain-build-temp 2>nul || docker rm -f grain-build-temp 2>/dev/null || echo "No temp container to remove"
	@docker run --name grain-build-temp \
		-e VERSION=$(BUILD_VERSION)-dev \
		$(BUILD_IMAGE)
	@docker cp grain-build-temp:/output/dist/. $(PROJECT_ROOT)/build/dist/
	@docker rm grain-build-temp 2>nul || docker rm grain-build-temp 2>/dev/null || echo "Container removed"
	@echo "$(GREEN)Development build completed$(NC)"

# Tag and release workflow
tag: ## Create a new version tag (use: make tag VERSION=v1.2.3)
ifndef VERSION
	@echo "$(RED)Error: VERSION required. Use: make tag VERSION=v1.2.3$(NC)"
	@exit 1
endif
	@echo "$(YELLOW)Creating tag $(VERSION)$(NC)"
	@git tag -a $(VERSION) -m "Release $(VERSION)"
	@echo "$(GREEN)Tag $(VERSION) created. Push with: git push origin $(VERSION)$(NC)"

# Show build information  
info: ## Show build information
	@echo "$(BLUE)Build Information$(NC)"
	@echo "App Name:      $(APP_NAME)"
	@echo "Build Version: $(BUILD_VERSION)"
	@echo "Project Root:  $(PROJECT_ROOT)"
	@echo "Build Image:   $(BUILD_IMAGE)"
	@echo "Docker:        $(shell docker --version 2>/dev/null || echo 'Not installed')"
	@echo "Git Status:    $(shell git status --porcelain 2>/dev/null | wc -l | tr -d ' ') uncommitted changes"

# Workflow helpers
prepare-release: test ## Prepare for release (run tests, check git status)
	@echo "$(BLUE)Preparing for release$(NC)"
	@if [ "$$(git status --porcelain 2>/dev/null | wc -l)" -ne 0 ]; then \
		echo "$(RED)Error: Working directory has uncommitted changes$(NC)"; \
		git status --short; \
		exit 1; \
	fi
	@echo "$(GREEN)Ready for release$(NC)"

quick: dev-release ## Quick development build (alias for dev-release)
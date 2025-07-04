# GRAIN Docker-based Build System
# Platform-independent builds using Docker containers

# Configuration
APP_NAME := grain
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
PROJECT_ROOT := ../..
BUILD_IMAGE := grain-builder

# Colors for output
GREEN := \033[0;32m
YELLOW := \033[1;33m
BLUE := \033[0;34m
NC := \033[0m

.PHONY: help clean build-image release test info

# Default target
all: release

help: ## Show available targets
	@echo "$(BLUE)GRAIN Docker Build System$(NC)"
	@echo "Platform-independent builds using Docker"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(GREEN)%-12s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(YELLOW)Usage:$(NC)"
	@echo "  make release VERSION=v1.2.3"
	@echo "  make test release VERSION=v1.2.3"
	@echo ""
	@echo "$(YELLOW)Requirements:$(NC)"
	@echo "  - Docker installed and running"
	@echo "  - No other dependencies needed"

clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	@rm -rf $(PROJECT_ROOT)/build
	@docker rmi $(BUILD_IMAGE) 2>/dev/null || true
	@echo "$(GREEN)Clean completed$(NC)"

build-image: ## Build the Docker build image
	@echo "$(YELLOW)Building Docker build image...$(NC)"
	@cd $(PROJECT_ROOT) && docker build \
		--no-cache \
		-f docs/development/Dockerfile.build \
		-t $(BUILD_IMAGE) \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		.
	@echo "$(GREEN)Build image ready$(NC)"

test: ## Run tests
	@echo "$(YELLOW)Running tests...$(NC)"
	@cd $(PROJECT_ROOT)/tests && $(MAKE) test
	@echo "$(GREEN)Tests completed$(NC)"

release: test build-image ## Build complete release using Docker (runs tests first)
	@echo "$(BLUE)Starting Docker-based release build$(NC)"
	@echo ""
	@mkdir -p $(PROJECT_ROOT)/build/dist
	@echo "$(YELLOW)Running build in container...$(NC)"
	@docker run --name grain-build-temp \
		$(BUILD_IMAGE)
	@echo "$(YELLOW)Copying artifacts from container...$(NC)"
	@docker cp grain-build-temp:/output/dist/. $(PROJECT_ROOT)/build/dist/
	@docker rm grain-build-temp
	@echo ""
	@echo "$(GREEN)Release build completed!$(NC)"
	@echo ""
	@echo "$(BLUE)Artifacts in $(PROJECT_ROOT)/build/dist/:$(NC)"
	@ls -la $(PROJECT_ROOT)/build/dist/ 2>/dev/null || echo "No artifacts found"

# Quick development cycle
quick: test release ## Quick cycle: test then release

# Show build information  
info: ## Show build information
	@echo "$(BLUE)Build Information$(NC)"
	@echo "App Name:     $(APP_NAME)"
	@echo "Version:      $(VERSION)"
	@echo "Project Root: $(PROJECT_ROOT)"
	@echo "Build Image:  $(BUILD_IMAGE)"
	@echo "Docker:       $(shell docker --version 2>/dev/null || echo 'Not installed')"

# Verify Docker is available
check-docker: ## Check if Docker is available
	@docker --version >/dev/null 2>&1 || (echo "$(RED)Error: Docker not found. Please install Docker.$(NC)" && exit 1)
	@docker info >/dev/null 2>&1 || (echo "$(RED)Error: Docker daemon not running.$(NC)" && exit 1)
	@echo "$(GREEN)Docker is ready$(NC)"
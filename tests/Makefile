DOCKER_COMPOSE ?= docker compose -f docker/docker-compose.yml
TEST_LOG_DIR = logs

# Cross-platform timestamp generation
ifeq ($(OS),Windows_NT)
    TIMESTAMP := $(shell powershell -Command "Get-Date -Format 'yyyyMMdd_HHmmss'")
    MKDIR = if not exist $(TEST_LOG_DIR) mkdir $(TEST_LOG_DIR)
    RM = del /f /q
    SHELL = cmd
else
    TIMESTAMP := $(shell date +%Y%m%d_%H%M%S)
    MKDIR = mkdir -p $(TEST_LOG_DIR)
    RM = rm -f
    # bash, not /bin/sh — dash has no `set -o pipefail`, which we need so
    # `go test ... | tee` doesn't mask test failures as exit 0.
    SHELL := /bin/bash
endif

CONTAINER_NAME = grain-test-relay

.PHONY: help test test-start test-run-all test-run test-single test-stop test-clean-logs

help:
	@echo "Available make targets:"
	@echo "  make test              - Complete test run (start, test, stop, cleanup)"
	@echo "  make test-interactive  - Interactive test mode (keeps environment running)"
	@echo "  make test-all          - Run integration tests + code review tests"
	@echo "  make test-review       - Run code quality review tests only"
	@echo "  make test-start        - Start test Docker environment"
	@echo "  make test-run          - Run integration tests"
	@echo "  make test-single TEST=TestName - Run a specific test by name"
	@echo "  make test-file FILE=filename.go - Run all tests in a specific file"
	@echo "  make test-stop         - Stop environment, collect log, and cleanup images"
	@echo "  make test-clean-logs   - Remove old logs from this directory"

# Main test target - complete self-contained test run
test: test-start test-run test-stop
	@echo ""
	@echo "Test execution completed with cleanup."

# Interactive test mode - keeps environment running for development
test-interactive: test-start test-run
ifeq ($(OS),Windows_NT)
	@echo.
	@echo Test execution completed.
	@echo To stop the test environment and collect logs, run: make test-stop
	@echo To run individual tests, use: make test-single TEST=YourTestName
	@echo To run tests in a specific file, use: make test-file FILE=filename.go
else
	@echo ""
	@echo "Test execution completed."
	@echo -n "Stop the test environment and collect logs? (y/n): "; \
	read answer; \
	if [ "$$answer" = "y" ] || [ "$$answer" = "Y" ]; then \
		$(MAKE) test-stop; \
	else \
		echo "Test environment is still running."; \
		echo "Use 'make test-stop' to stop, collect logs and cleanup the test environment"; \
		echo "Use 'make test-single TEST=YourTestName' to run individual tests."; \
		echo "Use 'make test-file FILE=filename.go' to run all tests in a specific file."; \
	fi
endif

# Start environment
test-start:
	@echo "Generating dynamic config fixtures..."
	@go run ./genconfigs
	@echo "Starting test environment..."
	@$(MKDIR)
	@$(DOCKER_COMPOSE) up -d --build
ifeq ($(OS),Windows_NT)
	@timeout /t 5 /nobreak >nul
else
	@sleep 5
endif
	@echo "Test environment started."

# Windows-specific test runner
test-run-windows:
	@echo "Running all tests..."
	@go test -v ./... > $(TEST_LOG_DIR)\test-results-$(TIMESTAMP).log 2>&1
	@type $(TEST_LOG_DIR)\test-results-$(TIMESTAMP).log

# Unix-specific test runner.
# `set -o pipefail` is required — otherwise `go test ... | tee` always
# exits 0 (tee's status) and masks test failures from make.
test-run-unix:
	@echo "Running all tests..."
	@set -o pipefail; go test -v ./... 2>&1 | tee $(TEST_LOG_DIR)/test-results-$(TIMESTAMP).log

# Simplified test-run target that delegates to platform-specific version
test-run:
ifeq ($(OS),Windows_NT)
	@$(MAKE) test-run-windows
else
	@$(MAKE) test-run-unix
endif

# Cross-platform single test runner
ifeq ($(OS),Windows_NT)
test-single:
	@if "$(TEST)"=="" ( echo Please specify a test: make test-single TEST=TestName & exit /b 1 )
	@echo Running test: $(TEST)
	@go test -v -run $(TEST) ./... > $(TEST_LOG_DIR)\test-$(TEST)-$(TIMESTAMP).log 2>&1
	@type $(TEST_LOG_DIR)\test-$(TEST)-$(TIMESTAMP).log
else
test-single:
	@if [ -z "$(TEST)" ]; then \
		echo "Please specify a test: make test-single TEST=TestName"; \
		exit 1; \
	fi
	@echo "Running test: $(TEST)"
	@go test -v -run $(TEST) ./... 2>&1 | tee $(TEST_LOG_DIR)/test-$(TEST)-$(TIMESTAMP).log
endif

# Simplified test-file (Windows version removes complex bash scripting)
ifeq ($(OS),Windows_NT)
test-file:
	@if "$(FILE)"=="" ( echo Please specify a file: make test-file FILE=relay_test.go & exit /b 1 )
	@echo Running all tests in integration\$(FILE)...
	@go test -v ./integration -run ".*" > $(TEST_LOG_DIR)\test-file-$(FILE)-$(TIMESTAMP).log 2>&1
	@type $(TEST_LOG_DIR)\test-file-$(FILE)-$(TIMESTAMP).log
else
test-file:
	@if [ -z "$(FILE)" ]; then \
		echo "Please specify a file: make test-file FILE=relay_test.go"; \
		echo "Available files:"; \
		ls integration/*.go 2>/dev/null | sed 's/integration\///' || echo "No test files found in integration/"; \
		exit 1; \
	fi
	@if [ ! -f "integration/$(FILE)" ]; then \
		echo "File integration/$(FILE) not found"; \
		echo "Available files:"; \
		ls integration/*.go 2>/dev/null | sed 's/integration\///' || echo "No test files found in integration/"; \
		exit 1; \
	fi
	@echo "Running all tests in integration/$(FILE)..."
	@TESTS=$$(grep -o 'func Test[A-Za-z0-9_]*' integration/$(FILE) | sed 's/func //' | tr '\n' '|' | sed 's/|$$//'); \
	if [ -n "$$TESTS" ]; then \
		echo "Found test functions: $$(echo $$TESTS | tr '|' ' ')"; \
		mkdir -p $(TEST_LOG_DIR); \
		go test -v ./integration -run "$$TESTS" 2>&1 | tee $(TEST_LOG_DIR)/test-file-$(FILE)-$(TIMESTAMP).log; \
	else \
		echo "No test functions found in integration/$(FILE)"; \
		exit 1; \
	fi
endif

# Stop environment and collect logs.
# Uses `-` prefix on commands that legitimately fail (containers already
# exited, no images to remove) so make keeps going and we always get as
# much diagnostic output as possible.
test-stop:
	@echo "Collecting logs from all services..."
	@$(MKDIR)
	-@$(DOCKER_COMPOSE) ps -a > $(TEST_LOG_DIR)/ps-$(TIMESTAMP).log 2>&1
	-@$(DOCKER_COMPOSE) logs -t --no-color > $(TEST_LOG_DIR)/grain-$(TIMESTAMP).log 2>&1
	@echo "--- container status ---"
	-@$(DOCKER_COMPOSE) ps -a
	@echo "--- recent logs (last 80 lines per service) ---"
	-@$(DOCKER_COMPOSE) logs -t --no-color --tail=80
	@echo "Stopping test environment..."
	-@$(DOCKER_COMPOSE) down -v --remove-orphans
	@echo "Logs saved in $(TEST_LOG_DIR)/"

# Clean all logs
test-clean-logs:
	@echo "Removing all logs from $(TEST_LOG_DIR)..."
ifeq ($(OS),Windows_NT)
	@if exist $(TEST_LOG_DIR)\*.log del /q $(TEST_LOG_DIR)\*.log 2>nul || echo "No logs to remove"
else
	@rm -f $(TEST_LOG_DIR)/*.log 2>/dev/null || echo "No logs to remove"
endif
	@echo "All logs removed."

# Cross-platform review tests
ifeq ($(OS),Windows_NT)
test-review:
	@echo "Running code quality review tests..."
	@$(MKDIR)
	@cd .. && if not exist tests\$(TEST_LOG_DIR) mkdir tests\$(TEST_LOG_DIR)
	@cd .. && go test -v ./tests/review > tests\$(TEST_LOG_DIR)\review-$(TIMESTAMP).log 2>&1
	@cd .. && type tests\$(TEST_LOG_DIR)\review-$(TIMESTAMP).log
else
test-review:
	@echo "Running code quality review tests..."
	@mkdir -p $(TEST_LOG_DIR)
	@cd .. && go test -v ./tests/review 2>&1 | tee tests/$(TEST_LOG_DIR)/review-$(TIMESTAMP).log
endif

# Run review tests as part of full test suite
test-all: test-start test-run test-review test-stop
	@echo ""
	@echo "All tests completed (integration + review) with cleanup."
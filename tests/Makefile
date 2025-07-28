DOCKER_COMPOSE = docker-compose -f docker/docker-compose.yml
TEST_LOG_DIR = logs
TIMESTAMP := $(shell date +%Y%m%d_%H%M%S)

CONTAINER_NAME = grain-test-relay
MONGO_CONTAINER = grain-test-mongo

.PHONY: help test test-start test-run-all test-run test-single test-stop test-clean-logs

help:
	@echo "Available make targets:"
	@echo "  make test              			- Start environment, run integration tests, then prompt to stop"
	@echo "  make test-all          			- Run integration tests + code review tests"
	@echo "  make test-review       			- Run code quality review tests only"
	@echo "  make test-start        			- Start test Docker environment"
	@echo "  make test-run          			- Run integration tests"
	@echo "  make test-single TEST=TestName 	- Run a specific test by name"
	@echo "  make test-file FILE=filename.go	- Run all tests in a specific file"
	@echo "  make test-stop       				- Stop environment, collect log, and cleanup images"
	@echo "  make test-clean-logs  				- Remove old logs from this directory"


# Main entry point
test: test-start test-run
	@echo ""
	@echo "Test execution completed."
	@echo -n "Stop the test environment and collect logs? (y/n): "; \
	read answer; \
	if [ "$$answer" = "y" ] || [ "$$answer" = "Y" ]; then \
		$(MAKE) test-stop; \
	else \
		echo "Test environment is still running."; \
		echo "Use 'make test-stop' to stop, collect logs and cleanup the test enviornment"; \
		echo "Use 'make test-single TEST=YourTestName' to run individual tests."; \
		echo "Use 'make test-file TEST=filename.go' to run all tests in a specific file."; \
	fi

# Start environment
test-start:
	@echo "Starting test environment..."
	@mkdir -p $(TEST_LOG_DIR)
	@$(DOCKER_COMPOSE) up -d --build
	@sleep 5
	@echo "Test environment started."

# Run all tests and log output
test-run:
	@echo "Running all tests..."
	@go test -v ./... 2>&1 | tee $(TEST_LOG_DIR)/test-results-$(TIMESTAMP).log

# Run all tests in a specific Go file
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

# Run specific test with TEST=TestName
test-single:
	@if [ -z "$(TEST)" ]; then \
		echo "Please specify a test: make test-single TEST=TestName"; \
		exit 1; \
	fi
	@echo "Running test: $(TEST)"
	@go test -v -run $(TEST) ./... 2>&1 | tee $(TEST_LOG_DIR)/test-$(TEST)-$(TIMESTAMP).log

# Stop environment and collect logs
test-stop:
	@echo "Collecting logs..."
	@mkdir -p $(TEST_LOG_DIR)

	# Container logs
	@$(DOCKER_COMPOSE) logs -t grain > $(TEST_LOG_DIR)/grain-$(TIMESTAMP).log 2>/dev/null || echo "Failed to get grain logs"

	# Application debug log from inside container
	@docker exec $(CONTAINER_NAME) sh -c 'cat /app/debug.log 2>/dev/null || echo "debug.log not found"' > $(TEST_LOG_DIR)/debug-$(TIMESTAMP).log

	@echo "Stopping test environment..."
	@$(DOCKER_COMPOSE) down -v --remove-orphans

	@echo "Removing test images..."
	# Remove Docker images labeled for test
	@docker rmi -f $$(docker images -q --filter "label=test-image=grain") 2>/dev/null || true


	@echo "Logs saved in $(TEST_LOG_DIR)/"


# Clean all logs
test-clean-logs:
	@echo "Removing all logs from $(TEST_LOG_DIR)..."
	@rm -f $(TEST_LOG_DIR)/*.log 2>/dev/null || true
	@echo "All logs removed."

# Review tests for code quality and standards
test-review:
	@echo "Running code quality review tests..."
	@mkdir -p $(TEST_LOG_DIR)
	@cd .. && go test -v ./tests/review 2>&1 | tee tests/$(TEST_LOG_DIR)/review-$(TIMESTAMP).log

# Run review tests as part of full test suite
test-all: test-start test-run test-review
	@echo ""
	@echo "All tests completed (integration + review)."
	@echo -n "Stop the test environment and collect logs? (y/n): "; \
	read answer; \
	if [ "$$answer" = "y" ] || [ "$$answer" = "Y" ]; then \
		$(MAKE) test-stop; \
	else \
		echo "Test environment is still running."; \
		echo "Use 'make test-stop' to stop and cleanup"; \
	fi
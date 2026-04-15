.PHONY: build test clean install lint docker-test

BINARY_NAME=go-agent
CMD_PATH=./cmd/go-agent
BUILD_DIR=./build

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)

install:
	@echo "Installing $(BINARY_NAME)..."
	go install $(CMD_PATH)

test:
	@echo "Running tests..."
	go test -v -cover ./...

lint:
	@echo "Running linter..."
	golangci-lint run ./...

clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR) .go-agent/workflows

init:
	$(BUILD_DIR)/$(BINARY_NAME) init

# Example: Run a feature workflow
run-example:
	$(BUILD_DIR)/$(BINARY_NAME) create-feature "Add restaurant search endpoint" -i

docker-test:
	@echo "Testing Docker sandbox..."
	@docker run --rm golang:1.23-alpine sh -c "echo Docker available"

# Development helpers
dev-init:
	go mod tidy
dev-run:
	go run $(CMD_PATH) $(ARGS)

.DEFAULT_GOAL := build

.PHONY: build clean test run docker-build docker-run fmt lint help

# Variables
APP_NAME = chat-bot
BUILD_DIR = ./bin
MAIN_PATH = ./cmd/server
CONFIG_PATH = ./config
DATA_DIR = ./data

# Ensure data directory exists
$(shell mkdir -p $(DATA_DIR))

# Go commands
GO = go
GOFLAGS = -v
GOBUILD = $(GO) build $(GOFLAGS)
GOTEST = $(GO) test $(GOFLAGS)
GOMOD = $(GO) mod
GOFMT = $(GO) fmt
GOVET = $(GO) vet

# Docker commands
DOCKER = docker
DOCKER_COMPOSE = docker-compose

# Help target
help:
	@echo "Available targets:"
	@echo "  build          - Build the application"
	@echo "  run            - Run the application locally"
	@echo "  test           - Run tests"
	@echo "  clean          - Remove build artifacts"
	@echo "  fmt            - Format Go code"
	@echo "  lint           - Run go vet and other linters"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run application in Docker"
	@echo "  docker-stop    - Stop Docker container"
	@echo "  docker-logs    - Show Docker container logs"
	@echo "  mod-tidy       - Tidy Go modules"
	@echo "  prepare-config - Create config.json from sample if not exists"

# Default target
all: clean build

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)
	@echo "Build successful! Binary located at $(BUILD_DIR)/$(APP_NAME)"

# Run the application
run: prepare-config
	@echo "Running $(APP_NAME)..."
	$(GO) run $(MAIN_PATH)

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# Lint code
lint:
	@echo "Linting code..."
	$(GOVET) ./...
	@echo "Lint complete"

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	$(DOCKER) build -t vibin/$(APP_NAME) .

# Run Docker container
docker-run: prepare-config
	@echo "Starting Docker container..."
	$(DOCKER_COMPOSE) up -d

# Stop Docker container
docker-stop:
	@echo "Stopping Docker container..."
	$(DOCKER_COMPOSE) down

# Show Docker logs
docker-logs:
	@echo "Showing Docker logs..."
	$(DOCKER_COMPOSE) logs -f

# Tidy Go modules
mod-tidy:
	@echo "Tidying Go modules..."
	$(GOMOD) tidy
	@echo "Modules tidied!"

# Prepare config file
prepare-config:
	@if [ ! -f $(CONFIG_PATH)/config.json ]; then \
		echo "Creating config.json from sample..."; \
		cp $(CONFIG_PATH)/config.sample.json $(CONFIG_PATH)/config.json; \
		echo "Please update $(CONFIG_PATH)/config.json with your API keys and settings"; \
	fi

# Default target when user types "make"
.DEFAULT_GOAL := help

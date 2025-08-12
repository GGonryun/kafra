.PHONY: build clean test help

# Build configuration
BINARY_NAME=p0-ssh-agent
BUILD_DIR=bin
CMD_CLIENT_DIR=./cmd/client

# Go build flags
LDFLAGS=-ldflags="-s -w"
BUILD_FLAGS=-v $(LDFLAGS)

# Default target
all: build

# Build the main binary with subcommands
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_CLIENT_DIR)

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	go clean

# Install binary to /usr/local/bin
install: build
	@echo "Installing binary to /usr/local/bin..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

# Uninstall binary from /usr/local/bin
uninstall:
	@echo "Uninstalling binary from /usr/local/bin..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)

# Development build (no optimization)
dev:
	@echo "Building development version..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_CLIENT_DIR)

# Show help
help:
	@echo "Available targets:"
	@echo "  build     - Build p0-ssh-agent binary with subcommands (default)"
	@echo "  deps      - Install Go module dependencies"
	@echo "  test      - Run tests"
	@echo "  clean     - Remove build artifacts"
	@echo "  install   - Install binary to /usr/local/bin (requires sudo)"
	@echo "  uninstall - Remove binary from /usr/local/bin (requires sudo)"
	@echo "  dev       - Build development version without optimization"
	@echo "  help      - Show this help message"
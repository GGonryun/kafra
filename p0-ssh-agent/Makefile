.PHONY: build clean test help build-linux build-all-platforms build-ubuntu build-debian build-centos build-fedora build-arch build-alpine build-nixos

# Build configuration
BINARY_NAME=p0-ssh-agent
BUILD_DIR=bin
DIST_DIR=dist
CMD_DIR=./cmd
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Go build flags
LDFLAGS=-ldflags="-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"
BUILD_FLAGS=-v $(LDFLAGS)

# Cross-compilation targets
LINUX_TARGETS=linux/amd64 linux/arm64 linux/386 linux/arm

# Default target
all: build

# Build the unified binary (start, keygen, register commands)
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

# Build for Linux distributions (all architectures)
build-linux:
	@echo "Building $(BINARY_NAME) for Linux distributions..."
	@mkdir -p $(DIST_DIR)
	@for target in $(LINUX_TARGETS); do \
		os=$$(echo $$target | cut -d'/' -f1); \
		arch=$$(echo $$target | cut -d'/' -f2); \
		echo "Building for $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build $(BUILD_FLAGS) \
			-o $(DIST_DIR)/$(BINARY_NAME)-$$os-$$arch $(CMD_DIR); \
	done

# Build for specific distributions with optimized static binaries
build-ubuntu:
	@echo "Building $(BINARY_NAME) for Ubuntu (amd64, arm64)..."
	@mkdir -p $(DIST_DIR)/ubuntu
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) \
		-o $(DIST_DIR)/ubuntu/$(BINARY_NAME)-ubuntu-amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(BUILD_FLAGS) \
		-o $(DIST_DIR)/ubuntu/$(BINARY_NAME)-ubuntu-arm64 $(CMD_DIR)

build-debian:
	@echo "Building $(BINARY_NAME) for Debian (amd64, arm64, arm)..."
	@mkdir -p $(DIST_DIR)/debian
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) \
		-o $(DIST_DIR)/debian/$(BINARY_NAME)-debian-amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(BUILD_FLAGS) \
		-o $(DIST_DIR)/debian/$(BINARY_NAME)-debian-arm64 $(CMD_DIR)
	GOOS=linux GOARCH=arm CGO_ENABLED=0 go build $(BUILD_FLAGS) \
		-o $(DIST_DIR)/debian/$(BINARY_NAME)-debian-arm $(CMD_DIR)

build-centos:
	@echo "Building $(BINARY_NAME) for CentOS/RHEL (amd64, arm64)..."
	@mkdir -p $(DIST_DIR)/centos
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) \
		-o $(DIST_DIR)/centos/$(BINARY_NAME)-centos-amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(BUILD_FLAGS) \
		-o $(DIST_DIR)/centos/$(BINARY_NAME)-centos-arm64 $(CMD_DIR)

build-fedora:
	@echo "Building $(BINARY_NAME) for Fedora (amd64, arm64)..."
	@mkdir -p $(DIST_DIR)/fedora
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) \
		-o $(DIST_DIR)/fedora/$(BINARY_NAME)-fedora-amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(BUILD_FLAGS) \
		-o $(DIST_DIR)/fedora/$(BINARY_NAME)-fedora-arm64 $(CMD_DIR)

build-arch:
	@echo "Building $(BINARY_NAME) for Arch Linux (amd64, arm64)..."
	@mkdir -p $(DIST_DIR)/arch
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) \
		-o $(DIST_DIR)/arch/$(BINARY_NAME)-arch-amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(BUILD_FLAGS) \
		-o $(DIST_DIR)/arch/$(BINARY_NAME)-arch-arm64 $(CMD_DIR)

build-alpine:
	@echo "Building $(BINARY_NAME) for Alpine Linux (amd64, arm64, arm)..."
	@mkdir -p $(DIST_DIR)/alpine
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) \
		-tags 'osusergo netgo static_build' \
		-o $(DIST_DIR)/alpine/$(BINARY_NAME)-alpine-amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(BUILD_FLAGS) \
		-tags 'osusergo netgo static_build' \
		-o $(DIST_DIR)/alpine/$(BINARY_NAME)-alpine-arm64 $(CMD_DIR)
	GOOS=linux GOARCH=arm CGO_ENABLED=0 go build $(BUILD_FLAGS) \
		-tags 'osusergo netgo static_build' \
		-o $(DIST_DIR)/alpine/$(BINARY_NAME)-alpine-arm $(CMD_DIR)

build-nixos:
	@echo "Building $(BINARY_NAME) for NixOS (amd64, arm64)..."
	@mkdir -p $(DIST_DIR)/nixos
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) \
		-tags 'osusergo netgo static_build' \
		-o $(DIST_DIR)/nixos/$(BINARY_NAME)-nixos-amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(BUILD_FLAGS) \
		-tags 'osusergo netgo static_build' \
		-o $(DIST_DIR)/nixos/$(BINARY_NAME)-nixos-arm64 $(CMD_DIR)

# Build for all supported platforms and distributions
build-all-platforms: build-ubuntu build-debian build-centos build-fedora build-arch build-alpine build-nixos
	@echo "Built binaries for all supported Linux distributions"
	@echo "Binaries available in $(DIST_DIR)/"

# Create distribution packages
package-all: build-all-platforms
	@echo "Creating distribution packages..."
	@mkdir -p $(DIST_DIR)/packages
	@cd $(DIST_DIR) && for distro in ubuntu debian centos fedora arch alpine nixos; do \
		if [ -d "$$distro" ]; then \
			tar -czf packages/$(BINARY_NAME)-$$distro.tar.gz $$distro/; \
			echo "Created $(DIST_DIR)/packages/$(BINARY_NAME)-$$distro.tar.gz"; \
		fi; \
	done

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
	rm -rf $(BUILD_DIR) $(DIST_DIR)
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
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

# Show help
help:
	@echo "Available targets:"
	@echo "  build              - Build unified p0-ssh-agent binary (default)"
	@echo "  build-linux        - Build for all Linux architectures (amd64, arm64, 386, arm)"
	@echo "  build-ubuntu       - Build optimized binaries for Ubuntu"
	@echo "  build-debian       - Build optimized binaries for Debian"
	@echo "  build-centos       - Build optimized binaries for CentOS/RHEL"
	@echo "  build-fedora       - Build optimized binaries for Fedora"
	@echo "  build-arch         - Build optimized binaries for Arch Linux"
	@echo "  build-alpine       - Build static binaries for Alpine Linux"
	@echo "  build-nixos        - Build static binaries for NixOS"
	@echo "  build-all-platforms- Build for all supported Linux distributions"
	@echo "  package-all        - Create tar.gz packages for all distributions"
	@echo "  deps               - Install Go module dependencies"
	@echo "  test               - Run tests"
	@echo "  clean              - Remove build artifacts and distribution files"
	@echo "  install            - Install binary to /usr/local/bin (requires sudo)"
	@echo "  uninstall          - Remove binary from /usr/local/bin (requires sudo)"
	@echo "  dev                - Build development version without optimization"
	@echo "  help               - Show this help message"
	@echo ""
	@echo "Distribution-specific builds support multiple architectures:"
	@echo "  - Ubuntu/Debian: amd64, arm64, arm (Debian only)"
	@echo "  - CentOS/Fedora/Arch: amd64, arm64"
	@echo "  - Alpine/NixOS: amd64, arm64, arm (Alpine only) - fully static builds"
	@echo ""
	@echo "All binaries are built with CGO_ENABLED=0 for maximum compatibility."
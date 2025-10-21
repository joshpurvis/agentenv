# Makefile for agentenv

.PHONY: build install clean test help

# Build variables
BINARY_NAME=agentenv
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X github.com/joshpurvis/agentenv/cmd.Version=$(VERSION) -X github.com/joshpurvis/agentenv/cmd.BuildDate=$(BUILD_DATE) -X github.com/joshpurvis/agentenv/cmd.GitCommit=$(GIT_COMMIT)"

# Installation paths
INSTALL_PATH=/usr/local/bin

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) .
	@echo "Built bin/$(BINARY_NAME) (version: $(VERSION))"

## install: Install the binary to $(INSTALL_PATH)
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	@sudo cp bin/$(BINARY_NAME) $(INSTALL_PATH)/
	@echo "Installed successfully!"
	@echo "Run '$(BINARY_NAME) --help' to get started"

## uninstall: Remove the binary from $(INSTALL_PATH)
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Uninstalled successfully!"

## clean: Remove build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin
	@go clean
	@echo "Clean complete!"

## test: Run tests
test:
	@echo "Running tests..."
	go test -v ./...

## fmt: Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

## mod: Tidy go modules
mod:
	@echo "Tidying go modules..."
	go mod tidy

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/  /'

.DEFAULT_GOAL := help

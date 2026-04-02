.PHONY: build run test clean fmt help

# Binary name
BINARY_NAME=lazylinear

# Default target
all: build

build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) main.go

run:
	@echo "Running $(BINARY_NAME)..."
	go run main.go

test:
	@echo "Running tests..."
	go test ./...

clean:
	@echo "Cleaning up..."
	go clean
	rm -f $(BINARY_NAME)

fmt:
	@echo "Formatting code..."
	go fmt ./...

help:
	@echo "Available targets:"
	@echo "  make build  - Build the binary"
	@echo "  make run    - Run the application directly"
	@echo "  make test   - Run all tests"
	@echo "  make clean  - Remove binary and clean build cache"
	@echo "  make fmt    - Run go fmt"
	@echo "  make help   - Show this help message"

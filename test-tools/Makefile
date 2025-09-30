.PHONY: build run-producer run-consumer test bench clean deps install help

# Variables
BINARY_DIR=bin
PRODUCER_BINARY=$(BINARY_DIR)/producer
CONSUMER_BINARY=$(BINARY_DIR)/consumer
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags "-s -w"

help: ## Display this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

deps: ## Download dependencies
	$(GO) mod download
	$(GO) mod verify

build: deps ## Build both producer and consumer binaries
	@mkdir -p $(BINARY_DIR)
	@echo "Building producer..."
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(PRODUCER_BINARY) ./cmd/producer
	@echo "Building consumer..."
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(CONSUMER_BINARY) ./cmd/consumer
	@echo "Build complete: $(PRODUCER_BINARY) and $(CONSUMER_BINARY)"

build-producer: deps ## Build only the producer binary
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(PRODUCER_BINARY) ./cmd/producer

build-consumer: deps ## Build only the consumer binary
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(CONSUMER_BINARY) ./cmd/consumer

install: ## Install binaries to $GOPATH/bin
	$(GO) install $(GOFLAGS) $(LDFLAGS) ./cmd/producer
	$(GO) install $(GOFLAGS) $(LDFLAGS) ./cmd/consumer

run-producer: build-producer ## Build and run the producer
	$(PRODUCER_BINARY)

run-consumer: build-consumer ## Build and run the consumer
	$(CONSUMER_BINARY)

test: ## Run tests
	$(GO) test -v ./...

test-coverage: ## Run tests with coverage
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

bench: ## Run benchmarks
	$(GO) test -bench=. -benchmem ./...

clean: ## Remove build artifacts
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html
	$(GO) clean

fmt: ## Format code
	$(GO) fmt ./...

vet: ## Run go vet
	$(GO) vet ./...

lint: ## Run golangci-lint (requires golangci-lint installed)
	golangci-lint run

tidy: ## Tidy go.mod
	$(GO) mod tidy

all: clean deps fmt vet test build ## Run all checks and build
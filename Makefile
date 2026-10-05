.PHONY: help build install clean test cover fmt vet check
.DEFAULT_GOAL := help

BINARY_NAME=tk

help: ## Show this help message
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

build: ## Build the tk binary
	go build -o $(BINARY_NAME) .

install: ## Install tk into GOPATH/bin
	go install

clean: ## Remove build artifacts
	go clean
	rm -f $(BINARY_NAME) coverage.out

test: ## Run all tests with the race detector
	go test -race ./...

cover: ## Run tests with a coverage summary
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

fmt: ## Format code
	gofmt -w .

vet: ## Run go vet
	go vet ./...

check: fmt vet test ## Format, vet and test

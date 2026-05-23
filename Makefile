# Makefile — developer commands for local work.
# Run any target with: make <target>
# Example: make up  make test  make lint

.PHONY: up down test lint vet proto clean help

## ── Local development ────────────────────────────────────────────────────────

up: ## Start the full stack (HAProxy + KrakenD + Go service)
	docker compose up --build

down: ## Stop and remove all containers
	docker compose down

logs: ## Tail logs from all containers
	docker compose logs -f

## ── Go service ───────────────────────────────────────────────────────────────

deps: ## Generate go.sum (run this once after cloning)
	cd service && go mod tidy

test: ## Run unit tests with race detector (same flags as CI)
	cd service && go test -race -covermode=atomic -coverprofile=coverage.out ./...
	cd service && go tool cover -func=coverage.out | tail -1

test-verbose: ## Run tests with verbose output
	cd service && go test -race -v ./...

vet: ## Run go vet (catches suspicious constructs)
	cd service && go vet ./...

lint: ## Run golangci-lint (requires: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	cd service && golangci-lint run ./...

fmt: ## Format all Go files
	cd service && gofmt -w .

build: ## Build the binary locally (not Docker)
	cd service && CGO_ENABLED=0 go build -o /tmp/demo-service ./cmd/server

## ── Proto generation ─────────────────────────────────────────────────────────
# Run this to regenerate Go code from user.proto.
# Requires: protoc, protoc-gen-go, protoc-gen-go-grpc
#   brew install protobuf
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto: ## Generate Go code from proto/user.proto
	protoc \
		--go_out=service \
		--go_opt=paths=source_relative \
		--go-grpc_out=service \
		--go-grpc_opt=paths=source_relative \
		-I service \
		service/proto/user.proto
	@echo "Generated proto files. You can now delete service/internal/handler/grpc.go and use the generated stubs."

## ── Security (run locally before pushing) ────────────────────────────────────

secret-scan: ## Scan for committed secrets (requires: go install github.com/zricethezav/gitleaks/v8@latest)
	gitleaks detect --source . --no-git --redact

vuln-check: ## Check for known vulnerabilities in dependencies
	cd service && go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...

## ── Utilities ────────────────────────────────────────────────────────────────

clean: ## Remove build artifacts
	rm -f service/coverage.out
	rm -f /tmp/demo-service

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

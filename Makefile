# SecuRizon Makefile

.PHONY: help build test clean dev-setup dev-start dev-stop docker-build docker-run

# Default target
help:
	@echo "SecuRizon - Real-time Security Posture Management Platform"
	@echo ""
	@echo "Available targets:"
	@echo "  help         - Show this help message"
	@echo "  build        - Build all binaries"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts"
	@echo "  dev-setup    - Setup development environment"
	@echo "  dev-start    - Start development services"
	@echo "  dev-stop     - Stop development services"
	@echo "  docker-build - Build Docker images"
	@echo "  docker-run   - Run with Docker Compose"
	@echo "  lint         - Run linter"
	@echo "  format       - Format code"
	@echo "  deps         - Download dependencies"

# Variables
BINARY_NAME=securizon
VERSION=$(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Build targets
build:
	@echo "Building SecuRizon..."
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/securizon
	go build $(LDFLAGS) -o bin/collector-aws ./cmd/collectors/aws
	go build $(LDFLAGS) -o bin/collector-azure ./cmd/collectors/azure
	go build $(LDFLAGS) -o bin/collector-gcp ./cmd/collectors/gcp

build-linux:
	@echo "Building SecuRizon for Linux..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/securizon
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/collector-aws-linux-amd64 ./cmd/collectors/aws
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/collector-azure-linux-amd64 ./cmd/collectors/azure
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/collector-gcp-linux-amd64 ./cmd/collectors/gcp

# Development targets
dev-setup:
	@echo "Setting up development environment..."
	go mod download
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/air-verse/air@latest

dev-start:
	@echo "Starting development services..."
	docker-compose -f deployments/docker/docker-compose.dev.yml up -d
	@echo "Waiting for services to be ready..."
	sleep 10
	@echo "Starting development server..."
	air -c .air.toml

dev-stop:
	@echo "Stopping development services..."
	docker-compose -f deployments/docker/docker-compose.dev.yml down
	pkill -f air || true

# Testing
test:
	@echo "Running tests..."
	go test -v ./...

test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-integration:
	@echo "Running integration tests..."
	go test -v -tags=integration ./...

# Code quality
lint:
	@echo "Running linter..."
	golangci-lint run

format:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Docker targets
docker-build:
	@echo "Building Docker images..."
	docker build -t securizon:latest -f deployments/docker/Dockerfile .
	docker build -t securizon/collector-aws:latest -f deployments/docker/Dockerfile.aws ./cmd/collectors/aws
	docker build -t securizon/collector-azure:latest -f deployments/docker/Dockerfile.azure ./cmd/collectors/azure
	docker build -t securizon/collector-gcp:latest -f deployments/docker/Dockerfile.gcp ./cmd/collectors/gcp

docker-run:
	@echo "Running SecuRizon with Docker Compose..."
	docker-compose -f deployments/docker/docker-compose.yml up -d

docker-stop:
	@echo "Stopping Docker Compose..."
	docker-compose -f deployments/docker/docker-compose.yml down

# Cleanup
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean -cache

# Installation
install: build
	@echo "Installing SecuRizon..."
	sudo cp bin/$(BINARY_NAME) /usr/local/bin/

uninstall:
	@echo "Uninstalling SecuRizon..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)

# Documentation
docs:
	@echo "Generating documentation..."
	godoc -http=:6060

# Security
security-scan:
	@echo "Running security scan..."
	gosec ./...

# Performance
benchmark:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Release
release: clean test build
	@echo "Creating release..."
	mkdir -p release
	tar -czf release/securizon-$(VERSION).tar.gz bin/ README.md LICENSE

# Database
db-migrate:
	@echo "Running database migrations..."
	# Add migration logic here

db-seed:
	@echo "Seeding database..."
	# Add seeding logic here

# Configuration
config-validate:
	@echo "Validating configuration..."
	# Add config validation logic here

# Monitoring
metrics:
	@echo "Starting metrics collection..."
	# Add metrics collection logic here

# CI/CD helpers
ci: lint test security-scan
	@echo "CI pipeline completed successfully"

cd: docker-build
	@echo "CD pipeline completed successfully"

# Development shortcuts
run: build
	./bin/$(BINARY_NAME)

debug: build
	dlv --listen=:4000 --headless=true --api-version=2 exec ./bin/$(BINARY_NAME)

# Quick start for new developers
quick-start: dev-setup dev-start
	@echo "SecuRizon development environment is ready!"
	@echo "API: http://localhost:8080"
	@echo "Neo4j: http://localhost:7474"
	@echo "Kafka: localhost:9092"

# Production deployment
deploy-staging:
	@echo "Deploying to staging..."
	# Add staging deployment logic here

deploy-production:
	@echo "Deploying to production..."
	# Add production deployment logic here

# Backup and restore
backup:
	@echo "Creating backup..."
	# Add backup logic here

restore:
	@echo "Restoring from backup..."
	# Add restore logic here

# Health checks
health-check:
	@echo "Checking system health..."
	curl -f http://localhost:8080/api/v1/health || exit 1

# Load testing
load-test:
	@echo "Running load tests..."
	# Add load testing logic here

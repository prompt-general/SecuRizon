#!/bin/bash

# SecuRizon Development Setup Script
# This script sets up the complete development environment

set -e

echo "🚀 Setting up SecuRizon development environment..."

# Check prerequisites
check_prerequisites() {
    echo "📋 Checking prerequisites..."
    
    if ! command -v go &> /dev/null; then
        echo "❌ Go is not installed. Please install Go 1.21 or higher."
        exit 1
    fi
    
    if ! command -v docker &> /dev/null; then
        echo "❌ Docker is not installed. Please install Docker."
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        echo "❌ Docker Compose is not installed. Please install Docker Compose."
        exit 1
    fi
    
    echo "✅ Prerequisites check passed"
}

# Install Go dependencies
install_dependencies() {
    echo "📦 Installing Go dependencies..."
    go mod download
    go mod tidy
    
    # Install development tools
    echo "🛠️ Installing development tools..."
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    go install github.com/air-verse/air@latest
    go install github.com/swaggo/swag/cmd/swag@latest
    
    echo "✅ Dependencies installed"
}

# Setup Docker environment
setup_docker() {
    echo "🐳 Setting up Docker environment..."
    
    # Create docker network if it doesn't exist
    docker network create securizon-network 2>/dev/null || true
    
    # Start development services
    echo "🚀 Starting development services..."
    docker-compose -f deployments/docker/docker-compose.dev.yml up -d
    
    # Wait for services to be ready
    echo "⏳ Waiting for services to be ready..."
    sleep 30
    
    # Check service health
    check_service_health
    
    echo "✅ Docker environment setup complete"
}

# Check service health
check_service_health() {
    echo "🔍 Checking service health..."
    
    # Check Neo4j
    if curl -f http://localhost:7474 &>/dev/null; then
        echo "✅ Neo4j is healthy"
    else
        echo "❌ Neo4j is not ready"
    fi
    
    # Check Kafka
    if docker exec securizon-kafka kafka-broker-api-versions --bootstrap-server localhost:9092 &>/dev/null; then
        echo "✅ Kafka is healthy"
    else
        echo "❌ Kafka is not ready"
    fi
    
    # Check Redis
    if docker exec securizon-redis redis-cli ping &>/dev/null; then
        echo "✅ Redis is healthy"
    else
        echo "❌ Redis is not ready"
    fi
}

# Setup development configuration
setup_config() {
    echo "⚙️ Setting up development configuration..."
    
    # Create config directory if it doesn't exist
    mkdir -p config
    
    # Copy development configuration
    if [ ! -f config/config.yaml ]; then
        echo "📝 Creating development configuration..."
        cp config/config.yaml.example config/config.yaml 2>/dev/null || echo "Using default config"
    fi
    
    echo "✅ Configuration setup complete"
}

# Run initial tests
run_tests() {
    echo "🧪 Running initial tests..."
    
    # Run unit tests
    if go test ./...; then
        echo "✅ Unit tests passed"
    else
        echo "❌ Unit tests failed"
        exit 1
    fi
    
    # Run linter
    if command -v golangci-lint &> /dev/null; then
        if golangci-lint run; then
            echo "✅ Linter checks passed"
        else
            echo "❌ Linter checks failed"
            exit 1
        fi
    fi
    
    echo "✅ All tests passed"
}

# Setup git hooks
setup_git_hooks() {
    echo "🔧 Setting up git hooks..."
    
    # Create pre-commit hook
    cat > .git/hooks/pre-commit << 'EOF'
#!/bin/bash
# Pre-commit hook for SecuRizon

# Run tests
go test ./...

# Run linter
if command -v golangci-lint &> /dev/null; then
    golangci-lint run
fi

# Check gofmt
if [ "$(gofmt -s -l . | wc -l)" -gt 0 ]; then
    echo "❌ Code is not formatted. Please run 'go fmt ./...'"
    exit 1
fi

echo "✅ Pre-commit checks passed"
EOF
    
    chmod +x .git/hooks/pre-commit
    
    echo "✅ Git hooks setup complete"
}

# Print development information
print_dev_info() {
    echo ""
    echo "🎉 SecuRizon development environment is ready!"
    echo ""
    echo "📊 Development Services:"
    echo "  - Neo4j: http://localhost:7474 (neo4j/password)"
    echo "  - Kafka: localhost:9092"
    echo "  - Redis: localhost:6379"
    echo "  - Grafana: http://localhost:3000 (admin/admin)"
    echo "  - Prometheus: http://localhost:9090"
    echo "  - Jaeger: http://localhost:16686"
    echo ""
    echo "🚀 Quick Commands:"
    echo "  make dev-start    - Start development server"
    echo "  make test         - Run tests"
    echo "  make lint         - Run linter"
    echo "  make docker-run   - Run with Docker"
    echo ""
    echo "📚 Documentation:"
    echo "  - README.md - Project overview"
    echo "  - CONTRIBUTING.md - Contribution guidelines"
    echo "  - docs/architecture.md - Architecture documentation"
    echo ""
}

# Main execution
main() {
    echo "🔧 SecuRizon Development Setup"
    echo "================================"
    
    check_prerequisites
    install_dependencies
    setup_docker
    setup_config
    run_tests
    setup_git_hooks
    print_dev_info
    
    echo "✅ Development setup complete!"
}

# Handle script interruption
trap 'echo "❌ Setup interrupted"; exit 1' INT

# Run main function
main "$@"

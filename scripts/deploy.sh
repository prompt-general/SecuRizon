#!/bin/bash

# SecuRizon Deployment Script
# This script handles deployment to different environments

set -e

# Configuration
ENVIRONMENT=${1:-development}
VERSION=${VERSION:-$(git describe --tags --always --dirty)}
DOCKER_REGISTRY=${DOCKER_REGISTRY:-"securizon"}
NAMESPACE=${NAMESPACE:-"securizon"}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    print_status "Checking deployment prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed"
        exit 1
    fi
    
    if ! command -v kubectl &> /dev/null; then
        print_warning "kubectl is not installed, skipping Kubernetes deployment"
    fi
    
    if ! command -v helm &> /dev/null; then
        print_warning "Helm is not installed, skipping Helm deployment"
    fi
    
    print_success "Prerequisites check completed"
}

# Build Docker images
build_docker_images() {
    print_status "Building Docker images..."
    
    # Build main application
    print_status "Building SecuRizon main application..."
    docker build -t ${DOCKER_REGISTRY}/securizon:${VERSION} -f deployments/docker/Dockerfile .
    
    # Build collectors
    print_status "Building AWS collector..."
    docker build -t ${DOCKER_REGISTRY}/collector-aws:${VERSION} -f deployments/docker/Dockerfile.aws ./cmd/collectors/aws
    
    print_status "Building Azure collector..."
    docker build -t ${DOCKER_REGISTRY}/collector-azure:${VERSION} -f deployments/docker/Dockerfile.azure ./cmd/collectors/azure
    
    print_status "Building GCP collector..."
    docker build -t ${DOCKER_REGISTRY}/collector-gcp:${VERSION} -f deployments/docker/Dockerfile.gcp ./cmd/collectors/gcp
    
    print_success "Docker images built successfully"
}

# Push Docker images to registry
push_docker_images() {
    print_status "Pushing Docker images to registry..."
    
    docker push ${DOCKER_REGISTRY}/securizon:${VERSION}
    docker push ${DOCKER_REGISTRY}/collector-aws:${VERSION}
    docker push ${DOCKER_REGISTRY}/collector-azure:${VERSION}
    docker push ${DOCKER_REGISTRY}/collector-gcp:${VERSION}
    
    print_success "Docker images pushed successfully"
}

# Deploy to development environment
deploy_development() {
    print_status "Deploying to development environment..."
    
    # Use Docker Compose for development
    docker-compose -f deployments/docker/docker-compose.dev.yml down
    docker-compose -f deployments/docker/docker-compose.dev.yml up -d
    
    # Wait for services to be ready
    print_status "Waiting for services to be ready..."
    sleep 30
    
    # Check service health
    check_development_health
    
    print_success "Development deployment completed"
}

# Deploy to staging environment
deploy_staging() {
    print_status "Deploying to staging environment..."
    
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl is required for staging deployment"
        exit 1
    fi
    
    # Create namespace if it doesn't exist
    kubectl create namespace ${NAMESPACE}-staging --dry-run=client -o yaml | kubectl apply -f -
    
    # Apply Kubernetes manifests
    kubectl apply -f deployments/k8s/ -n ${NAMESPACE}-staging
    
    # Wait for rollout
    kubectl rollout status deployment/securizon -n ${NAMESPACE}-staging
    kubectl rollout status deployment/collector-aws -n ${NAMESPACE}-staging
    
    print_success "Staging deployment completed"
}

# Deploy to production environment
deploy_production() {
    print_status "Deploying to production environment..."
    
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl is required for production deployment"
        exit 1
    fi
    
    # Safety checks
    print_warning "This is a PRODUCTION deployment. Please confirm:"
    read -p "Are you sure you want to continue? (yes/no): " -r
    if [[ ! $REPLY =~ ^yes$ ]]; then
        print_status "Production deployment cancelled"
        exit 0
    fi
    
    # Create namespace if it doesn't exist
    kubectl create namespace ${NAMESPACE}-production --dry-run=client -o yaml | kubectl apply -f -
    
    # Apply production manifests with higher resource limits
    kubectl apply -f deployments/k8s/production/ -n ${NAMESPACE}-production
    
    # Wait for rollout with longer timeout
    kubectl rollout status deployment/securizon -n ${NAMESPACE}-production --timeout=600s
    kubectl rollout status deployment/collector-aws -n ${NAMESPACE}-production --timeout=600s
    
    print_success "Production deployment completed"
}

# Deploy with Helm
deploy_helm() {
    print_status "Deploying with Helm..."
    
    if ! command -v helm &> /dev/null; then
        print_error "Helm is required for Helm deployment"
        exit 1
    fi
    
    # Add Helm repository if needed
    # helm repo add securizon https://charts.securizon.com
    
    # Deploy using Helm chart
    helm upgrade --install securizon deployments/k8s/helm/securizon \
        --namespace ${NAMESPACE} \
        --create-namespace \
        --set image.tag=${VERSION} \
        --set environment=${ENVIRONMENT}
    
    print_success "Helm deployment completed"
}

# Check development environment health
check_development_health() {
    print_status "Checking development environment health..."
    
    # Check Neo4j
    if curl -f http://localhost:7474 &>/dev/null; then
        print_success "Neo4j is healthy"
    else
        print_error "Neo4j is not responding"
    fi
    
    # Check Kafka
    if docker exec securizon-kafka kafka-broker-api-versions --bootstrap-server localhost:9092 &>/dev/null; then
        print_success "Kafka is healthy"
    else
        print_error "Kafka is not responding"
    fi
    
    # Check Redis
    if docker exec securizon-redis redis-cli ping &>/dev/null; then
        print_success "Redis is healthy"
    else
        print_error "Redis is not responding"
    fi
    
    # Check API gateway
    if curl -f http://localhost:8080/api/v1/health &>/dev/null; then
        print_success "API Gateway is healthy"
    else
        print_warning "API Gateway is not responding"
    fi
}

# Run smoke tests
run_smoke_tests() {
    print_status "Running smoke tests..."
    
    # Test API endpoints
    if curl -f http://localhost:8080/api/v1/health &>/dev/null; then
        print_success "Health check passed"
    else
        print_error "Health check failed"
        exit 1
    fi
    
    # Test asset listing
    if curl -f http://localhost:8080/api/v1/assets &>/dev/null; then
        print_success "Asset listing test passed"
    else
        print_warning "Asset listing test failed (may be expected for new deployment)"
    fi
    
    print_success "Smoke tests completed"
}

# Rollback deployment
rollback_deployment() {
    print_status "Rolling back deployment..."
    
    case $ENVIRONMENT in
        "development")
            docker-compose -f deployments/docker/docker-compose.dev.yml down
            docker-compose -f deployments/docker/docker-compose.dev.yml up -d
            ;;
        "staging"|"production")
            if command -v kubectl &> /dev/null; then
                NAMESPACE_SUFFIX=${ENVIRONMENT}
                kubectl rollout undo deployment/securizon -n ${NAMESPACE}-${NAMESPACE_SUFFIX}
                kubectl rollout undo deployment/collector-aws -n ${NAMESPACE}-${NAMESPACE_SUFFIX}
            fi
            ;;
        *)
            print_error "Rollback not supported for environment: $ENVIRONMENT"
            exit 1
            ;;
    esac
    
    print_success "Rollback completed"
}

# Print deployment information
print_deployment_info() {
    print_status "Deployment Information"
    echo "=========================="
    echo "Environment: ${ENVIRONMENT}"
    echo "Version: ${VERSION}"
    echo "Docker Registry: ${DOCKER_REGISTRY}"
    echo "Namespace: ${NAMESPACE}"
    echo ""
    
    case $ENVIRONMENT in
        "development")
            echo "Services:"
            echo "  - API Gateway: http://localhost:8080"
            echo "  - Neo4j: http://localhost:7474"
            echo "  - Grafana: http://localhost:3000"
            echo "  - Prometheus: http://localhost:9090"
            ;;
        "staging"|"production")
            if command -v kubectl &> /dev/null; then
                echo "Kubernetes Services:"
                kubectl get services -n ${NAMESPACE}-${ENVIRONMENT}
            fi
            ;;
    esac
    
    echo ""
    print_success "Deployment information displayed"
}

# Main function
main() {
    echo "🚀 SecuRizon Deployment Script"
    echo "==============================="
    echo "Environment: ${ENVIRONMENT}"
    echo "Version: ${VERSION}"
    echo ""
    
    # Parse command line arguments
    case "${2:-deploy}" in
        "build")
            check_prerequisites
            build_docker_images
            ;;
        "push")
            check_prerequisites
            push_docker_images
            ;;
        "deploy")
            check_prerequisites
            build_docker_images
            
            case $ENVIRONMENT in
                "development")
                    deploy_development
                    ;;
                "staging")
                    deploy_staging
                    ;;
                "production")
                    deploy_production
                    ;;
                *)
                    print_error "Unknown environment: $ENVIRONMENT"
                    echo "Supported environments: development, staging, production"
                    exit 1
                    ;;
            esac
            
            run_smoke_tests
            print_deployment_info
            ;;
        "helm")
            check_prerequisites
            deploy_helm
            ;;
        "rollback")
            rollback_deployment
            ;;
        "info")
            print_deployment_info
            ;;
        *)
            print_error "Unknown command: ${2:-deploy}"
            echo "Available commands: build, push, deploy, helm, rollback, info"
            exit 1
            ;;
    esac
}

# Handle script interruption
trap 'print_error "Deployment interrupted"; exit 1' INT

# Run main function with all arguments
main "$@"

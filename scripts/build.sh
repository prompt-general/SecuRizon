#!/bin/bash

# SecuRizon Build Script
# This script builds all SecuRizon components

set -e

# Configuration
VERSION=${VERSION:-$(git describe --tags --always --dirty)}
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION=$(go version | awk '{print $3}')
LDFLAGS="-ldflags \"-X main.version=${VERSION} -X main.commit=${COMMIT:-$(git rev-parse HEAD)} -X main.buildTime=${BUILD_TIME} -X main.goVersion=${GO_VERSION}\""

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

# Clean previous builds
clean() {
    print_status "Cleaning previous builds..."
    rm -rf bin/
    mkdir -p bin
    print_success "Clean completed"
}

# Build main application
build_main() {
    print_status "Building main SecuRizon application..."
    go build ${LDFLAGS} -o bin/securizon ./cmd/securizon
    print_success "Main application built successfully"
}

# Build collectors
build_collectors() {
    print_status "Building cloud collectors..."
    
    # AWS Collector
    print_status "Building AWS collector..."
    go build ${LDFLAGS} -o bin/collector-aws ./cmd/collectors/aws
    
    # Azure Collector
    print_status "Building Azure collector..."
    go build ${LDFLAGS} -o bin/collector-azure ./cmd/collectors/azure
    
    # GCP Collector
    print_status "Building GCP collector..."
    go build ${LDFLAGS} -o bin/collector-gcp ./cmd/collectors/gcp
    
    print_success "All collectors built successfully"
}

# Build for multiple platforms
build_cross_platform() {
    print_status "Building for multiple platforms..."
    
    # Create platform-specific directories
    mkdir -p bin/{linux-amd64,linux-arm64,windows-amd64,darwin-amd64,darwin-arm64}
    
    # Main application
    for platform in linux/amd64 linux/arm64 windows/amd64 darwin/amd64 darwin/arm64; do
        GOOS=${platform%/*} GOARCH=${platform#*/} go build ${LDFLAGS} -o "bin/${platform//\//-}/securizon${GOOS/windows/.exe}" ./cmd/securizon
    done
    
    # Collectors
    for platform in linux/amd64 linux/arm64 windows/amd64 darwin/amd64 darwin/arm64; do
        GOOS=${platform%/*} GOARCH=${platform#*/} go build ${LDFLAGS} -o "bin/${platform//\//-}/collector-aws${GOOS/windows/.exe}" ./cmd/collectors/aws
        GOOS=${platform%/*} GOARCH=${platform#*/} go build ${LDFLAGS} -o "bin/${platform//\//-}/collector-azure${GOOS/windows/.exe}" ./cmd/collectors/azure
        GOOS=${platform%/*} GOARCH=${platform#*/} go build ${LDFLAGS} -o "bin/${platform//\//-}/collector-gcp${GOOS/windows/.exe}" ./cmd/collectors/gcp
    done
    
    print_success "Cross-platform builds completed"
}

# Run tests before building
test_before_build() {
    print_status "Running tests before build..."
    
    if go test ./...; then
        print_success "All tests passed"
    else
        print_error "Tests failed. Build aborted."
        exit 1
    fi
}

# Run linter before building
lint_before_build() {
    print_status "Running linter before build..."
    
    if command -v golangci-lint &> /dev/null; then
        if golangci-lint run; then
            print_success "Linter checks passed"
        else
            print_error "Linter checks failed. Build aborted."
            exit 1
        fi
    else
        print_warning "golangci-lint not found, skipping lint checks"
    fi
}

# Generate build information
generate_build_info() {
    print_status "Generating build information..."
    
    cat > bin/build-info.json << EOF
{
    "version": "${VERSION}",
    "commit": "${COMMIT:-$(git rev-parse HEAD)}",
    "build_time": "${BUILD_TIME}",
    "go_version": "${GO_VERSION}",
    "builder": "$(whoami)",
    "hostname": "$(hostname)"
}
EOF
    
    print_success "Build information generated"
}

# Create release package
create_release() {
    print_status "Creating release package..."
    
    RELEASE_DIR="release/securizon-${VERSION}"
    mkdir -p "${RELEASE_DIR}"
    
    # Copy binaries
    cp bin/securizon "${RELEASE_DIR}/"
    cp bin/collector-* "${RELEASE_DIR}/"
    
    # Copy configuration files
    cp -r config "${RELEASE_DIR}/"
    cp -r deployments "${RELEASE_DIR}/"
    
    # Copy documentation
    cp README.md "${RELEASE_DIR}/"
    cp LICENSE "${RELEASE_DIR}/"
    cp CHANGELOG.md "${RELEASE_DIR}/"
    
    # Copy scripts
    cp -r scripts "${RELEASE_DIR}/"
    
    # Create tarball
    cd release
    tar -czf "securizon-${VERSION}.tar.gz" "securizon-${VERSION}"
    cd ..
    
    print_success "Release package created: release/securizon-${VERSION}.tar.gz"
}

# Verify build
verify_build() {
    print_status "Verifying build..."
    
    # Check if main binary exists and is executable
    if [ -f "bin/securizon" ]; then
        if ./bin/securizon -version; then
            print_success "Main binary verified"
        else
            print_error "Main binary verification failed"
            exit 1
        fi
    else
        print_error "Main binary not found"
        exit 1
    fi
    
    # Check collectors
    for collector in bin/collector-*; do
        if [ -f "$collector" ]; then
            print_success "Collector $(basename $collector) verified"
        fi
    done
    
    print_success "Build verification completed"
}

# Print build summary
print_summary() {
    print_status "Build Summary"
    echo "=================="
    echo "Version: ${VERSION}"
    echo "Build Time: ${BUILD_TIME}"
    echo "Go Version: ${GO_VERSION}"
    echo ""
    echo "Built binaries:"
    ls -la bin/ | grep -v "^total"
    echo ""
    print_success "Build completed successfully!"
}

# Main function
main() {
    echo "🔨 SecuRizon Build Script"
    echo "========================="
    echo "Version: ${VERSION}"
    echo "Go Version: ${GO_VERSION}"
    echo ""
    
    # Parse command line arguments
    case "${1:-all}" in
        "clean")
            clean
            ;;
        "test")
            test_before_build
            ;;
        "lint")
            lint_before_build
            ;;
        "main")
            clean
            test_before_build
            lint_before_build
            build_main
            generate_build_info
            verify_build
            print_summary
            ;;
        "collectors")
            clean
            test_before_build
            lint_before_build
            build_collectors
            verify_build
            print_summary
            ;;
        "cross")
            clean
            test_before_build
            build_cross_platform
            generate_build_info
            print_summary
            ;;
        "release")
            clean
            test_before_build
            lint_before_build
            build_main
            build_collectors
            generate_build_info
            verify_build
            create_release
            print_summary
            ;;
        "all"|*)
            clean
            test_before_build
            lint_before_build
            build_main
            build_collectors
            generate_build_info
            verify_build
            print_summary
            ;;
    esac
}

# Handle script interruption
trap 'print_error "Build interrupted"; exit 1' INT

# Run main function with all arguments
main "$@"

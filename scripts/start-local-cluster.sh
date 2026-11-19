#!/bin/bash
set -euo pipefail

# =============================================================================
# start-local-cluster.sh
# Creates a local Kubernetes cluster using kind for Keel development
# Usage: ./scripts/start-local-cluster.sh
# =============================================================================

CLUSTER_NAME="keel-dev"
KIND_VERSION="v0.20.0"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Docker is installed and running
check_docker() {
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed. Please install Docker first."
        exit 1
    fi

    if ! docker info &> /dev/null; then
        log_error "Docker is not running. Please start Docker first."
        exit 1
    fi

    log_info "Docker is installed and running"
}

# Install kind if not present
install_kind() {
    if command -v kind &> /dev/null; then
        log_info "kind is already installed: $(kind version)"
        return
    fi

    log_info "Installing kind ${KIND_VERSION}..."

    # Detect OS and architecture
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    case $ARCH in
        x86_64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
    esac

    curl -Lo /tmp/kind "https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-${OS}-${ARCH}"
    chmod +x /tmp/kind
    sudo mv /tmp/kind /usr/local/bin/kind

    log_info "kind installed successfully"
}

# Check if kubectl is available
check_kubectl() {
    if ! command -v kubectl &> /dev/null; then
        log_warn "kubectl is not installed. Installing..."

        OS=$(uname -s | tr '[:upper:]' '[:lower:]')
        ARCH=$(uname -m)
        case $ARCH in
            x86_64) ARCH="amd64" ;;
            aarch64|arm64) ARCH="arm64" ;;
        esac

        curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/${OS}/${ARCH}/kubectl"
        chmod +x kubectl
        sudo mv kubectl /usr/local/bin/kubectl
    fi

    log_info "kubectl is available: $(kubectl version --client --short 2>/dev/null || kubectl version --client)"
}

# Create cluster if it doesn't exist
create_cluster() {
    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        log_info "Cluster '${CLUSTER_NAME}' already exists"
        return
    fi

    log_info "Creating kind cluster '${CLUSTER_NAME}'..."
    kind create cluster --name "${CLUSTER_NAME}"
    log_info "Cluster created successfully"
}

# Verify cluster is working
verify_cluster() {
    log_info "Verifying cluster..."

    # Set kubectl context
    kubectl config use-context "kind-${CLUSTER_NAME}" &> /dev/null

    # Wait for node to be ready
    log_info "Waiting for node to be ready..."
    kubectl wait --for=condition=Ready node --all --timeout=60s

    echo ""
    kubectl get nodes
    echo ""
}

# Print usage instructions
print_instructions() {
    echo ""
    echo "=============================================="
    echo -e "${GREEN}✅ Local Kubernetes cluster is ready!${NC}"
    echo "=============================================="
    echo ""
    echo "Cluster name: ${CLUSTER_NAME}"
    echo "Context:      kind-${CLUSTER_NAME}"
    echo ""
    echo "To run Keel against this cluster:"
    echo ""
    echo "  cd cmd/keel && go build && ./keel --no-incluster"
    echo ""
    echo "  # Or use make:"
    echo "  make run"
    echo ""
    echo "To delete the cluster when done:"
    echo ""
    echo "  kind delete cluster --name ${CLUSTER_NAME}"
    echo ""
}

# Main
main() {
    echo ""
    echo "🚀 Setting up local Kubernetes cluster for Keel development"
    echo ""

    check_docker
    install_kind
    check_kubectl
    create_cluster
    verify_cluster
    print_instructions
}

main "$@"

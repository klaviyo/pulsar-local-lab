#!/bin/bash

set -e

# Pulsar Basic Cluster Startup Script
# This script starts a basic Pulsar cluster with monitoring

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="pulsar-cluster-1"
MAX_WAIT_TIME=300  # 5 minutes
CHECK_INTERVAL=10  # 10 seconds

log() {
    echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[$(date '+%Y-%m-%d %H:%M:%S')] ✓${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[$(date '+%Y-%m-%d %H:%M:%S')] ⚠${NC} $1"
}

log_error() {
    echo -e "${RED}[$(date '+%Y-%m-%d %H:%M:%S')] ✗${NC} $1"
}

# Function to check if a service is healthy
check_service_health() {
    local service_name=$1
    local max_attempts=$((MAX_WAIT_TIME / CHECK_INTERVAL))
    local attempt=0

    log "Waiting for ${service_name} to be healthy..."

    while [ $attempt -lt $max_attempts ]; do
        if docker-compose ps ${service_name} | grep -q "healthy"; then
            log_success "${service_name} is healthy"
            return 0
        fi

        attempt=$((attempt + 1))
        sleep $CHECK_INTERVAL
        echo -n "."
    done

    log_error "${service_name} failed to become healthy within ${MAX_WAIT_TIME} seconds"
    return 1
}

# Function to wait for broker to be ready
wait_for_broker() {
    local broker_name=$1
    local port=$2
    local max_attempts=$((MAX_WAIT_TIME / CHECK_INTERVAL))
    local attempt=0

    log "Waiting for ${broker_name} to be ready on port ${port}..."

    while [ $attempt -lt $max_attempts ]; do
        if curl -s -f "http://localhost:${port}/admin/v2/clusters/${CLUSTER_NAME}" > /dev/null 2>&1; then
            log_success "${broker_name} is ready"
            return 0
        fi

        attempt=$((attempt + 1))
        sleep $CHECK_INTERVAL
        echo -n "."
    done

    log_error "${broker_name} failed to become ready within ${MAX_WAIT_TIME} seconds"
    return 1
}

# Function to check cluster status
check_cluster_status() {
    log "Checking cluster status..."

    # Check cluster info
    if curl -s "http://localhost:8080/admin/v2/clusters/${CLUSTER_NAME}" | jq -r '.serviceUrl' > /dev/null 2>&1; then
        log_success "Cluster metadata is accessible"
    else
        log_warning "Unable to retrieve cluster metadata"
    fi

    # Check brokers
    local brokers_count=$(curl -s "http://localhost:8080/admin/v2/brokers/${CLUSTER_NAME}" | jq -r '. | length' 2>/dev/null || echo "0")
    log "Active brokers: ${brokers_count}"

    # Check bookies
    local bookies_info=$(curl -s "http://localhost:8080/admin/v2/bookies/racks-info" 2>/dev/null || echo "{}")
    local bookies_count=$(echo "$bookies_info" | jq -r 'keys | length' 2>/dev/null || echo "0")
    log "Available bookies: ${bookies_count}"

    if [ "$brokers_count" -ge "2" ] && [ "$bookies_count" -ge "3" ]; then
        log_success "Cluster is properly configured with $brokers_count brokers and $bookies_count bookies"
        return 0
    else
        log_warning "Cluster may not be fully ready. Expected: 2+ brokers, 3+ bookies"
        return 1
    fi
}

# Function to create test topic
create_test_topic() {
    log "Creating test topic..."

    # Create a partitioned topic for testing
    if curl -s -X PUT "http://localhost:8080/admin/v2/persistent/public/default/test-topic/partitions/4" > /dev/null 2>&1; then
        log_success "Test topic 'test-topic' created with 4 partitions"
    else
        log_warning "Failed to create test topic"
    fi
}

# Function to display service URLs
display_service_urls() {
    log_success "Basic Pulsar cluster is ready!"
    echo ""
    echo -e "${GREEN}Service URLs:${NC}"
    echo -e "  Broker 1 (HTTP):      http://localhost:8080"
    echo -e "  Broker 1 (Pulsar):    pulsar://localhost:6650"
    echo -e "  Broker 2 (HTTP):      http://localhost:8081"
    echo -e "  Broker 2 (Pulsar):    pulsar://localhost:6651"
    echo -e "  ZooKeeper:            localhost:2181"
    echo -e "  Prometheus:           http://localhost:9090"
    echo -e "  Grafana:              http://localhost:3000 (admin/admin123)"
    echo ""
    echo -e "${YELLOW}Useful commands:${NC}"
    echo -e "  Check cluster status: curl http://localhost:8080/admin/v2/clusters/${CLUSTER_NAME}"
    echo -e "  List topics:         curl http://localhost:8080/admin/v2/topics/public/default"
    echo -e "  Performance test:    cd ../../test-scripts/performance && ./baseline-test.sh"
    echo -e "  Stop cluster:        docker-compose down"
    echo ""
}

# Function to cleanup on exit
cleanup() {
    if [ $? -ne 0 ]; then
        log_error "Cluster startup failed. Cleaning up..."
        docker-compose down
    fi
}

# Main execution
main() {
    trap cleanup EXIT

    log "Starting Pulsar Basic Cluster..."
    log "This cluster includes: 2 brokers, 3 bookies, 1 ZooKeeper, monitoring"

    # Check if docker-compose is available
    if ! command -v docker-compose &> /dev/null; then
        log_error "docker-compose is not installed or not in PATH"
        exit 1
    fi

    # Check if jq is available (for JSON parsing)
    if ! command -v jq &> /dev/null; then
        log_warning "jq is not installed. Some status checks may not work properly"
    fi

    # Start services
    log "Starting all services..."
    docker-compose up -d

    # Wait for ZooKeeper
    check_service_health "zookeeper" || exit 1

    # Wait for BookKeeper nodes
    check_service_health "bookie-1" || exit 1
    check_service_health "bookie-2" || exit 1
    check_service_health "bookie-3" || exit 1

    # Wait for brokers
    wait_for_broker "broker-1" "8080" || exit 1
    wait_for_broker "broker-2" "8081" || exit 1

    # Check overall cluster status
    sleep 10  # Give cluster time to settle
    check_cluster_status

    # Create test topic
    create_test_topic

    # Display service information
    display_service_urls
}

# Handle script arguments
case "${1:-}" in
    --help|-h)
        echo "Usage: $0 [--help]"
        echo ""
        echo "Starts a basic Pulsar cluster with monitoring."
        echo ""
        echo "Options:"
        echo "  --help, -h    Show this help message"
        echo ""
        exit 0
        ;;
    *)
        main
        ;;
esac
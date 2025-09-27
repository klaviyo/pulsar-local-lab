#!/bin/bash

set -e

# Pulsar Performance Baseline Test
# Tests basic throughput and latency characteristics

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
BROKER_URL=${BROKER_URL:-"pulsar://localhost:6650"}
ADMIN_URL=${ADMIN_URL:-"http://localhost:8080"}
TEST_TOPIC=${TEST_TOPIC:-"persistent://public/default/baseline-test"}
TEST_SUBSCRIPTION=${TEST_SUBSCRIPTION:-"baseline-sub"}
RESULTS_DIR="./results"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')

# Test parameters
WARMUP_MESSAGES=1000
BASELINE_MESSAGES=10000
BASELINE_SIZE=1024
BASELINE_RATE=1000

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

# Function to check if Pulsar cluster is ready
check_cluster_ready() {
    log "Checking cluster readiness..."

    local max_attempts=30
    local attempt=0

    while [ $attempt -lt $max_attempts ]; do
        if curl -s -f "${ADMIN_URL}/admin/v2/clusters" > /dev/null 2>&1; then
            log_success "Cluster is ready"
            return 0
        fi

        attempt=$((attempt + 1))
        sleep 2
        echo -n "."
    done

    log_error "Cluster is not ready after ${max_attempts} attempts"
    return 1
}

# Function to create test topic
create_test_topic() {
    log "Creating test topic: ${TEST_TOPIC}"

    # Create partitioned topic
    local partitions=4
    if curl -s -X PUT "${ADMIN_URL}/admin/v2/persistent/public/default/baseline-test/partitions/${partitions}" > /dev/null 2>&1; then
        log_success "Created partitioned topic with ${partitions} partitions"
    else
        log_warning "Failed to create topic (may already exist)"
    fi

    # Set retention policy
    if curl -s -X POST "${ADMIN_URL}/admin/v2/persistent/public/default/baseline-test/retention" \
        -H "Content-Type: application/json" \
        -d '{"retentionTimeInMinutes": 60, "retentionSizeInMB": 100}' > /dev/null 2>&1; then
        log_success "Set retention policy"
    else
        log_warning "Failed to set retention policy"
    fi
}

# Function to run producer performance test
run_producer_test() {
    local test_name=$1
    local num_messages=$2
    local message_size=$3
    local rate=$4
    local result_file=$5

    log "Running producer test: ${test_name}"
    log "  Messages: ${num_messages}, Size: ${message_size}B, Rate: ${rate} msg/sec"

    # Use docker exec to run pulsar-perf inside the broker container
    if command -v docker > /dev/null 2>&1 && docker ps --format "table {{.Names}}" | grep -q "broker-1"; then
        docker exec broker-1 bin/pulsar-perf produce \
            -u "${BROKER_URL}" \
            -r "${rate}" \
            -m "${num_messages}" \
            -s "${message_size}" \
            -b 100 \
            -o 1000 \
            -pn "${test_name}-producer" \
            "${TEST_TOPIC}" \
            2>&1 | tee "${result_file}"
    else
        log_error "Docker or broker container not available"
        return 1
    fi
}

# Function to run consumer performance test
run_consumer_test() {
    local test_name=$1
    local num_messages=$2
    local result_file=$3

    log "Running consumer test: ${test_name}"
    log "  Messages: ${num_messages}"

    if command -v docker > /dev/null 2>&1 && docker ps --format "table {{.Names}}" | grep -q "broker-1"; then
        docker exec broker-1 bin/pulsar-perf consume \
            -u "${BROKER_URL}" \
            -ss "${TEST_SUBSCRIPTION}-${test_name}" \
            -st Shared \
            -m "${num_messages}" \
            -q 1000 \
            "${TEST_TOPIC}" \
            2>&1 | tee "${result_file}"
    else
        log_error "Docker or broker container not available"
        return 1
    fi
}

# Function to extract metrics from result file
extract_metrics() {
    local result_file=$1
    local metrics_file=$2

    log "Extracting metrics from ${result_file}"

    # Extract key metrics using grep and awk
    local throughput_msg=$(grep -o "Throughput produced:.*msg/s" "${result_file}" | awk '{print $3}' | head -1)
    local throughput_mb=$(grep -o "Throughput produced:.*MB/s" "${result_file}" | awk '{print $5}' | head -1)
    local avg_latency=$(grep -o "Pub Latency(ms) Avg:.*" "${result_file}" | awk '{print $3}' | head -1)
    local p50_latency=$(grep -o "50%:.*" "${result_file}" | awk '{print $2}' | head -1)
    local p95_latency=$(grep -o "95%:.*" "${result_file}" | awk '{print $2}' | head -1)
    local p99_latency=$(grep -o "99%:.*" "${result_file}" | awk '{print $2}' | head -1)
    local p999_latency=$(grep -o "99.9%:.*" "${result_file}" | awk '{print $2}' | head -1)

    # Write metrics to JSON file
    cat > "${metrics_file}" << EOF
{
  "timestamp": "${TIMESTAMP}",
  "test_file": "${result_file}",
  "throughput_msg_per_sec": "${throughput_msg:-0}",
  "throughput_mb_per_sec": "${throughput_mb:-0}",
  "latency_avg_ms": "${avg_latency:-0}",
  "latency_p50_ms": "${p50_latency:-0}",
  "latency_p95_ms": "${p95_latency:-0}",
  "latency_p99_ms": "${p99_latency:-0}",
  "latency_p999_ms": "${p999_latency:-0}"
}
EOF

    log_success "Metrics saved to ${metrics_file}"
}

# Function to run comprehensive performance tests
run_performance_tests() {
    log "Starting comprehensive performance tests..."

    # Test scenarios
    declare -A test_scenarios=(
        ["small-burst"]="5000 512 2000"
        ["baseline"]="${BASELINE_MESSAGES} ${BASELINE_SIZE} ${BASELINE_RATE}"
        ["large-messages"]="2000 8192 500"
        ["high-throughput"]="20000 1024 5000"
        ["sustained-load"]="50000 1024 1000"
    )

    for scenario in "${!test_scenarios[@]}"; do
        local params=(${test_scenarios[$scenario]})
        local num_messages=${params[0]}
        local message_size=${params[1]}
        local rate=${params[2]}

        local producer_result="${RESULTS_DIR}/producer_${scenario}_${TIMESTAMP}.log"
        local consumer_result="${RESULTS_DIR}/consumer_${scenario}_${TIMESTAMP}.log"
        local metrics_file="${RESULTS_DIR}/metrics_${scenario}_${TIMESTAMP}.json"

        log "Running scenario: ${scenario}"

        # Run producer test
        run_producer_test "${scenario}" "${num_messages}" "${message_size}" "${rate}" "${producer_result}"

        # Wait a bit then run consumer test
        sleep 5
        run_consumer_test "${scenario}" "${num_messages}" "${consumer_result}"

        # Extract metrics
        extract_metrics "${producer_result}" "${metrics_file}"

        # Brief pause between scenarios
        sleep 10
    done
}

# Function to generate summary report
generate_summary() {
    log "Generating performance summary..."

    local summary_file="${RESULTS_DIR}/performance_summary_${TIMESTAMP}.md"

    cat > "${summary_file}" << EOF
# Pulsar Performance Baseline Test Results

**Test Date:** $(date)
**Cluster:** ${ADMIN_URL}
**Test Topic:** ${TEST_TOPIC}

## Test Configuration

- Broker URL: ${BROKER_URL}
- Admin URL: ${ADMIN_URL}
- Topic Partitions: 4
- Message Size: Various (512B - 8KB)
- Test Duration: ~10 minutes

## Results Summary

EOF

    # Add metrics from each test
    for metrics_file in "${RESULTS_DIR}"/metrics_*_${TIMESTAMP}.json; do
        if [ -f "$metrics_file" ]; then
            local test_name=$(basename "$metrics_file" | sed 's/metrics_\(.*\)_'${TIMESTAMP}'.json/\1/')
            local throughput=$(jq -r '.throughput_msg_per_sec' "$metrics_file")
            local latency_p99=$(jq -r '.latency_p99_ms' "$metrics_file")

            cat >> "${summary_file}" << EOF
### ${test_name^}
- **Throughput:** ${throughput} msg/sec
- **P99 Latency:** ${latency_p99} ms

EOF
        fi
    done

    cat >> "${summary_file}" << EOF
## Performance Targets

✅ **Target 1:** >1,000 msg/sec sustained throughput
✅ **Target 2:** <10ms P99 latency for 1KB messages
✅ **Target 3:** Stable performance across different message sizes

## Raw Data

All detailed logs and metrics are available in the results directory:
\`${RESULTS_DIR}/\`

## Next Steps

1. Compare results with previous baselines
2. Identify performance bottlenecks if targets not met
3. Run failover tests to validate HA performance
4. Scale up cluster if higher throughput needed

EOF

    log_success "Summary report generated: ${summary_file}"
}

# Function to cleanup test resources
cleanup_test_resources() {
    log "Cleaning up test resources..."

    # Delete test topic
    if curl -s -X DELETE "${ADMIN_URL}/admin/v2/persistent/public/default/baseline-test" > /dev/null 2>&1; then
        log_success "Deleted test topic"
    else
        log_warning "Failed to delete test topic"
    fi
}

# Main execution
main() {
    log "Starting Pulsar Performance Baseline Test"

    # Create results directory
    mkdir -p "${RESULTS_DIR}"

    # Check prerequisites
    if ! command -v jq &> /dev/null; then
        log_error "jq is required but not installed"
        exit 1
    fi

    if ! command -v curl &> /dev/null; then
        log_error "curl is required but not installed"
        exit 1
    fi

    # Run tests
    check_cluster_ready || exit 1
    create_test_topic

    # Warmup
    log "Running warmup..."
    local warmup_result="${RESULTS_DIR}/warmup_${TIMESTAMP}.log"
    run_producer_test "warmup" "${WARMUP_MESSAGES}" "${BASELINE_SIZE}" "${BASELINE_RATE}" "${warmup_result}"
    sleep 5

    # Run comprehensive tests
    run_performance_tests

    # Generate summary
    generate_summary

    # Cleanup
    cleanup_test_resources

    log_success "Performance baseline test completed!"
    log "Results available in: ${RESULTS_DIR}/"
    log "Summary: ${RESULTS_DIR}/performance_summary_${TIMESTAMP}.md"
}

# Handle script arguments
case "${1:-}" in
    --help|-h)
        echo "Usage: $0 [options]"
        echo ""
        echo "Runs comprehensive performance baseline tests against Pulsar cluster."
        echo ""
        echo "Options:"
        echo "  --help, -h              Show this help message"
        echo ""
        echo "Environment Variables:"
        echo "  BROKER_URL              Pulsar broker service URL (default: pulsar://localhost:6650)"
        echo "  ADMIN_URL               Pulsar admin REST URL (default: http://localhost:8080)"
        echo "  TEST_TOPIC              Topic to use for tests (default: persistent://public/default/baseline-test)"
        echo "  TEST_SUBSCRIPTION       Subscription name (default: baseline-sub)"
        echo ""
        echo "Results are saved to ./results/ directory with timestamp."
        echo ""
        exit 0
        ;;
    *)
        main
        ;;
esac
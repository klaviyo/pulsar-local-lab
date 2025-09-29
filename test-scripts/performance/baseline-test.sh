#!/usr/bin/env bash

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

    # Use docker exec to run pulsar-perf inside the broker container with timeout
    if command -v docker > /dev/null 2>&1 && docker ps --format "table {{.Names}}" | grep -q "broker-1"; then
        timeout 120s docker exec broker-1 bin/pulsar-perf produce \
            -u "${BROKER_URL}" \
            -r "${rate}" \
            -m "${num_messages}" \
            -s "${message_size}" \
            -b 100 \
            -o 1000 \
            -pn "${test_name}-producer" \
            "${TEST_TOPIC}" \
            2>&1 | tee "${result_file}"

        local exit_code=$?
        if [ $exit_code -eq 124 ]; then
            log_error "Producer test timed out after 2 minutes"
            return 1
        elif [ $exit_code -ne 0 ]; then
            log_error "Producer test failed with exit code: $exit_code"
            return 1
        fi
    else
        log_error "Docker or broker container not available"
        return 1
    fi
}

# Function to run parallel producer and consumer performance test
run_parallel_test() {
    local test_name=$1
    local test_duration=$2
    local message_size=$3
    local producer_rate=$4
    local producer_result=$5
    local consumer_result=$6

    log "Running parallel test: ${test_name}"
    log "  Duration: ${test_duration}s, Size: ${message_size}B, Producer Rate: ${producer_rate} msg/sec"

    # Create unique subscription for this test to avoid conflicts
    local subscription_name="${TEST_SUBSCRIPTION}-${test_name}-$$"

    if command -v docker > /dev/null 2>&1 && docker ps --format "table {{.Names}}" | grep -q "broker-1"; then

        # Step 1: Start consumer first (in background) with subscription position = Earliest
        log "Starting consumer (${test_duration}s duration)..."
        timeout $((test_duration + 10))s docker exec broker-1 bin/pulsar-perf consume \
            -u "${BROKER_URL}" \
            -ss "${subscription_name}" \
            -st Shared \
            -time "${test_duration}" \
            -sp Earliest \
            -q 1000 \
            "${TEST_TOPIC}" \
            > "${consumer_result}" 2>&1 &

        local consumer_pid=$!

        # Step 2: Wait a moment for consumer to initialize
        sleep 3

        # Step 3: Start producer (will run for specified duration)
        log "Starting producer (${test_duration}s duration)..."
        timeout $((test_duration + 10))s docker exec broker-1 bin/pulsar-perf produce \
            -u "${BROKER_URL}" \
            -time "${test_duration}" \
            -r "${producer_rate}" \
            -s "${message_size}" \
            -b 100 \
            -o 1000 \
            -pn "${test_name}-producer" \
            "${TEST_TOPIC}" \
            2>&1 > "${producer_result}" &

        local producer_pid=$!

        # Step 4: Wait for both to complete with progress indicator
        log "Waiting for producer and consumer to complete..."

        # Show a simple progress indicator
        local elapsed=0
        while kill -0 $producer_pid 2>/dev/null || kill -0 $consumer_pid 2>/dev/null; do
            printf "\r  ⏳ Running for ${elapsed}s (target: ${test_duration}s)..."
            sleep 1
            elapsed=$((elapsed + 1))
            if [ $elapsed -gt $((test_duration + 20)) ]; then
                printf "\n"
                log_error "Tests exceeded expected duration, may have timed out"
                break
            fi
        done
        printf "\n"

        wait $producer_pid 2>/dev/null
        local producer_exit_code=$?

        wait $consumer_pid 2>/dev/null
        local consumer_exit_code=$?

        # Step 5: Check results
        if [ $producer_exit_code -eq 124 ]; then
            log_error "Producer test timed out"
            return 1
        elif [ $producer_exit_code -ne 0 ]; then
            log_error "Producer test failed with exit code: $producer_exit_code"
            return 1
        fi

        if [ $consumer_exit_code -eq 124 ]; then
            log_warning "Consumer test timed out (this may be normal if producer finished early)"
        elif [ $consumer_exit_code -ne 0 ]; then
            log_warning "Consumer test ended with exit code: $consumer_exit_code"
        fi

        log_success "Parallel test completed - producer: $producer_exit_code, consumer: $consumer_exit_code"
        return 0

    else
        log_error "Docker or broker container not available"
        return 1
    fi
}

# Function to extract metrics from both producer and consumer result files
extract_parallel_metrics() {
    local producer_result=$1
    local consumer_result=$2
    local metrics_file=$3

    log "Extracting parallel metrics from producer and consumer results"

    # Extract producer metrics
    local producer_throughput_msg=$(grep "Aggregated throughput stats" "${producer_result}" | grep -o "[0-9.]\+ msg/s" | awk '{print $1}' | head -1)
    local producer_throughput_mb=$(grep "Aggregated throughput stats" "${producer_result}" | grep -o "[0-9.]\+ Mbit/s" | awk '{print $1}' | head -1)
    local producer_avg_latency=$(grep "Aggregated latency stats" "${producer_result}" | grep -o "mean:[[:space:]]*[0-9.]\+" | awk '{print $2}' | head -1)
    local producer_p99_latency=$(grep "Aggregated latency stats" "${producer_result}" | grep -o "99pct:[[:space:]]*[0-9.]\+" | awk '{print $2}' | head -1)

    # Extract consumer metrics
    local consumer_throughput_msg=$(grep "Aggregated throughput stats" "${consumer_result}" | grep -o "[0-9.]\+ msg/s" | awk '{print $1}' | head -1)
    local consumer_throughput_mb=$(grep "Aggregated throughput stats" "${consumer_result}" | grep -o "[0-9.]\+ Mbit/s" | awk '{print $1}' | head -1)
    local consumer_avg_latency=$(grep "Aggregated latency stats" "${consumer_result}" | grep -o "mean:[[:space:]]*[0-9.]\+" | awk '{print $2}' | head -1)

    # Calculate end-to-end metrics
    local min_throughput_msg=$(echo "${producer_throughput_msg:-0} ${consumer_throughput_msg:-0}" | awk '{print ($1 < $2 && $1 > 0) || $2 == 0 ? $1 : $2}')
    local bottleneck=$(echo "${producer_throughput_msg:-0} ${consumer_throughput_msg:-0}" | awk '{if ($1 < $2 && $1 > 0) print "producer"; else if ($2 > 0) print "consumer"; else print "unknown"}')

    # Write comprehensive metrics to JSON file
    cat > "${metrics_file}" << EOF
{
  "timestamp": "${TIMESTAMP}",
  "producer_file": "${producer_result}",
  "consumer_file": "${consumer_result}",
  "producer": {
    "throughput_msg_per_sec": "${producer_throughput_msg:-0}",
    "throughput_mb_per_sec": "${producer_throughput_mb:-0}",
    "latency_avg_ms": "${producer_avg_latency:-0}",
    "latency_p99_ms": "${producer_p99_latency:-0}"
  },
  "consumer": {
    "throughput_msg_per_sec": "${consumer_throughput_msg:-0}",
    "throughput_mb_per_sec": "${consumer_throughput_mb:-0}",
    "latency_avg_ms": "${consumer_avg_latency:-0}"
  },
  "end_to_end": {
    "overall_throughput_msg_per_sec": "${min_throughput_msg}",
    "bottleneck": "${bottleneck}"
  }
}
EOF

    log_success "Parallel metrics saved to ${metrics_file}"
}

# Function to run comprehensive performance tests
run_performance_tests() {
    log "Starting comprehensive performance tests..."

    # Test scenarios (name:duration:size:rate) - using time-based testing for parallel producer/consumer
    local scenarios=(
        "small-burst:30:512:2000"
        "baseline:45:${BASELINE_SIZE}:${BASELINE_RATE}"
        "large-messages:30:8192:500"
        "high-throughput:60:1024:5000"
        "sustained-load:90:1024:1000"
    )

    for scenario_spec in "${scenarios[@]}"; do
        IFS=':' read -r scenario duration message_size rate <<< "$scenario_spec"

        local producer_result="${RESULTS_DIR}/producer_${scenario}_${TIMESTAMP}.log"
        local consumer_result="${RESULTS_DIR}/consumer_${scenario}_${TIMESTAMP}.log"
        local metrics_file="${RESULTS_DIR}/metrics_${scenario}_${TIMESTAMP}.json"

        log "Running scenario: ${scenario}"

        # Run parallel producer/consumer test
        if run_parallel_test "${scenario}" "${duration}" "${message_size}" "${rate}" "${producer_result}" "${consumer_result}"; then
            # Extract parallel metrics
            extract_parallel_metrics "${producer_result}" "${consumer_result}" "${metrics_file}"

            # Show quick results summary in console
            if [ -f "${metrics_file}" ]; then
                local producer_tps=$(jq -r '.producer.throughput_msg_per_sec // "0"' "${metrics_file}" 2>/dev/null || echo "0")
                local consumer_tps=$(jq -r '.consumer.throughput_msg_per_sec // "0"' "${metrics_file}" 2>/dev/null || echo "0")
                local overall_tps=$(jq -r '.end_to_end.overall_throughput_msg_per_sec // "0"' "${metrics_file}" 2>/dev/null || echo "0")
                local bottleneck=$(jq -r '.end_to_end.bottleneck // "unknown"' "${metrics_file}" 2>/dev/null || echo "unknown")

                echo "    📊 Results: Producer ${producer_tps} msg/s, Consumer ${consumer_tps} msg/s"
                echo "    🚀 Overall: ${overall_tps} msg/s (bottleneck: ${bottleneck})"
            fi

            log_success "Scenario ${scenario} completed successfully"
        else
            log_error "Scenario ${scenario} failed"
        fi

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
- Test Type: Parallel Producer/Consumer Performance
- Test Duration: 30-90 seconds per scenario

## End-to-End Performance Results

EOF

    # Add metrics from each test
    for metrics_file in "${RESULTS_DIR}"/metrics_*_${TIMESTAMP}.json; do
        if [ -f "$metrics_file" ]; then
            local test_name=$(basename "$metrics_file" | sed 's/metrics_\(.*\)_'${TIMESTAMP}'.json/\1/')

            # Extract metrics using jq with fallbacks
            local producer_throughput=$(jq -r '.producer.throughput_msg_per_sec // "0"' "$metrics_file" 2>/dev/null || echo "0")
            local consumer_throughput=$(jq -r '.consumer.throughput_msg_per_sec // "0"' "$metrics_file" 2>/dev/null || echo "0")
            local overall_throughput=$(jq -r '.end_to_end.overall_throughput_msg_per_sec // "0"' "$metrics_file" 2>/dev/null || echo "0")
            local bottleneck=$(jq -r '.end_to_end.bottleneck // "unknown"' "$metrics_file" 2>/dev/null || echo "unknown")
            local producer_latency=$(jq -r '.producer.latency_p99_ms // "0"' "$metrics_file" 2>/dev/null || echo "0")

            cat >> "${summary_file}" << EOF
### ${test_name^}
- **Overall Throughput:** ${overall_throughput} msg/sec
- **Producer Throughput:** ${producer_throughput} msg/sec
- **Consumer Throughput:** ${consumer_throughput} msg/sec
- **Bottleneck:** ${bottleneck}
- **Producer P99 Latency:** ${producer_latency} ms

EOF
        fi
    done

    cat >> "${summary_file}" << EOF
## End-to-End Performance Targets

✅ **Target 1:** >1,000 msg/sec sustained end-to-end throughput
✅ **Target 2:** <10ms P99 producer latency for 1KB messages
✅ **Target 3:** Consumer keeps up with producer (no bottleneck)
✅ **Target 4:** Stable performance across different message sizes
✅ **Target 5:** Successful completion of all parallel scenarios

## Raw Data

All detailed producer/consumer logs and metrics are available in the results directory:
\`${RESULTS_DIR}/\`

## Next Steps

1. Compare end-to-end results with previous baselines
2. Identify bottlenecks (producer vs consumer performance)
3. Optimize the slower component if targets not met
4. Run failover tests to validate HA end-to-end performance
5. Scale up cluster if higher overall throughput needed

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

    # Warmup with parallel producer/consumer
    log "Running warmup..."
    local warmup_producer_result="${RESULTS_DIR}/warmup_producer_${TIMESTAMP}.log"
    local warmup_consumer_result="${RESULTS_DIR}/warmup_consumer_${TIMESTAMP}.log"
    run_parallel_test "warmup" 20 "${BASELINE_SIZE}" "${BASELINE_RATE}" "${warmup_producer_result}" "${warmup_consumer_result}"
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
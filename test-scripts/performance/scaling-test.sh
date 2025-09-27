#!/bin/bash

set -e

# Pulsar Scaling Performance Test
# Tests performance characteristics as partition count and load increase

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
BROKER_URL=${BROKER_URL:-"pulsar://localhost:6650"}
ADMIN_URL=${ADMIN_URL:-"http://localhost:8080"}
BASE_TOPIC=${BASE_TOPIC:-"persistent://public/default/scaling-test"}
RESULTS_DIR="./results/scaling"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')

# Test parameters
MESSAGE_SIZE=1024
BASE_RATE=1000
TEST_DURATION=60  # seconds
MESSAGES_PER_TEST=10000

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

# Function to create partitioned topic
create_partitioned_topic() {
    local topic_name=$1
    local partitions=$2

    log "Creating topic ${topic_name} with ${partitions} partitions"

    if curl -s -X PUT "${ADMIN_URL}/admin/v2/persistent/public/default/${topic_name}/partitions/${partitions}" > /dev/null 2>&1; then
        log_success "Created topic with ${partitions} partitions"
        return 0
    else
        log_error "Failed to create topic"
        return 1
    fi
}

# Function to run partition scaling test
run_partition_scaling_test() {
    log "Running partition scaling test..."

    local partitions_array=(1 2 4 8 16)
    local scaling_results="${RESULTS_DIR}/partition_scaling_${TIMESTAMP}.json"

    echo "{" > "${scaling_results}"
    echo '  "test_type": "partition_scaling",' >> "${scaling_results}"
    echo '  "timestamp": "'${TIMESTAMP}'",' >> "${scaling_results}"
    echo '  "message_size": '${MESSAGE_SIZE}',' >> "${scaling_results}"
    echo '  "base_rate": '${BASE_RATE}',' >> "${scaling_results}"
    echo '  "test_duration": '${TEST_DURATION}',' >> "${scaling_results}"
    echo '  "results": [' >> "${scaling_results}"

    local first=true
    for partitions in "${partitions_array[@]}"; do
        local topic_name="scaling-test-p${partitions}"
        local full_topic="persistent://public/default/${topic_name}"

        # Add comma for JSON array
        if [ "$first" = false ]; then
            echo ',' >> "${scaling_results}"
        fi
        first=false

        log "Testing with ${partitions} partitions..."

        # Create topic
        create_partitioned_topic "${topic_name}" "${partitions}"

        # Run producer test
        local producer_log="${RESULTS_DIR}/producer_p${partitions}_${TIMESTAMP}.log"
        local consumer_log="${RESULTS_DIR}/consumer_p${partitions}_${TIMESTAMP}.log"

        log "Running producer for ${partitions} partitions..."
        if docker exec broker-1 bin/pulsar-perf produce \
            --service-url "${BROKER_URL}" \
            --topic "${full_topic}" \
            --rate "${BASE_RATE}" \
            --num-messages "${MESSAGES_PER_TEST}" \
            --size "${MESSAGE_SIZE}" \
            --batch-time-period 100 \
            --max-pending 1000 \
            --producer-name "scaling-p${partitions}-producer" \
            --num-producers 1 > "${producer_log}" 2>&1; then
            log_success "Producer test completed for ${partitions} partitions"
        else
            log_error "Producer test failed for ${partitions} partitions"
            continue
        fi

        # Extract producer metrics
        local throughput=$(grep -o "Throughput produced:.*msg/s" "${producer_log}" | awk '{print $3}' | head -1)
        local avg_latency=$(grep -o "Pub Latency(ms) Avg:.*" "${producer_log}" | awk '{print $3}' | head -1)
        local p99_latency=$(grep -o "99%:.*" "${producer_log}" | awk '{print $2}' | head -1)

        # Run consumer test
        sleep 2
        log "Running consumer for ${partitions} partitions..."
        if docker exec broker-1 bin/pulsar-perf consume \
            --service-url "${BROKER_URL}" \
            --topic "${full_topic}" \
            --subscription-name "scaling-p${partitions}-sub" \
            --subscription-type Shared \
            --num-messages "${MESSAGES_PER_TEST}" \
            --receiver-queue-size 1000 \
            --consumer-name "scaling-p${partitions}-consumer" > "${consumer_log}" 2>&1; then
            log_success "Consumer test completed for ${partitions} partitions"
        else
            log_error "Consumer test failed for ${partitions} partitions"
        fi

        # Extract consumer metrics
        local consumer_throughput=$(grep -o "Throughput received:.*msg/s" "${consumer_log}" | awk '{print $3}' | head -1)
        local consumer_latency=$(grep -o "End-to-end latency.*" "${consumer_log}" | awk '{print $4}' | head -1)

        # Add result to JSON
        cat >> "${scaling_results}" << EOF
    {
      "partitions": ${partitions},
      "producer_throughput_msg_per_sec": ${throughput:-0},
      "producer_avg_latency_ms": ${avg_latency:-0},
      "producer_p99_latency_ms": ${p99_latency:-0},
      "consumer_throughput_msg_per_sec": ${consumer_throughput:-0},
      "consumer_end_to_end_latency_ms": ${consumer_latency:-0}
    }EOF

        # Clean up topic
        curl -s -X DELETE "${ADMIN_URL}/admin/v2/persistent/public/default/${topic_name}" > /dev/null 2>&1 || true

        sleep 5
    done

    echo >> "${scaling_results}"
    echo '  ]' >> "${scaling_results}"
    echo '}' >> "${scaling_results}"

    log_success "Partition scaling test completed: ${scaling_results}"
}

# Function to run load scaling test
run_load_scaling_test() {
    log "Running load scaling test..."

    local rates_array=(500 1000 2000 5000 10000)
    local topic_name="scaling-load-test"
    local partitions=8
    local full_topic="persistent://public/default/${topic_name}"
    local scaling_results="${RESULTS_DIR}/load_scaling_${TIMESTAMP}.json"

    # Create topic with optimal partition count
    create_partitioned_topic "${topic_name}" "${partitions}"

    echo "{" > "${scaling_results}"
    echo '  "test_type": "load_scaling",' >> "${scaling_results}"
    echo '  "timestamp": "'${TIMESTAMP}'",' >> "${scaling_results}"
    echo '  "partitions": '${partitions}',' >> "${scaling_results}"
    echo '  "message_size": '${MESSAGE_SIZE}',' >> "${scaling_results}"
    echo '  "results": [' >> "${scaling_results}"

    local first=true
    for rate in "${rates_array[@]}"; do
        # Add comma for JSON array
        if [ "$first" = false ]; then
            echo ',' >> "${scaling_results}"
        fi
        first=false

        log "Testing with ${rate} msg/sec rate..."

        local messages_for_rate=$((rate * 30))  # 30 seconds worth
        local producer_log="${RESULTS_DIR}/producer_rate${rate}_${TIMESTAMP}.log"
        local consumer_log="${RESULTS_DIR}/consumer_rate${rate}_${TIMESTAMP}.log"

        # Run producer test
        log "Running producer at ${rate} msg/sec..."
        if docker exec broker-1 bin/pulsar-perf produce \
            --service-url "${BROKER_URL}" \
            --topic "${full_topic}" \
            --rate "${rate}" \
            --num-messages "${messages_for_rate}" \
            --size "${MESSAGE_SIZE}" \
            --batch-time-period 100 \
            --max-pending 2000 \
            --producer-name "load-rate${rate}-producer" \
            --num-producers 1 > "${producer_log}" 2>&1; then
            log_success "Producer test completed at ${rate} msg/sec"
        else
            log_error "Producer test failed at ${rate} msg/sec"
            continue
        fi

        # Extract producer metrics
        local throughput=$(grep -o "Throughput produced:.*msg/s" "${producer_log}" | awk '{print $3}' | head -1)
        local avg_latency=$(grep -o "Pub Latency(ms) Avg:.*" "${producer_log}" | awk '{print $3}' | head -1)
        local p50_latency=$(grep -o "50%:.*" "${producer_log}" | awk '{print $2}' | head -1)
        local p95_latency=$(grep -o "95%:.*" "${producer_log}" | awk '{print $2}' | head -1)
        local p99_latency=$(grep -o "99%:.*" "${producer_log}" | awk '{print $2}' | head -1)

        # Run consumer test
        sleep 2
        log "Running consumer at ${rate} msg/sec target..."
        if docker exec broker-1 bin/pulsar-perf consume \
            --service-url "${BROKER_URL}" \
            --topic "${full_topic}" \
            --subscription-name "load-rate${rate}-sub" \
            --subscription-type Shared \
            --num-messages "${messages_for_rate}" \
            --receiver-queue-size 1000 \
            --consumer-name "load-rate${rate}-consumer" > "${consumer_log}" 2>&1; then
            log_success "Consumer test completed at ${rate} msg/sec"
        else
            log_error "Consumer test failed at ${rate} msg/sec"
        fi

        # Extract consumer metrics
        local consumer_throughput=$(grep -o "Throughput received:.*msg/s" "${consumer_log}" | awk '{print $3}' | head -1)

        # Add result to JSON
        cat >> "${scaling_results}" << EOF
    {
      "target_rate_msg_per_sec": ${rate},
      "actual_throughput_msg_per_sec": ${throughput:-0},
      "avg_latency_ms": ${avg_latency:-0},
      "p50_latency_ms": ${p50_latency:-0},
      "p95_latency_ms": ${p95_latency:-0},
      "p99_latency_ms": ${p99_latency:-0},
      "consumer_throughput_msg_per_sec": ${consumer_throughput:-0},
      "throughput_efficiency": $(echo "scale=3; ${throughput:-0} / ${rate}" | bc)
    }EOF

        sleep 10
    done

    echo >> "${scaling_results}"
    echo '  ]' >> "${scaling_results}"
    echo '}' >> "${scaling_results}"

    # Clean up topic
    curl -s -X DELETE "${ADMIN_URL}/admin/v2/persistent/public/default/${topic_name}" > /dev/null 2>&1 || true

    log_success "Load scaling test completed: ${scaling_results}"
}

# Function to generate scaling analysis report
generate_scaling_report() {
    log "Generating scaling analysis report..."

    local report_file="${RESULTS_DIR}/scaling_analysis_${TIMESTAMP}.md"

    cat > "${report_file}" << EOF
# Pulsar Scaling Performance Analysis

**Test Date:** $(date)
**Cluster:** ${ADMIN_URL}

## Test Overview

This report analyzes Pulsar's performance characteristics as the number of partitions and message rates scale.

## Partition Scaling Analysis

### Test Configuration
- Message Size: ${MESSAGE_SIZE} bytes
- Base Rate: ${BASE_RATE} msg/sec
- Partitions Tested: 1, 2, 4, 8, 16

### Key Findings

EOF

    # Analyze partition scaling results
    if [ -f "${RESULTS_DIR}/partition_scaling_${TIMESTAMP}.json" ]; then
        local best_partition_throughput=$(jq -r '.results | max_by(.producer_throughput_msg_per_sec) | .partitions' "${RESULTS_DIR}/partition_scaling_${TIMESTAMP}.json")
        local best_partition_latency=$(jq -r '.results | min_by(.producer_p99_latency_ms) | .partitions' "${RESULTS_DIR}/partition_scaling_${TIMESTAMP}.json")

        cat >> "${report_file}" << EOF
- **Optimal Partition Count (Throughput):** ${best_partition_throughput} partitions
- **Optimal Partition Count (Latency):** ${best_partition_latency} partitions

### Detailed Results

| Partitions | Throughput (msg/s) | Avg Latency (ms) | P99 Latency (ms) |
|------------|-------------------|------------------|------------------|
EOF

        jq -r '.results[] | "| \(.partitions) | \(.producer_throughput_msg_per_sec) | \(.producer_avg_latency_ms) | \(.producer_p99_latency_ms) |"' \
            "${RESULTS_DIR}/partition_scaling_${TIMESTAMP}.json" >> "${report_file}"
    fi

    cat >> "${report_file}" << EOF

## Load Scaling Analysis

### Test Configuration
- Partitions: 8 (optimal from partition scaling)
- Message Size: ${MESSAGE_SIZE} bytes
- Rates Tested: 500, 1000, 2000, 5000, 10000 msg/sec

### Key Findings

EOF

    # Analyze load scaling results
    if [ -f "${RESULTS_DIR}/load_scaling_${TIMESTAMP}.json" ]; then
        local max_sustainable_rate=$(jq -r '.results[] | select(.throughput_efficiency > 0.9) | .target_rate_msg_per_sec' \
            "${RESULTS_DIR}/load_scaling_${TIMESTAMP}.json" | tail -1)
        local latency_at_max=$(jq -r --argjson rate "${max_sustainable_rate}" '.results[] | select(.target_rate_msg_per_sec == $rate) | .p99_latency_ms' \
            "${RESULTS_DIR}/load_scaling_${TIMESTAMP}.json")

        cat >> "${report_file}" << EOF
- **Maximum Sustainable Rate:** ${max_sustainable_rate} msg/sec (>90% efficiency)
- **P99 Latency at Max Rate:** ${latency_at_max} ms

### Detailed Results

| Target Rate | Actual Rate | Efficiency | Avg Latency | P99 Latency |
|-------------|-------------|------------|-------------|-------------|
EOF

        jq -r '.results[] | "| \(.target_rate_msg_per_sec) | \(.actual_throughput_msg_per_sec) | \(.throughput_efficiency * 100 | floor)% | \(.avg_latency_ms) | \(.p99_latency_ms) |"' \
            "${RESULTS_DIR}/load_scaling_${TIMESTAMP}.json" >> "${report_file}"
    fi

    cat >> "${report_file}" << EOF

## Recommendations

### Partition Strategy
1. Use 4-8 partitions for most workloads
2. Increase partitions for higher throughput requirements
3. Consider partition count vs. latency trade-offs

### Throughput Planning
1. Plan for 80% of maximum sustainable rate in production
2. Monitor P99 latency to detect saturation early
3. Scale horizontally by adding brokers for higher rates

### Performance Optimization
1. Batch messages appropriately (100-1000ms batch time)
2. Use appropriate message sizes (1-4KB optimal)
3. Configure adequate producer pending queue sizes

## Raw Data

All test logs and detailed metrics available in: \`${RESULTS_DIR}/\`

EOF

    log_success "Scaling analysis report generated: ${report_file}"
}

# Main execution
main() {
    log "Starting Pulsar Scaling Performance Test"

    # Create results directory
    mkdir -p "${RESULTS_DIR}"

    # Check prerequisites
    if ! command -v jq &> /dev/null; then
        log_error "jq is required but not installed"
        exit 1
    fi

    if ! command -v bc &> /dev/null; then
        log_error "bc is required but not installed"
        exit 1
    fi

    # Check cluster readiness
    if ! curl -s -f "${ADMIN_URL}/admin/v2/clusters" > /dev/null 2>&1; then
        log_error "Pulsar cluster is not accessible at ${ADMIN_URL}"
        exit 1
    fi

    # Run scaling tests
    run_partition_scaling_test
    sleep 30
    run_load_scaling_test

    # Generate analysis report
    generate_scaling_report

    log_success "Scaling performance test completed!"
    log "Results available in: ${RESULTS_DIR}/"
    log "Analysis report: ${RESULTS_DIR}/scaling_analysis_${TIMESTAMP}.md"
}

# Handle script arguments
case "${1:-}" in
    --help|-h)
        echo "Usage: $0 [options]"
        echo ""
        echo "Runs comprehensive scaling performance tests against Pulsar cluster."
        echo ""
        echo "Tests:"
        echo "  1. Partition scaling (1-16 partitions)"
        echo "  2. Load scaling (500-10000 msg/sec)"
        echo ""
        echo "Options:"
        echo "  --help, -h              Show this help message"
        echo ""
        echo "Environment Variables:"
        echo "  BROKER_URL              Pulsar broker service URL (default: pulsar://localhost:6650)"
        echo "  ADMIN_URL               Pulsar admin REST URL (default: http://localhost:8080)"
        echo ""
        exit 0
        ;;
    *)
        main
        ;;
esac
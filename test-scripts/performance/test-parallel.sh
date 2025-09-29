#!/usr/bin/env bash

# Quick test of the parallel producer/consumer functionality
set -e

# Configuration
BROKER_URL=${BROKER_URL:-"pulsar://localhost:6650"}
TEST_TOPIC="persistent://public/default/quick-test"
TEST_SUBSCRIPTION="quick-sub"
RESULTS_DIR="./test-results"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')

mkdir -p "${RESULTS_DIR}"

# Log function
log() {
    echo -e "\033[0;34m[$(date '+%H:%M:%S')]\033[0m $1"
}

log_success() {
    echo -e "\033[0;32m[$(date '+%H:%M:%S')] ✓\033[0m $1"
}

log_error() {
    echo -e "\033[0;31m[$(date '+%H:%M:%S')] ✗\033[0m $1"
}

log "Testing parallel producer/consumer with 10-second duration"

# Check if Docker and broker are available
if ! docker ps --format "table {{.Names}}" | grep -q "broker-1"; then
    log_error "broker-1 container not found"
    exit 1
fi

# Create subscription name with PID to avoid conflicts
subscription_name="${TEST_SUBSCRIPTION}-$$"

# Start consumer first
log "Starting consumer (10s duration)..."
timeout 20s docker exec broker-1 bin/pulsar-perf consume \
    -u "${BROKER_URL}" \
    -ss "${subscription_name}" \
    -st Shared \
    -time 10 \
    -sp Earliest \
    -q 1000 \
    "${TEST_TOPIC}" \
    > "${RESULTS_DIR}/consumer_${TIMESTAMP}.log" 2>&1 &

consumer_pid=$!

# Wait for consumer to initialize
sleep 2

# Start producer
log "Starting producer (10s duration)..."
timeout 20s docker exec broker-1 bin/pulsar-perf produce \
    -u "${BROKER_URL}" \
    -time 10 \
    -r 1000 \
    -s 512 \
    -b 100 \
    -o 1000 \
    -pn "quick-producer" \
    "${TEST_TOPIC}" \
    > "${RESULTS_DIR}/producer_${TIMESTAMP}.log" 2>&1 &

producer_pid=$!

# Wait for both to complete with progress indicator
log "Waiting for producer and consumer to complete..."

# Show progress while tests are running
elapsed=0
while kill -0 $producer_pid 2>/dev/null || kill -0 $consumer_pid 2>/dev/null; do
    printf "\r  ⏳ Test running for ${elapsed}s (target: 10s)..."
    sleep 1
    elapsed=$((elapsed + 1))

    # Force kill after reasonable time
    if [ $elapsed -gt 15 ]; then
        printf "\n"
        log "  Forcing completion after 15s..."
        kill $producer_pid 2>/dev/null
        kill $consumer_pid 2>/dev/null
        sleep 2
        break
    fi
done
printf "\n"

# Get exit codes (don't wait forever)
wait $producer_pid 2>/dev/null
producer_exit_code=$?

wait $consumer_pid 2>/dev/null
consumer_exit_code=$?

log "Tests completed - Producer: code $producer_exit_code, Consumer: code $consumer_exit_code"

# Always show results section (even if tests didn't exit cleanly)
echo ""
echo "=== PERFORMANCE TEST RESULTS ==="
echo ""

# Check if tests completed successfully
if [ $producer_exit_code -eq 0 ] && [ $consumer_exit_code -eq 0 ]; then
    log_success "Both producer and consumer completed successfully"
elif [ $producer_exit_code -ne 0 ]; then
    log_error "Producer had issues (exit code: $producer_exit_code)"
elif [ $consumer_exit_code -ne 0 ]; then
    log_error "Consumer had issues (exit code: $consumer_exit_code)"
fi

# Producer Results (always try to show, regardless of exit codes)
if [ -s "${RESULTS_DIR}/producer_${TIMESTAMP}.log" ]; then
    log_success "Producer Results:"

    # Extract producer metrics
    producer_throughput=$(grep "Throughput produced:" "${RESULTS_DIR}/producer_${TIMESTAMP}.log" | tail -1 | grep -o "[0-9.]\+ msg/s" | head -1 | awk '{print $1}')
    producer_latency_mean=$(grep "Throughput produced:" "${RESULTS_DIR}/producer_${TIMESTAMP}.log" | tail -1 | grep -o "mean:[[:space:]]*[0-9.]\+" | head -1 | awk '{print $2}')
    producer_latency_p99=$(grep "Throughput produced:" "${RESULTS_DIR}/producer_${TIMESTAMP}.log" | tail -1 | grep -o "99pct:[[:space:]]*[0-9.]\+" | head -1 | awk '{print $2}')

    echo "  📈 Throughput: ${producer_throughput:-"N/A"} messages/sec"
    echo "  ⏱️  Mean Latency: ${producer_latency_mean:-"N/A"} ms"
    echo "  ⏱️  P99 Latency: ${producer_latency_p99:-"N/A"} ms"
    echo ""
else
    log_error "Producer log file not found or empty"
fi

# Consumer Results
if [ -s "${RESULTS_DIR}/consumer_${TIMESTAMP}.log" ]; then
    log_success "Consumer Results:"

    # Extract consumer metrics
    consumer_throughput=$(grep "Aggregated throughput stats" "${RESULTS_DIR}/consumer_${TIMESTAMP}.log" | grep -o "[0-9.]\+ msg/s" | head -1 | awk '{print $1}')
    consumer_messages=$(grep "Aggregated throughput stats" "${RESULTS_DIR}/consumer_${TIMESTAMP}.log" | grep -o "[0-9]\+ records received" | head -1 | awk '{print $1}')
    consumer_latency_mean=$(grep "Aggregated latency stats" "${RESULTS_DIR}/consumer_${TIMESTAMP}.log" | grep -o "mean:[[:space:]]*[0-9.]\+" | head -1 | awk '{print $2}')
    consumer_latency_p99=$(grep "Aggregated latency stats" "${RESULTS_DIR}/consumer_${TIMESTAMP}.log" | grep -o "99pct:[[:space:]]*[0-9.]\+" | head -1 | awk '{print $2}')

    echo "  📈 Throughput: ${consumer_throughput:-"N/A"} messages/sec"
    echo "  📦 Messages Consumed: ${consumer_messages:-"N/A"}"
    echo "  ⏱️  Mean Latency: ${consumer_latency_mean:-"N/A"} ms"
    echo "  ⏱️  P99 Latency: ${consumer_latency_p99:-"N/A"} ms"
    echo ""
else
    log_error "Consumer log file not found or empty"
fi

# End-to-End Analysis (if we have metrics from both)
if [[ -n "$producer_throughput" && -n "$consumer_throughput" ]]; then
    min_throughput=$(echo "$producer_throughput $consumer_throughput" | awk '{print ($1 < $2) ? $1 : $2}')
    bottleneck=$(echo "$producer_throughput $consumer_throughput" | awk '{print ($1 < $2) ? "Producer" : "Consumer"}')

    echo "=== END-TO-END ANALYSIS ==="
    echo "  🚀 Overall Throughput: $min_throughput messages/sec"
    echo "  🔍 Bottleneck: $bottleneck"
    echo "  📊 Producer Utilization: $(echo "$producer_throughput $min_throughput" | awk '{printf "%.1f%%", ($2/$1)*100}' 2>/dev/null || echo "N/A")"
    echo "  📊 Consumer Utilization: $(echo "$consumer_throughput $min_throughput" | awk '{printf "%.1f%%", ($2/$1)*100}' 2>/dev/null || echo "N/A")"
    echo ""
fi

echo "=== SUMMARY ==="
if [ $producer_exit_code -eq 0 ] && [ $consumer_exit_code -eq 0 ]; then
    echo "  ✅ Test Status: SUCCESS"
else
    echo "  ⚠️  Test Status: COMPLETED WITH ISSUES"
fi
echo "  📁 Producer log: ${RESULTS_DIR}/producer_${TIMESTAMP}.log"
echo "  📁 Consumer log: ${RESULTS_DIR}/consumer_${TIMESTAMP}.log"
echo ""

log_success "✅ Parallel producer/consumer test completed!"
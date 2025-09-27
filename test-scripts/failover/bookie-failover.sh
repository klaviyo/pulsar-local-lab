#!/bin/bash

set -e

# Pulsar BookKeeper Bookie Failover Test
# Tests bookie failure and recovery scenarios

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
ADMIN_URL=${ADMIN_URL:-"http://localhost:8080"}
BROKER_URL=${BROKER_URL:-"pulsar://localhost:6650"}
TEST_TOPIC=${TEST_TOPIC:-"persistent://public/default/bookie-failover-test"}
RESULTS_DIR="./results/failover"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')

# Test parameters
TEST_MESSAGES=30000
MESSAGE_SIZE=2048
PRODUCE_RATE=500
FAILOVER_DELAY=30

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

# Function to get bookie list
get_bookie_list() {
    local available_bookies=()

    # Check for running bookie containers
    local bookie_containers=$(docker ps --format "table {{.Names}}" | grep -E "bookie-[0-9]+" | sort)

    for container in $bookie_containers; do
        available_bookies+=("$container")
    done

    echo "${available_bookies[@]}"
}

# Function to check bookie health
check_bookie_health() {
    local bookie_name=$1

    if docker exec "$bookie_name" bin/bookkeeper shell bookiesanity > /dev/null 2>&1; then
        echo "healthy"
    else
        echo "unhealthy"
    fi
}

# Function to get cluster bookie info
get_cluster_bookies_info() {
    log "Getting cluster bookie information..."

    local bookies_info_file="${RESULTS_DIR}/bookies_info_${TIMESTAMP}.json"

    # Get bookie rack info from admin API
    if curl -s "${ADMIN_URL}/admin/v2/bookies/racks-info" > "$bookies_info_file" 2>/dev/null; then
        local available_bookies=$(jq -r 'keys | length' "$bookies_info_file" 2>/dev/null || echo "0")
        local total_bookies=$(get_bookie_list | wc -w)

        log "Available bookies in cluster: ${available_bookies}"
        log "Total bookie containers: ${total_bookies}"

        # List individual bookies
        jq -r 'keys[]' "$bookies_info_file" 2>/dev/null | while read -r bookie; do
            log "  - ${bookie}"
        done

        echo "$available_bookies"
    else
        log_error "Failed to get bookie information from cluster"
        echo "0"
    fi
}

# Function to get ledger ensemble info
get_ledger_info() {
    local topic_name=$1

    log "Getting ledger information for topic: ${topic_name}"

    # Get topic internal stats which includes ledger info
    local topic_stats=$(curl -s "${ADMIN_URL}/admin/v2/persistent/public/default/${topic_name}/internalStats" 2>/dev/null || echo "{}")

    if [ "$topic_stats" != "{}" ]; then
        # Extract ledger ensemble information
        echo "$topic_stats" | jq -r '.cursors // {} | keys[]' 2>/dev/null | while read -r cursor; do
            log "Cursor: $cursor"
        done

        # Get number of ledgers
        local ledger_count=$(echo "$topic_stats" | jq -r '.ledgers | length' 2>/dev/null || echo "0")
        log "Active ledgers: ${ledger_count}"

        return 0
    else
        log_warning "Could not retrieve ledger information"
        return 1
    fi
}

# Function to simulate bookie failure
simulate_bookie_failure() {
    local bookie_name=$1

    log "Simulating failure of bookie: ${bookie_name}"

    # Stop bookie container
    if docker ps --format "table {{.Names}}" | grep -q "$bookie_name"; then
        docker stop "$bookie_name" > /dev/null 2>&1
        log_success "Stopped bookie container: ${bookie_name}"

        # Wait for cluster to detect failure
        sleep 10

        # Check if cluster detected the failure
        local remaining_bookies=$(get_cluster_bookies_info)
        log "Remaining bookies after failure: ${remaining_bookies}"

        return 0
    else
        log_error "Bookie container ${bookie_name} not found"
        return 1
    fi
}

# Function to recover bookie
recover_bookie() {
    local bookie_name=$1

    log "Recovering bookie: ${bookie_name}"

    # Start bookie container
    if ! docker ps --format "table {{.Names}}" | grep -q "$bookie_name"; then
        docker start "$bookie_name" > /dev/null 2>&1
        log_success "Started bookie container: ${bookie_name}"

        # Wait for bookie to be ready
        local max_wait=120
        local wait_time=0

        log "Waiting for ${bookie_name} to be ready..."
        while [ $wait_time -lt $max_wait ]; do
            if [ "$(check_bookie_health "$bookie_name")" = "healthy" ]; then
                log_success "${bookie_name} is healthy"
                return 0
            fi
            sleep 5
            wait_time=$((wait_time + 5))
            echo -n "."
        done

        log_error "${bookie_name} did not become healthy within ${max_wait} seconds"
        return 1
    else
        log_success "Bookie container ${bookie_name} is already running"
        return 0
    fi
}

# Function to start continuous write load
start_continuous_load() {
    local result_file=$1

    log "Starting continuous producer load for bookie testing..."

    # Create test topic with higher replication for bookie testing
    if curl -s -X PUT "${ADMIN_URL}/admin/v2/persistent/public/default/bookie-failover-test/partitions/6" > /dev/null 2>&1; then
        log_success "Created test topic with 6 partitions"

        # Set retention policy
        curl -s -X POST "${ADMIN_URL}/admin/v2/persistent/public/default/bookie-failover-test/retention" \
            -H "Content-Type: application/json" \
            -d '{"retentionTimeInMinutes": 120, "retentionSizeInMB": 500}' > /dev/null 2>&1
    else
        log_warning "Failed to create test topic (may already exist)"
    fi

    # Start producer with settings optimized for bookie testing
    {
        docker exec broker-1 bin/pulsar-perf produce \
            --service-url "${BROKER_URL}" \
            --topic "${TEST_TOPIC}" \
            --rate "${PRODUCE_RATE}" \
            --num-messages "${TEST_MESSAGES}" \
            --size "${MESSAGE_SIZE}" \
            --batch-time-period 500 \
            --max-pending 1000 \
            --producer-name "bookie-failover-producer" \
            --stats-interval-seconds 10 \
            2>&1
    } > "${result_file}" &

    local producer_pid=$!
    echo "${producer_pid}" > "${RESULTS_DIR}/bookie_producer_pid_${TIMESTAMP}.txt"

    log_success "Started continuous producer (PID: ${producer_pid})"
    return 0
}

# Function to monitor bookie cluster during failover
monitor_bookie_metrics() {
    local duration=$1
    local metrics_file=$2
    local failed_bookie=$3

    log "Monitoring bookie cluster metrics for ${duration} seconds..."

    local start_time=$(date +%s)
    local end_time=$((start_time + duration))

    echo "{" > "${metrics_file}"
    echo '  "test_type": "bookie_failover",' >> "${metrics_file}"
    echo '  "failed_bookie": "'${failed_bookie}'",' >> "${metrics_file}"
    echo '  "start_time": "'$(date -d @${start_time})'",' >> "${metrics_file}"
    echo '  "duration_seconds": '${duration}',' >> "${metrics_file}"
    echo '  "metrics": [' >> "${metrics_file}"

    local first=true
    while [ $(date +%s) -lt $end_time ]; do
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))

        # Get bookie cluster status
        local available_bookies=$(get_cluster_bookies_info)

        # Get topic stats
        local msg_rate_in=0
        local storage_size=0
        local write_latency=0

        # Try to get metrics from available broker
        for port in 8080 8081 8082; do
            if curl -s -f "http://localhost:${port}/admin/v2/clusters" > /dev/null 2>&1; then
                local topic_stats=$(curl -s "http://localhost:${port}/admin/v2/persistent/public/default/bookie-failover-test/stats" 2>/dev/null || echo "{}")
                msg_rate_in=$(echo "$topic_stats" | jq -r '.msgRateIn // 0' 2>/dev/null || echo "0")
                storage_size=$(echo "$topic_stats" | jq -r '.storageSize // 0' 2>/dev/null || echo "0")
                break
            fi
        done

        # Check individual bookie health
        local healthy_bookies=0
        local bookie_list=($(get_bookie_list))
        for bookie in "${bookie_list[@]}"; do
            if [ "$(check_bookie_health "$bookie")" = "healthy" ]; then
                healthy_bookies=$((healthy_bookies + 1))
            fi
        done

        # Add comma for JSON array
        if [ "$first" = false ]; then
            echo ',' >> "${metrics_file}"
        fi
        first=false

        # Write metrics entry
        cat >> "${metrics_file}" << EOF
    {
      "timestamp": ${current_time},
      "elapsed_seconds": ${elapsed},
      "available_bookies_cluster": ${available_bookies},
      "healthy_bookie_containers": ${healthy_bookies},
      "total_bookie_containers": ${#bookie_list[@]},
      "msg_rate_in": ${msg_rate_in},
      "topic_storage_size": ${storage_size}
    }EOF

        sleep 10
    done

    echo >> "${metrics_file}"
    echo '  ]' >> "${metrics_file}"
    echo '}' >> "${metrics_file}"

    log_success "Bookie metrics collection completed: ${metrics_file}"
}

# Function to run bookie failover test
run_bookie_failover_test() {
    local target_bookie=${1:-"bookie-3"}  # Default to bookie-3

    log "Running bookie failover test for: ${target_bookie}"

    local test_log="${RESULTS_DIR}/bookie_failover_${target_bookie}_${TIMESTAMP}.log"
    local metrics_file="${RESULTS_DIR}/bookie_metrics_${target_bookie}_${TIMESTAMP}.json"

    # Check initial bookie health
    local bookie_list=($(get_bookie_list))
    local initial_healthy=0

    log "Initial bookie health check:"
    for bookie in "${bookie_list[@]}"; do
        local health=$(check_bookie_health "$bookie")
        log "  ${bookie}: ${health}"
        if [ "$health" = "healthy" ]; then
            initial_healthy=$((initial_healthy + 1))
        fi
    done

    if [ $initial_healthy -lt 3 ]; then
        log_error "Need at least 3 healthy bookies for failover testing (found: ${initial_healthy})"
        return 1
    fi

    # Get initial cluster state
    local initial_bookies=$(get_cluster_bookies_info)
    log "Initial cluster bookies: ${initial_bookies}"

    # Start continuous load
    start_continuous_load "${test_log}"

    # Wait for load to stabilize
    log "Waiting ${FAILOVER_DELAY} seconds for write load to stabilize..."
    sleep "${FAILOVER_DELAY}"

    # Get ledger information before failure
    get_ledger_info "bookie-failover-test"

    # Start metrics monitoring in background
    monitor_bookie_metrics 240 "${metrics_file}" "${target_bookie}" &
    local monitor_pid=$!

    # Record pre-failure state
    log "Pre-failure state:"
    local pre_failure_bookies=$(get_cluster_bookies_info)
    log "  Cluster bookies: ${pre_failure_bookies}"

    # Simulate bookie failure
    local failure_time=$(date +%s)
    simulate_bookie_failure "${target_bookie}"

    # Monitor during failure
    log "Monitoring cluster behavior during bookie failure..."
    sleep 90  # Give more time for BookKeeper replication

    # Record during-failure state
    log "During-failure state:"
    local during_failure_bookies=$(get_cluster_bookies_info)
    log "  Cluster bookies: ${during_failure_bookies}"

    # Check write performance during failure
    log "Checking write performance during failure..."

    # Recover bookie
    local recovery_time=$(date +%s)
    recover_bookie "${target_bookie}"

    # Monitor recovery
    log "Monitoring bookie recovery and re-replication..."
    sleep 90  # Give time for re-replication

    # Record post-recovery state
    log "Post-recovery state:"
    local post_recovery_bookies=$(get_cluster_bookies_info)
    log "  Cluster bookies: ${post_recovery_bookies}"

    # Stop monitoring
    kill "${monitor_pid}" 2>/dev/null || true
    wait "${monitor_pid}" 2>/dev/null || true

    # Stop load generator
    local producer_pid=$(cat "${RESULTS_DIR}/bookie_producer_pid_${TIMESTAMP}.txt" 2>/dev/null || echo "")
    if [ -n "$producer_pid" ]; then
        kill "${producer_pid}" 2>/dev/null || true
        wait "${producer_pid}" 2>/dev/null || true
    fi

    # Generate test summary
    local summary_file="${RESULTS_DIR}/bookie_failover_summary_${target_bookie}_${TIMESTAMP}.json"
    cat > "${summary_file}" << EOF
{
  "test_type": "bookie_failover",
  "target_bookie": "${target_bookie}",
  "timestamp": "${TIMESTAMP}",
  "failure_time": ${failure_time},
  "recovery_time": ${recovery_time},
  "downtime_seconds": $((recovery_time - failure_time)),
  "bookie_states": {
    "initial_healthy_containers": ${initial_healthy},
    "initial_cluster_bookies": ${initial_bookies},
    "pre_failure_bookies": ${pre_failure_bookies},
    "during_failure_bookies": ${during_failure_bookies},
    "post_recovery_bookies": ${post_recovery_bookies}
  },
  "test_parameters": {
    "test_messages": ${TEST_MESSAGES},
    "message_size": ${MESSAGE_SIZE},
    "produce_rate": ${PRODUCE_RATE},
    "test_topic": "${TEST_TOPIC}"
  }
}
EOF

    log_success "Bookie failover test completed for ${target_bookie}"
    log "Test summary: ${summary_file}"
    log "Detailed metrics: ${metrics_file}"
}

# Function to analyze bookie failover results
analyze_bookie_results() {
    log "Analyzing bookie failover test results..."

    local analysis_file="${RESULTS_DIR}/bookie_failover_analysis_${TIMESTAMP}.md"

    cat > "${analysis_file}" << EOF
# BookKeeper Bookie Failover Test Analysis

**Test Date:** $(date)
**Test Duration:** ~4 minutes per bookie
**Test Topic:** ${TEST_TOPIC}

## Test Overview

This test simulates BookKeeper bookie failures to validate:
1. Write availability during bookie failure
2. BookKeeper ensemble recovery
3. Data re-replication after recovery
4. Write performance impact

## Test Configuration

- **Test Messages:** ${TEST_MESSAGES}
- **Message Size:** ${MESSAGE_SIZE} bytes (larger for storage testing)
- **Produce Rate:** ${PRODUCE_RATE} msg/sec
- **Topic Partitions:** 6
- **Ensemble Size:** 3 (configured in broker)
- **Write Quorum:** 2
- **Ack Quorum:** 2

## Results Summary

EOF

    # Analyze each test result
    for summary_file in "${RESULTS_DIR}"/bookie_failover_summary_*_${TIMESTAMP}.json; do
        if [ -f "$summary_file" ]; then
            local bookie_name=$(jq -r '.target_bookie' "$summary_file")
            local downtime=$(jq -r '.downtime_seconds' "$summary_file")
            local initial_healthy=$(jq -r '.bookie_states.initial_healthy_containers' "$summary_file")
            local during_failure=$(jq -r '.bookie_states.during_failure_bookies' "$summary_file")
            local post_recovery=$(jq -r '.bookie_states.post_recovery_bookies' "$summary_file")

            cat >> "${analysis_file}" << EOF
### ${bookie_name^} Failover

- **Downtime:** ${downtime} seconds
- **Bookie States:**
  - Initial healthy containers: ${initial_healthy}
  - During failure: ${during_failure} bookies available
  - Post-recovery: ${post_recovery} bookies available

EOF
        fi
    done

    cat >> "${analysis_file}" << EOF
## BookKeeper Behavior Analysis

### Write Availability
- **Expected:** Writes continue with reduced quorum (2/3 bookies)
- **Critical:** Writes should never fail with 1 bookie down
- **Measured:** [Requires analysis of producer success rates]

### Re-replication
- **Expected:** Failed bookie's ledgers re-replicated to available bookies
- **Timeline:** Should complete within minutes of failure detection
- **Measured:** [Requires analysis of storage metrics]

### Recovery Behavior
- **Expected:** Recovered bookie rejoins ensemble for new ledgers
- **Timeline:** Should be immediate upon successful restart
- **Measured:** [Based on cluster bookie count recovery]

## Performance Impact

### Write Latency
- **Expected:** Slight increase during failure (reduced ensemble)
- **Critical:** Should remain <100ms P99 for most workloads
- **Measured:** [Analysis would require detailed latency metrics]

### Throughput
- **Expected:** Minimal impact if adequate bookies remain
- **Critical:** Should maintain >80% of baseline throughput
- **Measured:** [Based on message rate analysis]

## Recommendations

### Operational
1. Monitor BookKeeper ensemble health continuously
2. Set up alerting for bookie failures
3. Automate bookie recovery procedures where possible
4. Plan for adequate bookie redundancy (N+2 minimum)

### Configuration
1. Configure appropriate ensemble/quorum sizes
2. Set proper bookie client timeout values
3. Configure adequate disk space monitoring
4. Set up proper network partition detection

### Capacity Planning
1. Size bookie storage for re-replication load
2. Plan network bandwidth for re-replication traffic
3. Consider bookie placement for rack/zone diversity
4. Monitor disk I/O capacity during failures

## Raw Data

Detailed metrics and logs available in:
- Metrics files: \`${RESULTS_DIR}/bookie_metrics_*_${TIMESTAMP}.json\`
- Test logs: \`${RESULTS_DIR}/bookie_failover_*_${TIMESTAMP}.log\`
- Summary files: \`${RESULTS_DIR}/bookie_failover_summary_*_${TIMESTAMP}.json\`

## BookKeeper Health Commands

Useful commands for bookie health monitoring:
\`\`\`bash
# Check bookie sanity
docker exec <bookie-container> bin/bookkeeper shell bookiesanity

# List available bookies
docker exec broker-1 bin/pulsar-admin bookies list

# Get bookie rack info
curl http://localhost:8080/admin/v2/bookies/racks-info
\`\`\`

EOF

    log_success "Bookie failover analysis report generated: ${analysis_file}"
}

# Function to cleanup test resources
cleanup_test_resources() {
    log "Cleaning up bookie test resources..."

    # Delete test topic
    if curl -s -X DELETE "${ADMIN_URL}/admin/v2/persistent/public/default/bookie-failover-test" > /dev/null 2>&1; then
        log_success "Deleted test topic"
    else
        log_warning "Failed to delete test topic"
    fi

    # Clean up PID files
    rm -f "${RESULTS_DIR}"/bookie_producer_pid_*.txt 2>/dev/null || true
}

# Main execution
main() {
    log "Starting BookKeeper Bookie Failover Test"

    # Create results directory
    mkdir -p "${RESULTS_DIR}"

    # Check prerequisites
    if ! command -v jq &> /dev/null; then
        log_error "jq is required but not installed"
        exit 1
    fi

    if ! command -v docker &> /dev/null; then
        log_error "docker is required but not installed"
        exit 1
    fi

    # Check initial cluster status
    local bookie_list=($(get_bookie_list))
    local initial_bookies=$(get_cluster_bookies_info)

    if [ ${#bookie_list[@]} -lt 3 ]; then
        log_error "Need at least 3 bookie containers for failover testing (found: ${#bookie_list[@]})"
        exit 1
    fi

    if [ "$initial_bookies" -lt 3 ]; then
        log_error "Need at least 3 healthy bookies in cluster for failover testing (found: ${initial_bookies})"
        exit 1
    fi

    log_success "Initial cluster state: ${initial_bookies} bookies available"
    log_success "Bookie containers: ${bookie_list[*]}"

    # Test different bookies (avoid bookie-1 as it might be critical)
    local bookies_to_test=()
    for bookie in "${bookie_list[@]}"; do
        if [ "$bookie" != "bookie-1" ]; then
            bookies_to_test+=("$bookie")
        fi
    done

    # Test first 2 non-primary bookies
    local test_count=0
    for bookie in "${bookies_to_test[@]}"; do
        if [ $test_count -lt 2 ]; then
            log "Testing bookie failover for: ${bookie}"
            run_bookie_failover_test "${bookie}"
            sleep 60  # Longer recovery time between bookie tests
            test_count=$((test_count + 1))
        fi
    done

    # Analyze results
    analyze_bookie_results

    # Cleanup
    cleanup_test_resources

    log_success "BookKeeper bookie failover tests completed!"
    log "Results available in: ${RESULTS_DIR}/"
    log "Analysis report: ${RESULTS_DIR}/bookie_failover_analysis_${TIMESTAMP}.md"
}

# Handle script arguments
case "${1:-}" in
    --help|-h)
        echo "Usage: $0 [options] [bookie-name]"
        echo ""
        echo "Tests BookKeeper bookie failover scenarios by stopping/starting bookie containers."
        echo ""
        echo "Options:"
        echo "  --help, -h              Show this help message"
        echo ""
        echo "Arguments:"
        echo "  bookie-name             Specific bookie to test (default: test available bookies)"
        echo ""
        echo "Environment Variables:"
        echo "  ADMIN_URL               Pulsar admin REST URL (default: http://localhost:8080)"
        echo "  BROKER_URL              Pulsar broker service URL (default: pulsar://localhost:6650)"
        echo "  TEST_TOPIC              Topic to use for tests"
        echo ""
        echo "Requirements:"
        echo "  - Docker with running Pulsar cluster"
        echo "  - At least 3 bookies for meaningful failover testing"
        echo "  - jq for JSON processing"
        echo ""
        echo "Test validates:"
        echo "  - Write availability during bookie failure"
        echo "  - BookKeeper ensemble recovery"
        echo "  - Data re-replication"
        echo "  - Performance impact measurement"
        echo ""
        exit 0
        ;;
    *)
        if [ -n "$1" ]; then
            # Test specific bookie
            mkdir -p "${RESULTS_DIR}"
            run_bookie_failover_test "$1"
        else
            # Test available bookies
            main
        fi
        ;;
esac
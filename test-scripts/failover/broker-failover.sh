#!/bin/bash

set -e

# Pulsar Broker Failover Test
# Tests broker failure detection and recovery scenarios

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
ADMIN_URL=${ADMIN_URL:-"http://localhost:8080"}
BROKER_URL=${BROKER_URL:-"pulsar://localhost:6650"}
TEST_TOPIC=${TEST_TOPIC:-"persistent://public/default/failover-test"}
TEST_SUBSCRIPTION=${TEST_SUBSCRIPTION:-"failover-sub"}
RESULTS_DIR="./results/failover"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')

# Test parameters
TEST_MESSAGES=50000
MESSAGE_SIZE=1024
PRODUCE_RATE=1000
FAILOVER_DELAY=30  # seconds

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

# Function to check cluster status
check_cluster_status() {
    local brokers_count=0
    local healthy_brokers=()

    log "Checking cluster status..."

    # Try different broker ports
    local broker_ports=(8080 8081 8082)
    for port in "${broker_ports[@]}"; do
        if curl -s -f "http://localhost:${port}/admin/v2/clusters" > /dev/null 2>&1; then
            brokers_count=$((brokers_count + 1))
            healthy_brokers+=("localhost:${port}")
        fi
    done

    log "Healthy brokers: ${brokers_count} (${healthy_brokers[*]})"
    echo "${brokers_count}"
}

# Function to get broker list
get_broker_list() {
    local admin_url=$1
    local cluster_name=""

    # Get cluster name
    cluster_name=$(curl -s "${admin_url}/admin/v2/clusters" | jq -r '.[0]' 2>/dev/null || echo "")

    if [ -n "$cluster_name" ]; then
        curl -s "${admin_url}/admin/v2/brokers/${cluster_name}" | jq -r '.[]' 2>/dev/null || echo ""
    fi
}

# Function to simulate broker failure
simulate_broker_failure() {
    local broker_name=$1
    local method=${2:-"container"}

    log "Simulating failure of broker: ${broker_name}"

    case $method in
        "container")
            # Stop broker container
            if docker ps --format "table {{.Names}}" | grep -q "${broker_name}"; then
                docker stop "${broker_name}" > /dev/null 2>&1
                log_success "Stopped container: ${broker_name}"
            else
                log_error "Container ${broker_name} not found"
                return 1
            fi
            ;;
        "network")
            # Network partition using iptables (if available)
            log_warning "Network partition simulation not implemented in this version"
            return 1
            ;;
        *)
            log_error "Unknown failure method: ${method}"
            return 1
            ;;
    esac
}

# Function to recover broker
recover_broker() {
    local broker_name=$1
    local method=${2:-"container"}

    log "Recovering broker: ${broker_name}"

    case $method in
        "container")
            # Start broker container
            if ! docker ps --format "table {{.Names}}" | grep -q "${broker_name}"; then
                docker start "${broker_name}" > /dev/null 2>&1
                log_success "Started container: ${broker_name}"

                # Wait for broker to be ready
                local max_wait=120
                local wait_time=0
                local broker_port=""

                case $broker_name in
                    "broker-1") broker_port="8080" ;;
                    "broker-2") broker_port="8081" ;;
                    "broker-3") broker_port="8082" ;;
                esac

                if [ -n "$broker_port" ]; then
                    log "Waiting for ${broker_name} to be ready on port ${broker_port}..."
                    while [ $wait_time -lt $max_wait ]; do
                        if curl -s -f "http://localhost:${broker_port}/admin/v2/clusters" > /dev/null 2>&1; then
                            log_success "${broker_name} is ready"
                            return 0
                        fi
                        sleep 5
                        wait_time=$((wait_time + 5))
                        echo -n "."
                    done
                    log_error "${broker_name} did not become ready within ${max_wait} seconds"
                    return 1
                fi
            else
                log_success "Container ${broker_name} is already running"
            fi
            ;;
        *)
            log_error "Unknown recovery method: ${method}"
            return 1
            ;;
    esac
}

# Function to start continuous load
start_continuous_load() {
    local result_file=$1

    log "Starting continuous producer load..."

    # Create test topic
    if curl -s -X PUT "${ADMIN_URL}/admin/v2/persistent/public/default/failover-test/partitions/4" > /dev/null 2>&1; then
        log_success "Created test topic with 4 partitions"
    else
        log_warning "Failed to create test topic (may already exist)"
    fi

    # Start producer in background
    {
        docker exec broker-1 bin/pulsar-perf produce \
            --service-url "${BROKER_URL}" \
            --topic "${TEST_TOPIC}" \
            --rate "${PRODUCE_RATE}" \
            --num-messages "${TEST_MESSAGES}" \
            --size "${MESSAGE_SIZE}" \
            --batch-time-period 100 \
            --max-pending 2000 \
            --producer-name "failover-test-producer" \
            --stats-interval-seconds 5 \
            2>&1
    } > "${result_file}" &

    local producer_pid=$!
    echo "${producer_pid}" > "${RESULTS_DIR}/producer_pid_${TIMESTAMP}.txt"

    # Start consumer in background
    {
        sleep 5
        docker exec broker-1 bin/pulsar-perf consume \
            --service-url "${BROKER_URL}" \
            --topic "${TEST_TOPIC}" \
            --subscription-name "${TEST_SUBSCRIPTION}" \
            --subscription-type Shared \
            --num-messages "${TEST_MESSAGES}" \
            --receiver-queue-size 1000 \
            --consumer-name "failover-test-consumer" \
            --stats-interval-seconds 5 \
            2>&1
    } > "${result_file}.consumer" &

    local consumer_pid=$!
    echo "${consumer_pid}" > "${RESULTS_DIR}/consumer_pid_${TIMESTAMP}.txt"

    log_success "Started continuous load (Producer PID: ${producer_pid}, Consumer PID: ${consumer_pid})"
    return 0
}

# Function to monitor metrics during failover
monitor_failover_metrics() {
    local duration=$1
    local metrics_file=$2

    log "Monitoring cluster metrics for ${duration} seconds..."

    local start_time=$(date +%s)
    local end_time=$((start_time + duration))

    echo "{" > "${metrics_file}"
    echo '  "test_type": "broker_failover",' >> "${metrics_file}"
    echo '  "start_time": "'$(date -d @${start_time})'",' >> "${metrics_file}"
    echo '  "duration_seconds": '${duration}',' >> "${metrics_file}"
    echo '  "metrics": [' >> "${metrics_file}"

    local first=true
    while [ $(date +%s) -lt $end_time ]; do
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))

        # Get cluster status
        local healthy_brokers=$(check_cluster_status)

        # Get topic stats
        local msg_rate_in=0
        local msg_rate_out=0
        local backlog=0

        if [ "$healthy_brokers" -gt 0 ]; then
            # Try to get topic stats from available broker
            for port in 8080 8081 8082; do
                if curl -s -f "http://localhost:${port}/admin/v2/clusters" > /dev/null 2>&1; then
                    local topic_stats=$(curl -s "http://localhost:${port}/admin/v2/persistent/public/default/failover-test/stats" 2>/dev/null || echo "{}")
                    msg_rate_in=$(echo "$topic_stats" | jq -r '.msgRateIn // 0' 2>/dev/null || echo "0")
                    msg_rate_out=$(echo "$topic_stats" | jq -r '.msgRateOut // 0' 2>/dev/null || echo "0")
                    backlog=$(echo "$topic_stats" | jq -r '.subscriptions["'${TEST_SUBSCRIPTION}'"].msgBacklog // 0' 2>/dev/null || echo "0")
                    break
                fi
            done
        fi

        # Add comma for JSON array
        if [ "$first" = false ]; then
            echo ',' >> "${metrics_file}"
        fi
        first=false

        # Write metrics entry
        cat >> "${metrics_file}" << EOF
    {
      "timestamp": "${current_time}",
      "elapsed_seconds": ${elapsed},
      "healthy_brokers": ${healthy_brokers},
      "msg_rate_in": ${msg_rate_in},
      "msg_rate_out": ${msg_rate_out},
      "consumer_backlog": ${backlog}
    }EOF

        sleep 5
    done

    echo >> "${metrics_file}"
    echo '  ]' >> "${metrics_file}"
    echo '}' >> "${metrics_file}"

    log_success "Metrics collection completed: ${metrics_file}"
}

# Function to run broker failover test
run_broker_failover_test() {
    local target_broker=${1:-"broker-2"}  # Default to broker-2
    local failure_method=${2:-"container"}

    log "Running broker failover test for: ${target_broker}"

    local test_log="${RESULTS_DIR}/broker_failover_${target_broker}_${TIMESTAMP}.log"
    local metrics_file="${RESULTS_DIR}/failover_metrics_${target_broker}_${TIMESTAMP}.json"

    # Start continuous load
    start_continuous_load "${test_log}"

    # Wait for load to stabilize
    log "Waiting ${FAILOVER_DELAY} seconds for load to stabilize..."
    sleep "${FAILOVER_DELAY}"

    # Start metrics monitoring in background
    monitor_failover_metrics 180 "${metrics_file}" &
    local monitor_pid=$!

    # Record pre-failure state
    local pre_failure_brokers=$(check_cluster_status)
    log "Pre-failure broker count: ${pre_failure_brokers}"

    # Simulate broker failure
    local failure_time=$(date +%s)
    simulate_broker_failure "${target_broker}" "${failure_method}"

    # Monitor during failure
    log "Monitoring cluster behavior during failure..."
    sleep 60

    # Record during-failure state
    local during_failure_brokers=$(check_cluster_status)
    log "During-failure broker count: ${during_failure_brokers}"

    # Recover broker
    local recovery_time=$(date +%s)
    recover_broker "${target_broker}" "${failure_method}"

    # Monitor recovery
    log "Monitoring cluster recovery..."
    sleep 60

    # Record post-recovery state
    local post_recovery_brokers=$(check_cluster_status)
    log "Post-recovery broker count: ${post_recovery_brokers}"

    # Stop monitoring
    kill "${monitor_pid}" 2>/dev/null || true
    wait "${monitor_pid}" 2>/dev/null || true

    # Stop load generators
    local producer_pid=$(cat "${RESULTS_DIR}/producer_pid_${TIMESTAMP}.txt" 2>/dev/null || echo "")
    local consumer_pid=$(cat "${RESULTS_DIR}/consumer_pid_${TIMESTAMP}.txt" 2>/dev/null || echo "")

    if [ -n "$producer_pid" ]; then
        kill "${producer_pid}" 2>/dev/null || true
    fi
    if [ -n "$consumer_pid" ]; then
        kill "${consumer_pid}" 2>/dev/null || true
    fi

    # Generate test summary
    local summary_file="${RESULTS_DIR}/failover_summary_${target_broker}_${TIMESTAMP}.json"
    cat > "${summary_file}" << EOF
{
  "test_type": "broker_failover",
  "target_broker": "${target_broker}",
  "failure_method": "${failure_method}",
  "timestamp": "${TIMESTAMP}",
  "failure_time": ${failure_time},
  "recovery_time": ${recovery_time},
  "downtime_seconds": $((recovery_time - failure_time)),
  "broker_states": {
    "pre_failure": ${pre_failure_brokers},
    "during_failure": ${during_failure_brokers},
    "post_recovery": ${post_recovery_brokers}
  },
  "test_parameters": {
    "test_messages": ${TEST_MESSAGES},
    "message_size": ${MESSAGE_SIZE},
    "produce_rate": ${PRODUCE_RATE},
    "test_topic": "${TEST_TOPIC}"
  }
}
EOF

    log_success "Broker failover test completed for ${target_broker}"
    log "Test summary: ${summary_file}"
    log "Detailed metrics: ${metrics_file}"
}

# Function to analyze failover results
analyze_failover_results() {
    log "Analyzing failover test results..."

    local analysis_file="${RESULTS_DIR}/failover_analysis_${TIMESTAMP}.md"

    cat > "${analysis_file}" << EOF
# Broker Failover Test Analysis

**Test Date:** $(date)
**Test Duration:** ~3 minutes per broker
**Test Topic:** ${TEST_TOPIC}

## Test Overview

This test simulates broker failures to validate:
1. Automatic failover detection
2. Client reconnection behavior
3. Message delivery continuity
4. Recovery time characteristics

## Test Configuration

- **Test Messages:** ${TEST_MESSAGES}
- **Message Size:** ${MESSAGE_SIZE} bytes
- **Produce Rate:** ${PRODUCE_RATE} msg/sec
- **Topic Partitions:** 4

## Results Summary

EOF

    # Analyze each test result
    for summary_file in "${RESULTS_DIR}"/failover_summary_*_${TIMESTAMP}.json; do
        if [ -f "$summary_file" ]; then
            local broker_name=$(jq -r '.target_broker' "$summary_file")
            local downtime=$(jq -r '.downtime_seconds' "$summary_file")
            local pre_failure=$(jq -r '.broker_states.pre_failure' "$summary_file")
            local during_failure=$(jq -r '.broker_states.during_failure' "$summary_file")
            local post_recovery=$(jq -r '.broker_states.post_recovery' "$summary_file")

            cat >> "${analysis_file}" << EOF
### ${broker_name^} Failover

- **Downtime:** ${downtime} seconds
- **Broker States:**
  - Pre-failure: ${pre_failure} brokers
  - During failure: ${during_failure} brokers
  - Post-recovery: ${post_recovery} brokers

EOF
        fi
    done

    cat >> "${analysis_file}" << EOF
## Key Metrics Analysis

### Failover Detection Time
- Target: <30 seconds
- Measured: [Analysis would require detailed timestamp comparison]

### Message Loss
- Expected: Zero message loss with proper acknowledgment
- Measured: [Analysis would require message counting]

### Recovery Time
- Target: <60 seconds for full recovery
- Measured: [Based on broker health checks]

## Recommendations

### Operational
1. Monitor broker health continuously
2. Set up automated alerting for broker failures
3. Test failover procedures regularly
4. Document recovery procedures

### Configuration
1. Configure appropriate client timeout settings
2. Use appropriate producer retry policies
3. Set consumer acknowledgment timeouts properly
4. Configure topic replication factors appropriately

### Monitoring
1. Track broker health metrics
2. Monitor client connection patterns
3. Alert on message backlog growth
4. Track publish/consume latency during failures

## Raw Data

Detailed metrics and logs available in:
- Metrics files: \`${RESULTS_DIR}/failover_metrics_*_${TIMESTAMP}.json\`
- Test logs: \`${RESULTS_DIR}/broker_failover_*_${TIMESTAMP}.log\`
- Summary files: \`${RESULTS_DIR}/failover_summary_*_${TIMESTAMP}.json\`

EOF

    log_success "Failover analysis report generated: ${analysis_file}"
}

# Function to cleanup test resources
cleanup_test_resources() {
    log "Cleaning up test resources..."

    # Delete test topic
    if curl -s -X DELETE "${ADMIN_URL}/admin/v2/persistent/public/default/failover-test" > /dev/null 2>&1; then
        log_success "Deleted test topic"
    else
        log_warning "Failed to delete test topic"
    fi

    # Clean up PID files
    rm -f "${RESULTS_DIR}"/producer_pid_*.txt "${RESULTS_DIR}"/consumer_pid_*.txt 2>/dev/null || true
}

# Main execution
main() {
    log "Starting Pulsar Broker Failover Test"

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
    local initial_brokers=$(check_cluster_status)
    if [ "$initial_brokers" -lt 2 ]; then
        log_error "Need at least 2 brokers for failover testing (found: ${initial_brokers})"
        exit 1
    fi

    log_success "Initial cluster state: ${initial_brokers} brokers available"

    # Test different brokers
    local brokers_to_test=("broker-2")
    if [ "$initial_brokers" -ge 3 ]; then
        brokers_to_test+=("broker-3")
    fi

    for broker in "${brokers_to_test[@]}"; do
        log "Testing failover for: ${broker}"
        run_broker_failover_test "${broker}" "container"
        sleep 30  # Recovery time between tests
    done

    # Analyze results
    analyze_failover_results

    # Cleanup
    cleanup_test_resources

    log_success "Broker failover tests completed!"
    log "Results available in: ${RESULTS_DIR}/"
    log "Analysis report: ${RESULTS_DIR}/failover_analysis_${TIMESTAMP}.md"
}

# Handle script arguments
case "${1:-}" in
    --help|-h)
        echo "Usage: $0 [options] [broker-name]"
        echo ""
        echo "Tests broker failover scenarios by stopping/starting broker containers."
        echo ""
        echo "Options:"
        echo "  --help, -h              Show this help message"
        echo ""
        echo "Arguments:"
        echo "  broker-name             Specific broker to test (default: test all available)"
        echo ""
        echo "Environment Variables:"
        echo "  ADMIN_URL               Pulsar admin REST URL (default: http://localhost:8080)"
        echo "  BROKER_URL              Pulsar broker service URL (default: pulsar://localhost:6650)"
        echo "  TEST_TOPIC              Topic to use for tests"
        echo ""
        echo "Requirements:"
        echo "  - Docker with running Pulsar cluster"
        echo "  - At least 2 brokers for meaningful failover testing"
        echo "  - jq for JSON processing"
        echo ""
        exit 0
        ;;
    *)
        if [ -n "$1" ]; then
            # Test specific broker
            mkdir -p "${RESULTS_DIR}"
            run_broker_failover_test "$1" "container"
        else
            # Test all brokers
            main
        fi
        ;;
esac
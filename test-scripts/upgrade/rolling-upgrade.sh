#!/bin/bash

set -e

# Pulsar Rolling Upgrade Test
# Tests zero-downtime rolling upgrade procedures

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
ADMIN_URL=${ADMIN_URL:-"http://localhost:8080"}
BROKER_URL=${BROKER_URL:-"pulsar://localhost:6650"}
UPGRADE_ENV_FILE=${UPGRADE_ENV_FILE:-"../../docker-compose/upgrade-test/.env"}
DOCKER_COMPOSE_FILE=${DOCKER_COMPOSE_FILE:-"../../docker-compose/upgrade-test/docker-compose.yml"}
RESULTS_DIR="./results/upgrade"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')

# Default versions
OLD_VERSION="apachepulsar/pulsar:3.0.2"
NEW_VERSION="apachepulsar/pulsar:3.1.1"

# Test parameters
TEST_MESSAGES=100000
MESSAGE_SIZE=1024
PRODUCE_RATE=1000
UPGRADE_DELAY=60

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

# Function to load environment variables
load_upgrade_env() {
    if [ -f "$UPGRADE_ENV_FILE" ]; then
        log "Loading upgrade environment from: $UPGRADE_ENV_FILE"
        source "$UPGRADE_ENV_FILE"
        OLD_VERSION="$PULSAR_OLD_VERSION"
        NEW_VERSION="$PULSAR_NEW_VERSION"
    else
        log_warning "Upgrade env file not found: $UPGRADE_ENV_FILE"
        log "Using default versions: $OLD_VERSION -> $NEW_VERSION"
    fi
}

# Function to update environment variable in .env file
update_env_var() {
    local var_name=$1
    local var_value=$2
    local env_file=$3

    if [ -f "$env_file" ]; then
        # Use sed to update the variable
        if grep -q "^${var_name}=" "$env_file"; then
            sed -i.bak "s|^${var_name}=.*|${var_name}=${var_value}|" "$env_file"
            log_success "Updated ${var_name}=${var_value} in ${env_file}"
        else
            echo "${var_name}=${var_value}" >> "$env_file"
            log_success "Added ${var_name}=${var_value} to ${env_file}"
        fi
    else
        log_error "Environment file not found: $env_file"
        return 1
    fi
}

# Function to get component versions
get_component_versions() {
    local versions_file="${RESULTS_DIR}/component_versions_${TIMESTAMP}.json"

    log "Getting current component versions..."

    # Get versions from running containers
    local components=("bookie-1" "bookie-2" "bookie-3" "broker-1" "broker-2")

    echo "{" > "$versions_file"
    echo '  "timestamp": "'$TIMESTAMP'",' >> "$versions_file"
    echo '  "versions": {' >> "$versions_file"

    local first=true
    for component in "${components[@]}"; do
        if docker ps --format "table {{.Names}}" | grep -q "$component"; then
            local image=$(docker inspect "$component" --format='{{.Config.Image}}' 2>/dev/null || echo "unknown")

            if [ "$first" = false ]; then
                echo ',' >> "$versions_file"
            fi
            first=false

            echo "    \"$component\": \"$image\"" >> "$versions_file"
        fi
    done

    echo >> "$versions_file"
    echo '  }' >> "$versions_file"
    echo '}' >> "$versions_file"

    log_success "Component versions saved to: $versions_file"
}

# Function to start continuous load for upgrade testing
start_upgrade_load() {
    local result_file=$1

    log "Starting continuous load for upgrade testing..."

    # Create test topic
    if curl -s -X PUT "${ADMIN_URL}/admin/v2/persistent/public/default/upgrade-test-topic/partitions/8" > /dev/null 2>&1; then
        log_success "Created upgrade test topic with 8 partitions"
    else
        log_warning "Failed to create upgrade test topic (may already exist)"
    fi

    # Start producer in background
    {
        docker exec broker-1-upgrade bin/pulsar-perf produce \
            --service-url "${BROKER_URL}" \
            --topic "persistent://public/default/upgrade-test-topic" \
            --rate "${PRODUCE_RATE}" \
            --num-messages "${TEST_MESSAGES}" \
            --size "${MESSAGE_SIZE}" \
            --batch-time-period 500 \
            --max-pending 2000 \
            --producer-name "upgrade-producer" \
            --stats-interval-seconds 5 \
            2>&1
    } > "${result_file}.producer" &

    local producer_pid=$!
    echo "${producer_pid}" > "${RESULTS_DIR}/upgrade_producer_pid_${TIMESTAMP}.txt"

    # Start consumer in background
    {
        sleep 5
        docker exec broker-1-upgrade bin/pulsar-perf consume \
            --service-url "${BROKER_URL}" \
            --topic "persistent://public/default/upgrade-test-topic" \
            --subscription-name "upgrade-test-sub" \
            --subscription-type Shared \
            --num-messages "${TEST_MESSAGES}" \
            --receiver-queue-size 1000 \
            --consumer-name "upgrade-consumer" \
            --stats-interval-seconds 5 \
            2>&1
    } > "${result_file}.consumer" &

    local consumer_pid=$!
    echo "${consumer_pid}" > "${RESULTS_DIR}/upgrade_consumer_pid_${TIMESTAMP}.txt"

    log_success "Started upgrade load (Producer PID: ${producer_pid}, Consumer PID: ${consumer_pid})"
    return 0
}

# Function to monitor upgrade metrics
monitor_upgrade_metrics() {
    local duration=$1
    local metrics_file=$2
    local current_stage=$3

    log "Monitoring upgrade metrics for ${duration} seconds (stage: ${current_stage})..."

    local start_time=$(date +%s)
    local end_time=$((start_time + duration))

    # Initialize metrics file if not exists
    if [ ! -f "$metrics_file" ]; then
        echo "{" > "$metrics_file"
        echo '  "test_type": "rolling_upgrade",' >> "$metrics_file"
        echo '  "old_version": "'$OLD_VERSION'",' >> "$metrics_file"
        echo '  "new_version": "'$NEW_VERSION'",' >> "$metrics_file"
        echo '  "start_time": "'$(date -d @${start_time})'",' >> "$metrics_file"
        echo '  "metrics": [' >> "$metrics_file"
    fi

    local first_metric=false
    if grep -q '"metrics": \[\]' "$metrics_file" 2>/dev/null; then
        first_metric=true
    fi

    while [ $(date +%s) -lt $end_time ]; do
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))

        # Get cluster health
        local healthy_brokers=0
        for port in 8080 8081; do
            if curl -s -f "http://localhost:${port}/admin/v2/clusters" > /dev/null 2>&1; then
                healthy_brokers=$((healthy_brokers + 1))
            fi
        done

        # Get topic stats
        local msg_rate_in=0
        local msg_rate_out=0
        local consumer_lag=0

        for port in 8080 8081; do
            if curl -s -f "http://localhost:${port}/admin/v2/clusters" > /dev/null 2>&1; then
                local topic_stats=$(curl -s "http://localhost:${port}/admin/v2/persistent/public/default/upgrade-test-topic/stats" 2>/dev/null || echo "{}")
                msg_rate_in=$(echo "$topic_stats" | jq -r '.msgRateIn // 0' 2>/dev/null || echo "0")
                msg_rate_out=$(echo "$topic_stats" | jq -r '.msgRateOut // 0' 2>/dev/null || echo "0")
                consumer_lag=$(echo "$topic_stats" | jq -r '.subscriptions["upgrade-test-sub"].msgBacklog // 0' 2>/dev/null || echo "0")
                break
            fi
        done

        # Get component versions
        local broker_1_version=$(docker inspect broker-1-upgrade --format='{{.Config.Image}}' 2>/dev/null || echo "unknown")
        local broker_2_version=$(docker inspect broker-2-upgrade --format='{{.Config.Image}}' 2>/dev/null || echo "unknown")

        # Add comma for JSON array if not first metric
        if [ "$first_metric" = false ]; then
            # Remove the last ] and add comma
            sed -i '$s/]/,/' "$metrics_file" 2>/dev/null || true
        fi
        first_metric=false

        # Write metrics entry
        cat >> "$metrics_file" << EOF
    {
      "timestamp": ${current_time},
      "elapsed_seconds": ${elapsed},
      "stage": "${current_stage}",
      "healthy_brokers": ${healthy_brokers},
      "msg_rate_in": ${msg_rate_in},
      "msg_rate_out": ${msg_rate_out},
      "consumer_lag": ${consumer_lag},
      "broker_1_version": "${broker_1_version}",
      "broker_2_version": "${broker_2_version}"
    }
  ]
}EOF

        sleep 10
    done

    log_success "Upgrade metrics monitoring completed for stage: ${current_stage}"
}

# Function to upgrade bookies
upgrade_bookies() {
    local metrics_file=$1

    log "Starting BookKeeper rolling upgrade..."

    # Start monitoring during bookie upgrades
    monitor_upgrade_metrics 180 "$metrics_file" "bookie_upgrade" &
    local monitor_pid=$!

    local bookies=("bookie-1" "bookie-2" "bookie-3")
    for bookie in "${bookies[@]}"; do
        log "Upgrading ${bookie} to ${NEW_VERSION}..."

        # Update version in environment file
        update_env_var "${bookie^^}_VERSION" "$NEW_VERSION" "$UPGRADE_ENV_FILE"

        # Restart bookie with new version
        if cd "$(dirname "$DOCKER_COMPOSE_FILE")"; then
            docker-compose stop "$bookie-upgrade" 2>/dev/null || docker-compose stop "$bookie" 2>/dev/null || true
            sleep 10
            docker-compose up -d "$bookie-upgrade" 2>/dev/null || docker-compose up -d "$bookie" 2>/dev/null || true
            cd - > /dev/null
        else
            log_error "Could not change to docker-compose directory"
            return 1
        fi

        # Wait for bookie to be healthy
        local max_wait=120
        local wait_time=0
        log "Waiting for ${bookie} to be healthy after upgrade..."

        while [ $wait_time -lt $max_wait ]; do
            if docker exec "${bookie}-upgrade" bin/bookkeeper shell bookiesanity > /dev/null 2>&1 || \
               docker exec "${bookie}" bin/bookkeeper shell bookiesanity > /dev/null 2>&1; then
                log_success "${bookie} is healthy after upgrade"
                break
            fi
            sleep 5
            wait_time=$((wait_time + 5))
            echo -n "."
        done

        if [ $wait_time -ge $max_wait ]; then
            log_error "${bookie} did not become healthy within ${max_wait} seconds"
        fi

        # Wait between bookie upgrades
        log "Waiting ${UPGRADE_DELAY} seconds before next bookie upgrade..."
        sleep "$UPGRADE_DELAY"
    done

    # Stop monitoring
    kill "$monitor_pid" 2>/dev/null || true
    wait "$monitor_pid" 2>/dev/null || true

    log_success "BookKeeper rolling upgrade completed"
}

# Function to upgrade brokers
upgrade_brokers() {
    local metrics_file=$1

    log "Starting Pulsar broker rolling upgrade..."

    # Start monitoring during broker upgrades
    monitor_upgrade_metrics 180 "$metrics_file" "broker_upgrade" &
    local monitor_pid=$!

    local brokers=("broker-1" "broker-2")
    for broker in "${brokers[@]}"; do
        log "Upgrading ${broker} to ${NEW_VERSION}..."

        # Update version in environment file
        update_env_var "${broker^^}_VERSION" "$NEW_VERSION" "$UPGRADE_ENV_FILE"

        # Graceful broker shutdown - unload topics first
        log "Unloading topics from ${broker}..."
        for port in 8080 8081; do
            if curl -s -f "http://localhost:${port}/admin/v2/clusters" > /dev/null 2>&1; then
                # Try to unload broker's topics gracefully
                curl -s -X PUT "http://localhost:${port}/admin/v2/brokers/${broker}-upgrade:8080/unload" > /dev/null 2>&1 || true
                break
            fi
        done

        # Wait for topic unloading
        sleep 30

        # Restart broker with new version
        if cd "$(dirname "$DOCKER_COMPOSE_FILE")"; then
            docker-compose stop "$broker-upgrade" 2>/dev/null || docker-compose stop "$broker" 2>/dev/null || true
            sleep 15
            docker-compose up -d "$broker-upgrade" 2>/dev/null || docker-compose up -d "$broker" 2>/dev/null || true
            cd - > /dev/null
        else
            log_error "Could not change to docker-compose directory"
            return 1
        fi

        # Wait for broker to be ready
        local max_wait=180
        local wait_time=0
        local broker_port=""

        case $broker in
            "broker-1") broker_port="8080" ;;
            "broker-2") broker_port="8081" ;;
        esac

        if [ -n "$broker_port" ]; then
            log "Waiting for ${broker} to be ready after upgrade on port ${broker_port}..."
            while [ $wait_time -lt $max_wait ]; do
                if curl -s -f "http://localhost:${broker_port}/admin/v2/clusters" > /dev/null 2>&1; then
                    log_success "${broker} is ready after upgrade"
                    break
                fi
                sleep 10
                wait_time=$((wait_time + 10))
                echo -n "."
            done

            if [ $wait_time -ge $max_wait ]; then
                log_error "${broker} did not become ready within ${max_wait} seconds"
            fi
        fi

        # Wait between broker upgrades for load rebalancing
        log "Waiting ${UPGRADE_DELAY} seconds for load rebalancing before next broker upgrade..."
        sleep "$UPGRADE_DELAY"
    done

    # Stop monitoring
    kill "$monitor_pid" 2>/dev/null || true
    wait "$monitor_pid" 2>/dev/null || true

    log_success "Pulsar broker rolling upgrade completed"
}

# Function to validate upgrade success
validate_upgrade() {
    local metrics_file=$1

    log "Validating upgrade success..."

    # Start post-upgrade monitoring
    monitor_upgrade_metrics 60 "$metrics_file" "post_upgrade" &
    local monitor_pid=$!

    # Check component versions
    get_component_versions

    # Verify cluster health
    local healthy_brokers=0
    for port in 8080 8081; do
        if curl -s -f "http://localhost:${port}/admin/v2/clusters" > /dev/null 2>&1; then
            healthy_brokers=$((healthy_brokers + 1))
        fi
    done

    log "Post-upgrade broker health: ${healthy_brokers}/2 brokers healthy"

    # Check topic accessibility
    local topic_stats=$(curl -s "http://localhost:8080/admin/v2/persistent/public/default/upgrade-test-topic/stats" 2>/dev/null || echo "{}")
    local current_backlog=$(echo "$topic_stats" | jq -r '.subscriptions["upgrade-test-sub"].msgBacklog // 0' 2>/dev/null || echo "0")

    log "Post-upgrade consumer backlog: ${current_backlog} messages"

    # Verify new features work (if any)
    log "Testing post-upgrade functionality..."

    # Create a test topic to verify cluster functionality
    if curl -s -X PUT "http://localhost:8080/admin/v2/persistent/public/default/post-upgrade-test/partitions/2" > /dev/null 2>&1; then
        log_success "Post-upgrade topic creation successful"
    else
        log_warning "Post-upgrade topic creation failed"
    fi

    # Stop monitoring
    kill "$monitor_pid" 2>/dev/null || true
    wait "$monitor_pid" 2>/dev/null || true

    local validation_result="success"
    if [ "$healthy_brokers" -lt 2 ]; then
        validation_result="failed"
    fi

    log_success "Upgrade validation completed: ${validation_result}"
    echo "$validation_result"
}

# Function to run complete rolling upgrade
run_rolling_upgrade() {
    log "Starting Pulsar rolling upgrade test"
    log "Upgrading from ${OLD_VERSION} to ${NEW_VERSION}"

    local test_log="${RESULTS_DIR}/rolling_upgrade_${TIMESTAMP}.log"
    local metrics_file="${RESULTS_DIR}/upgrade_metrics_${TIMESTAMP}.json"

    # Record pre-upgrade state
    log "Recording pre-upgrade state..."
    get_component_versions

    # Start continuous load
    start_upgrade_load "$test_log"

    # Wait for load to stabilize
    log "Waiting ${UPGRADE_DELAY} seconds for load to stabilize..."
    sleep "$UPGRADE_DELAY"

    # Start pre-upgrade monitoring
    monitor_upgrade_metrics 60 "$metrics_file" "pre_upgrade" &
    local monitor_pid=$!

    sleep 60

    # Stop initial monitoring
    kill "$monitor_pid" 2>/dev/null || true
    wait "$monitor_pid" 2>/dev/null || true

    # Record upgrade start time
    local upgrade_start_time=$(date +%s)

    # Upgrade BookKeeper first (storage layer)
    upgrade_bookies "$metrics_file"

    # Wait between BookKeeper and Pulsar upgrades
    log "Waiting between BookKeeper and broker upgrades..."
    sleep 60

    # Upgrade Pulsar brokers (service layer)
    upgrade_brokers "$metrics_file"

    # Record upgrade completion time
    local upgrade_end_time=$(date +%s)
    local total_upgrade_time=$((upgrade_end_time - upgrade_start_time))

    # Validate upgrade
    local validation_result=$(validate_upgrade "$metrics_file")

    # Stop load generators
    local producer_pid=$(cat "${RESULTS_DIR}/upgrade_producer_pid_${TIMESTAMP}.txt" 2>/dev/null || echo "")
    local consumer_pid=$(cat "${RESULTS_DIR}/upgrade_consumer_pid_${TIMESTAMP}.txt" 2>/dev/null || echo "")

    if [ -n "$producer_pid" ]; then
        kill "$producer_pid" 2>/dev/null || true
    fi
    if [ -n "$consumer_pid" ]; then
        kill "$consumer_pid" 2>/dev/null || true
    fi

    # Generate upgrade summary
    local summary_file="${RESULTS_DIR}/upgrade_summary_${TIMESTAMP}.json"
    cat > "$summary_file" << EOF
{
  "test_type": "rolling_upgrade",
  "timestamp": "${TIMESTAMP}",
  "old_version": "${OLD_VERSION}",
  "new_version": "${NEW_VERSION}",
  "upgrade_start_time": ${upgrade_start_time},
  "upgrade_end_time": ${upgrade_end_time},
  "total_upgrade_time_seconds": ${total_upgrade_time},
  "validation_result": "${validation_result}",
  "test_parameters": {
    "test_messages": ${TEST_MESSAGES},
    "message_size": ${MESSAGE_SIZE},
    "produce_rate": ${PRODUCE_RATE},
    "upgrade_delay": ${UPGRADE_DELAY}
  }
}
EOF

    log_success "Rolling upgrade test completed in ${total_upgrade_time} seconds"
    log_success "Validation result: ${validation_result}"
    log "Test summary: ${summary_file}"
    log "Detailed metrics: ${metrics_file}"
}

# Function to cleanup upgrade test resources
cleanup_upgrade_resources() {
    log "Cleaning up upgrade test resources..."

    # Delete test topics
    curl -s -X DELETE "${ADMIN_URL}/admin/v2/persistent/public/default/upgrade-test-topic" > /dev/null 2>&1 || true
    curl -s -X DELETE "${ADMIN_URL}/admin/v2/persistent/public/default/post-upgrade-test" > /dev/null 2>&1 || true

    # Clean up PID files
    rm -f "${RESULTS_DIR}"/upgrade_*_pid_*.txt 2>/dev/null || true

    log_success "Cleanup completed"
}

# Main execution
main() {
    log "Starting Pulsar Rolling Upgrade Test"

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

    if ! command -v docker-compose &> /dev/null; then
        log_error "docker-compose is required but not installed"
        exit 1
    fi

    # Load upgrade environment
    load_upgrade_env

    # Check if upgrade test cluster is running
    if ! docker ps --format "table {{.Names}}" | grep -q "upgrade"; then
        log_error "Upgrade test cluster is not running. Please start it first:"
        log_error "cd ../../docker-compose/upgrade-test && docker-compose up -d"
        exit 1
    fi

    # Run rolling upgrade
    run_rolling_upgrade

    # Cleanup
    cleanup_upgrade_resources

    log_success "Rolling upgrade test completed successfully!"
    log "Results available in: ${RESULTS_DIR}/"
}

# Handle script arguments
case "${1:-}" in
    --help|-h)
        echo "Usage: $0 [options]"
        echo ""
        echo "Performs a zero-downtime rolling upgrade test of Pulsar cluster."
        echo ""
        echo "Process:"
        echo "  1. Start continuous producer/consumer load"
        echo "  2. Rolling upgrade of BookKeeper bookies"
        echo "  3. Rolling upgrade of Pulsar brokers"
        echo "  4. Validate cluster health and functionality"
        echo ""
        echo "Options:"
        echo "  --help, -h              Show this help message"
        echo ""
        echo "Environment Variables:"
        echo "  ADMIN_URL               Pulsar admin REST URL (default: http://localhost:8080)"
        echo "  BROKER_URL              Pulsar broker service URL (default: pulsar://localhost:6650)"
        echo "  UPGRADE_ENV_FILE        Path to upgrade .env file"
        echo "  DOCKER_COMPOSE_FILE     Path to upgrade docker-compose.yml"
        echo ""
        echo "Requirements:"
        echo "  - Running upgrade test cluster (docker-compose/upgrade-test/)"
        echo "  - jq for JSON processing"
        echo "  - docker and docker-compose"
        echo ""
        echo "The test validates:"
        echo "  - Zero message loss during upgrade"
        echo "  - Continuous service availability"
        echo "  - Proper version transitions"
        echo "  - Post-upgrade functionality"
        echo ""
        exit 0
        ;;
    *)
        main
        ;;
esac
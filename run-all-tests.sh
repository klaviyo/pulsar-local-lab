#!/bin/bash

set -e

# Pulsar Local Lab Test Runner
# Orchestrates all test scenarios for comprehensive Pulsar cluster testing

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="${SCRIPT_DIR}/test-results"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
LOG_FILE="${RESULTS_DIR}/test_run_${TIMESTAMP}.log"

# Test categories
declare -A TEST_CATEGORIES=(
    ["docker"]="Docker Compose basic and HA cluster tests"
    ["ha"]="High Availability cluster tests only"
    ["k8s"]="Kubernetes cluster tests"
    ["performance"]="Performance and scaling tests"
    ["failover"]="Failover and chaos engineering tests"
    ["upgrade"]="Rolling upgrade tests"
    ["all"]="All available tests"
)

# Test execution tracking
declare -A TEST_RESULTS=()
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0

log() {
    local message="$1"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo -e "${BLUE}[${timestamp}]${NC} $message" | tee -a "$LOG_FILE"
}

log_success() {
    local message="$1"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo -e "${GREEN}[${timestamp}] ✓${NC} $message" | tee -a "$LOG_FILE"
}

log_warning() {
    local message="$1"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo -e "${YELLOW}[${timestamp}] ⚠${NC} $message" | tee -a "$LOG_FILE"
}

log_error() {
    local message="$1"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo -e "${RED}[${timestamp}] ✗${NC} $message" | tee -a "$LOG_FILE"
}

log_section() {
    local title="$1"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "" | tee -a "$LOG_FILE"
    echo -e "${PURPLE}╔══════════════════════════════════════════════════════════════╗${NC}" | tee -a "$LOG_FILE"
    echo -e "${PURPLE}║${NC} ${CYAN}$title${NC}" | tee -a "$LOG_FILE"
    echo -e "${PURPLE}╚══════════════════════════════════════════════════════════════╝${NC}" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"
}

# Function to show available test categories
show_test_categories() {
    echo -e "${CYAN}Available test categories:${NC}"
    echo ""
    for category in "${!TEST_CATEGORIES[@]}"; do
        echo -e "  ${YELLOW}$category${NC} - ${TEST_CATEGORIES[$category]}"
    done
    echo ""
}

# Function to check prerequisites
check_prerequisites() {
    log_section "Checking Prerequisites"

    local missing_deps=()

    # Check required commands
    local required_commands=("docker" "docker-compose" "jq" "curl")
    for cmd in "${required_commands[@]}"; do
        if ! command -v "$cmd" &> /dev/null; then
            missing_deps+=("$cmd")
        else
            log_success "$cmd is available"
        fi
    done

    # Check optional commands
    local optional_commands=("kubectl" "helm" "kind")
    for cmd in "${optional_commands[@]}"; do
        if command -v "$cmd" &> /dev/null; then
            log_success "$cmd is available (optional)"
        else
            log_warning "$cmd not found (optional, needed for Kubernetes tests)"
        fi
    done

    if [ ${#missing_deps[@]} -gt 0 ]; then
        log_error "Missing required dependencies: ${missing_deps[*]}"
        log_error "Please install missing dependencies and try again."
        exit 1
    fi

    log_success "All required prerequisites are met"
}

# Function to execute a test with error handling
execute_test() {
    local test_name="$1"
    local test_command="$2"
    local category="$3"

    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    log "Executing test: $test_name"
    log "Command: $test_command"

    local test_start_time=$(date +%s)
    local test_result_file="${RESULTS_DIR}/${category}_${test_name}_${TIMESTAMP}.result"

    # Execute the test
    if eval "$test_command" > "$test_result_file" 2>&1; then
        local test_end_time=$(date +%s)
        local test_duration=$((test_end_time - test_start_time))

        log_success "Test '$test_name' PASSED (${test_duration}s)"
        TEST_RESULTS["$test_name"]="PASSED"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        local test_end_time=$(date +%s)
        local test_duration=$((test_end_time - test_start_time))

        log_error "Test '$test_name' FAILED (${test_duration}s)"
        log_error "Check logs in: $test_result_file"
        TEST_RESULTS["$test_name"]="FAILED"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
}

# Function to skip a test
skip_test() {
    local test_name="$1"
    local reason="$2"

    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    SKIPPED_TESTS=$((SKIPPED_TESTS + 1))

    log_warning "Skipping test '$test_name': $reason"
    TEST_RESULTS["$test_name"]="SKIPPED"
}

# Function to run Docker Compose basic cluster tests
run_docker_basic_tests() {
    log_section "Docker Compose Basic Cluster Tests"

    # Start basic cluster
    execute_test "basic-cluster-startup" \
        "cd ${SCRIPT_DIR}/docker-compose/basic-cluster && ./start-cluster.sh" \
        "docker"

    if [ "${TEST_RESULTS["basic-cluster-startup"]}" = "PASSED" ]; then
        # Run performance baseline
        execute_test "basic-performance-baseline" \
            "cd ${SCRIPT_DIR}/test-scripts/performance && ./baseline-test.sh" \
            "docker"

        # Run scaling test
        execute_test "basic-scaling-test" \
            "cd ${SCRIPT_DIR}/test-scripts/performance && ./scaling-test.sh" \
            "docker"

        # Stop basic cluster
        execute_test "basic-cluster-shutdown" \
            "cd ${SCRIPT_DIR}/docker-compose/basic-cluster && docker-compose down" \
            "docker"
    else
        skip_test "basic-performance-baseline" "Basic cluster failed to start"
        skip_test "basic-scaling-test" "Basic cluster failed to start"
        skip_test "basic-cluster-shutdown" "Basic cluster failed to start"
    fi
}

# Function to run HA cluster tests
run_ha_cluster_tests() {
    log_section "High Availability Cluster Tests"

    # Start HA cluster
    execute_test "ha-cluster-startup" \
        "cd ${SCRIPT_DIR}/docker-compose/ha-cluster && docker-compose up -d && sleep 120" \
        "ha"

    if [ "${TEST_RESULTS["ha-cluster-startup"]}" = "PASSED" ]; then
        # Run broker failover tests
        execute_test "ha-broker-failover" \
            "cd ${SCRIPT_DIR}/test-scripts/failover && ./broker-failover.sh" \
            "ha"

        # Run bookie failover tests
        execute_test "ha-bookie-failover" \
            "cd ${SCRIPT_DIR}/test-scripts/failover && ./bookie-failover.sh" \
            "ha"

        # Run performance test on HA cluster
        execute_test "ha-performance-test" \
            "cd ${SCRIPT_DIR}/test-scripts/performance && ADMIN_URL=http://localhost:8083 BROKER_URL=pulsar://localhost:6653 ./baseline-test.sh" \
            "ha"

        # Stop HA cluster
        execute_test "ha-cluster-shutdown" \
            "cd ${SCRIPT_DIR}/docker-compose/ha-cluster && docker-compose down" \
            "ha"
    else
        skip_test "ha-broker-failover" "HA cluster failed to start"
        skip_test "ha-bookie-failover" "HA cluster failed to start"
        skip_test "ha-performance-test" "HA cluster failed to start"
        skip_test "ha-cluster-shutdown" "HA cluster failed to start"
    fi
}

# Function to run upgrade tests
run_upgrade_tests() {
    log_section "Rolling Upgrade Tests"

    # Start upgrade test cluster
    execute_test "upgrade-cluster-startup" \
        "cd ${SCRIPT_DIR}/docker-compose/upgrade-test && docker-compose up -d && sleep 90" \
        "upgrade"

    if [ "${TEST_RESULTS["upgrade-cluster-startup"]}" = "PASSED" ]; then
        # Run rolling upgrade test
        execute_test "rolling-upgrade" \
            "cd ${SCRIPT_DIR}/test-scripts/upgrade && ./rolling-upgrade.sh" \
            "upgrade"

        # Stop upgrade test cluster
        execute_test "upgrade-cluster-shutdown" \
            "cd ${SCRIPT_DIR}/docker-compose/upgrade-test && docker-compose down" \
            "upgrade"
    else
        skip_test "rolling-upgrade" "Upgrade cluster failed to start"
        skip_test "upgrade-cluster-shutdown" "Upgrade cluster failed to start"
    fi
}

# Function to run Kubernetes tests
run_kubernetes_tests() {
    log_section "Kubernetes Tests"

    # Check if kubectl is available
    if ! command -v kubectl &> /dev/null; then
        skip_test "k8s-cluster-setup" "kubectl not available"
        skip_test "k8s-pulsar-deployment" "kubectl not available"
        skip_test "k8s-performance-test" "kubectl not available"
        skip_test "k8s-cleanup" "kubectl not available"
        return
    fi

    # Check if kind is available
    if ! command -v kind &> /dev/null; then
        log_warning "kind not available, checking for existing Kubernetes cluster..."
        if ! kubectl cluster-info &> /dev/null; then
            skip_test "k8s-cluster-setup" "No Kubernetes cluster available"
            skip_test "k8s-pulsar-deployment" "No Kubernetes cluster available"
            skip_test "k8s-performance-test" "No Kubernetes cluster available"
            skip_test "k8s-cleanup" "No Kubernetes cluster available"
            return
        fi
    else
        # Setup kind cluster
        execute_test "k8s-cluster-setup" \
            "cd ${SCRIPT_DIR}/kubernetes && ./setup-k8s.sh" \
            "k8s"
    fi

    # The rest of K8s tests would be implemented when Kubernetes manifests are created
    skip_test "k8s-pulsar-deployment" "Kubernetes manifests not yet implemented"
    skip_test "k8s-performance-test" "Kubernetes manifests not yet implemented"
    skip_test "k8s-cleanup" "Kubernetes manifests not yet implemented"
}

# Function to run performance-only tests
run_performance_tests() {
    log_section "Performance Tests Only"

    # Check if any cluster is running
    local cluster_available=false
    if curl -s -f "http://localhost:8080/admin/v2/clusters" > /dev/null 2>&1; then
        cluster_available=true
        log_success "Found running Pulsar cluster on localhost:8080"
    elif curl -s -f "http://localhost:8083/admin/v2/clusters" > /dev/null 2>&1; then
        cluster_available=true
        log_success "Found running Pulsar cluster on localhost:8083"
    fi

    if [ "$cluster_available" = true ]; then
        # Run baseline performance test
        execute_test "standalone-performance-baseline" \
            "cd ${SCRIPT_DIR}/test-scripts/performance && ./baseline-test.sh" \
            "performance"

        # Run scaling test
        execute_test "standalone-scaling-test" \
            "cd ${SCRIPT_DIR}/test-scripts/performance && ./scaling-test.sh" \
            "performance"
    else
        skip_test "standalone-performance-baseline" "No running Pulsar cluster found"
        skip_test "standalone-scaling-test" "No running Pulsar cluster found"
    fi
}

# Function to run failover-only tests
run_failover_tests() {
    log_section "Failover Tests Only"

    # Check if HA cluster is running (need multiple brokers for meaningful failover tests)
    local ha_cluster_available=false
    local broker_count=0

    for port in 8080 8081 8082; do
        if curl -s -f "http://localhost:${port}/admin/v2/clusters" > /dev/null 2>&1; then
            broker_count=$((broker_count + 1))
        fi
    done

    if [ $broker_count -ge 2 ]; then
        ha_cluster_available=true
        log_success "Found HA cluster with ${broker_count} brokers"
    fi

    if [ "$ha_cluster_available" = true ]; then
        # Run broker failover test
        execute_test "standalone-broker-failover" \
            "cd ${SCRIPT_DIR}/test-scripts/failover && ./broker-failover.sh" \
            "failover"

        # Run bookie failover test
        execute_test "standalone-bookie-failover" \
            "cd ${SCRIPT_DIR}/test-scripts/failover && ./bookie-failover.sh" \
            "failover"
    else
        skip_test "standalone-broker-failover" "Need HA cluster (2+ brokers) for failover testing"
        skip_test "standalone-bookie-failover" "Need HA cluster (3+ bookies) for failover testing"
    fi
}

# Function to generate comprehensive test report
generate_test_report() {
    log_section "Test Results Summary"

    local report_file="${RESULTS_DIR}/test_report_${TIMESTAMP}.md"
    local json_report="${RESULTS_DIR}/test_report_${TIMESTAMP}.json"

    # Generate Markdown report
    cat > "$report_file" << EOF
# Pulsar Local Lab Test Report

**Test Run Date:** $(date)
**Total Runtime:** $(date -u -d @$(($(date +%s) - START_TIME)) +%H:%M:%S)

## Summary

- **Total Tests:** ${TOTAL_TESTS}
- **Passed:** ${PASSED_TESTS}
- **Failed:** ${FAILED_TESTS}
- **Skipped:** ${SKIPPED_TESTS}
- **Success Rate:** $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%

## Test Results

EOF

    # Add test results to markdown report
    for test_name in "${!TEST_RESULTS[@]}"; do
        local result="${TEST_RESULTS[$test_name]}"
        local icon=""
        case $result in
            "PASSED") icon="✅" ;;
            "FAILED") icon="❌" ;;
            "SKIPPED") icon="⏭️" ;;
        esac
        echo "- $icon **$test_name**: $result" >> "$report_file"
    done

    # Add recommendations section
    cat >> "$report_file" << EOF

## Recommendations

### If Tests Passed
1. Your Pulsar setup is working correctly
2. Consider running tests regularly for regression detection
3. Use performance baselines for capacity planning

### If Tests Failed
1. Check individual test logs in \`${RESULTS_DIR}/\`
2. Verify Docker/Kubernetes cluster health
3. Check resource availability (CPU, memory, disk)
4. Review Pulsar configuration settings

### Next Steps
1. Customize test parameters for your specific use case
2. Add monitoring and alerting based on test results
3. Integrate tests into your CI/CD pipeline
4. Use results for production deployment planning

## Test Artifacts

- **Test Logs:** \`${LOG_FILE}\`
- **Individual Results:** \`${RESULTS_DIR}/*_${TIMESTAMP}.result\`
- **JSON Report:** \`${json_report}\`

EOF

    # Generate JSON report
    cat > "$json_report" << EOF
{
  "test_run": {
    "timestamp": "${TIMESTAMP}",
    "start_time": ${START_TIME},
    "end_time": $(date +%s),
    "total_tests": ${TOTAL_TESTS},
    "passed_tests": ${PASSED_TESTS},
    "failed_tests": ${FAILED_TESTS},
    "skipped_tests": ${SKIPPED_TESTS},
    "success_rate": $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))
  },
  "test_results": {
EOF

    local first=true
    for test_name in "${!TEST_RESULTS[@]}"; do
        if [ "$first" = false ]; then
            echo "," >> "$json_report"
        fi
        first=false
        echo "    \"$test_name\": \"${TEST_RESULTS[$test_name]}\"" >> "$json_report"
    done

    cat >> "$json_report" << EOF
  }
}
EOF

    # Display summary
    echo ""
    echo -e "${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║                        TEST SUMMARY                          ║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo -e "  Total Tests: ${TOTAL_TESTS}"
    echo -e "  ${GREEN}Passed: ${PASSED_TESTS}${NC}"
    echo -e "  ${RED}Failed: ${FAILED_TESTS}${NC}"
    echo -e "  ${YELLOW}Skipped: ${SKIPPED_TESTS}${NC}"
    echo -e "  Success Rate: $(( (PASSED_TESTS * 100) / TOTAL_TESTS ))%"
    echo ""
    echo -e "  📊 Report: ${report_file}"
    echo -e "  📋 JSON: ${json_report}"
    echo -e "  📝 Logs: ${LOG_FILE}"
    echo ""

    if [ $FAILED_TESTS -gt 0 ]; then
        log_error "Some tests failed. Check the report for details."
        return 1
    else
        log_success "All tests completed successfully!"
        return 0
    fi
}

# Main execution function
main() {
    local test_category="${1:-all}"

    # Record start time
    START_TIME=$(date +%s)

    # Create results directory
    mkdir -p "$RESULTS_DIR"

    log_section "Pulsar Local Lab Test Runner"
    log "Test category: $test_category"
    log "Results directory: $RESULTS_DIR"
    log "Log file: $LOG_FILE"

    # Check prerequisites
    check_prerequisites

    # Run tests based on category
    case "$test_category" in
        "docker")
            run_docker_basic_tests
            ;;
        "ha")
            run_ha_cluster_tests
            ;;
        "k8s")
            run_kubernetes_tests
            ;;
        "performance")
            run_performance_tests
            ;;
        "failover")
            run_failover_tests
            ;;
        "upgrade")
            run_upgrade_tests
            ;;
        "all")
            run_docker_basic_tests
            run_ha_cluster_tests
            run_upgrade_tests
            run_kubernetes_tests
            ;;
        *)
            log_error "Unknown test category: $test_category"
            show_test_categories
            exit 1
            ;;
    esac

    # Generate comprehensive test report
    generate_test_report

    # Exit with appropriate code
    if [ $FAILED_TESTS -gt 0 ]; then
        exit 1
    else
        exit 0
    fi
}

# Script argument handling
case "${1:-}" in
    --help|-h)
        echo "Usage: $0 [test-category]"
        echo ""
        echo "Runs comprehensive Pulsar cluster tests."
        echo ""
        show_test_categories
        echo ""
        echo "Examples:"
        echo "  $0 all          # Run all available tests"
        echo "  $0 docker       # Run only Docker Compose tests"
        echo "  $0 performance  # Run only performance tests"
        echo "  $0 ha           # Run only HA cluster tests"
        echo ""
        echo "Test results are saved to: ./test-results/"
        echo ""
        exit 0
        ;;
    *)
        main "$@"
        ;;
esac
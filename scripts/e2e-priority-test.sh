#!/bin/bash
#
# e2e-priority-test.sh - End-to-end test for /v2/priority/nearest endpoint
#
# This script tests the priority nearest flow in sandbox/staging environments:
# 1. Exchanges an API key for a JWT token via the auth service
# 2. Calls /v2/priority/nearest with the JWT
# 3. Validates the response contains targets with access tokens
#
# Usage:
#   ./scripts/e2e-priority-test.sh [options]
#
# Options:
#   -k, --api-key KEY       API key for token exchange (or set MLAB_API_KEY env var)
#   -e, --env ENV           Environment: sandbox (default) or staging
#   -s, --service SERVICE   Service path (default: ndt/ndt7)
#   -r, --run-test          Actually run an NDT test after locate succeeds
#   -v, --verbose           Enable verbose output
#   -h, --help              Show help
#
# Example:
#   MLAB_API_KEY="mlabk.oi_xxx..." ./scripts/e2e-priority-test.sh -v
#

set -euo pipefail

# Default values
API_KEY="${MLAB_API_KEY:-}"
ENVIRONMENT="sandbox"
SERVICE="ndt/ndt7"
RUN_TEST=false
VERBOSE=false

# Colors for output (if terminal supports it)
if [[ -t 1 ]]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    BLUE='\033[0;34m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    NC=''
fi

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[OK]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

log_verbose() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${BLUE}[DEBUG]${NC} $*"
    fi
}

# Print usage
usage() {
    cat << EOF
Usage: $0 [options]

End-to-end test for /v2/priority/nearest endpoint.

Options:
  -k, --api-key KEY       API key for token exchange (or set MLAB_API_KEY env var)
  -e, --env ENV           Environment: sandbox (default) or staging
  -s, --service SERVICE   Service path (default: ndt/ndt7)
  -r, --run-test          Actually run an NDT test after locate succeeds
  -v, --verbose           Enable verbose output
  -h, --help              Show this help

Environment variables:
  MLAB_API_KEY            API key for token exchange (alternative to -k flag)

Examples:
  # Basic test with API key
  MLAB_API_KEY="mlabk.oi_xxx..." $0 -v

  # Test staging environment
  $0 -k "mlabk.oi_xxx..." -e staging -v

  # Test with actual NDT measurement (requires ndt7-client)
  $0 -k "mlabk.oi_xxx..." -r -v
EOF
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -k|--api-key)
                API_KEY="$2"
                shift 2
                ;;
            -e|--env)
                ENVIRONMENT="$2"
                shift 2
                ;;
            -s|--service)
                SERVICE="$2"
                shift 2
                ;;
            -r|--run-test)
                RUN_TEST=true
                shift
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
}

# Get URLs for environment
get_urls() {
    case "$ENVIRONMENT" in
        sandbox)
            AUTH_URL="https://auth.mlab-sandbox.measurementlab.net/v0/token/integration"
            LOCATE_URL="https://locate-dot-mlab-sandbox.appspot.com"
            ;;
        staging)
            AUTH_URL="https://auth.mlab-staging.measurementlab.net/v0/token/integration"
            LOCATE_URL="https://locate-dot-mlab-staging.appspot.com"
            ;;
        *)
            log_error "Unknown environment: $ENVIRONMENT (use 'sandbox' or 'staging')"
            exit 1
            ;;
    esac
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check for curl
    if ! command -v curl &> /dev/null; then
        log_error "curl is required but not installed"
        exit 1
    fi
    log_verbose "curl found: $(command -v curl)"

    # Check for jq
    if ! command -v jq &> /dev/null; then
        log_error "jq is required but not installed"
        exit 1
    fi
    log_verbose "jq found: $(command -v jq)"

    # Check for API key
    if [[ -z "$API_KEY" ]]; then
        log_error "API key is required. Use -k flag or set MLAB_API_KEY env var"
        exit 1
    fi
    log_verbose "API key provided (${#API_KEY} chars)"

    log_success "Prerequisites check passed"
}

# Exchange API key for JWT
exchange_token() {
    log_info "Exchanging API key for JWT..."
    log_verbose "Auth URL: $AUTH_URL"

    local response
    local http_code

    # Make request and capture both response and HTTP code
    response=$(curl -s -w "\n%{http_code}" -X POST "$AUTH_URL" \
        -H "Content-Type: application/json" \
        -d "{\"api_key\": \"$API_KEY\"}")

    http_code=$(echo "$response" | tail -n1)
    response=$(echo "$response" | sed '$d')

    log_verbose "HTTP status: $http_code"
    log_verbose "Response: $response"

    if [[ "$http_code" != "200" ]]; then
        log_error "Token exchange failed with HTTP $http_code"
        log_error "Response: $response"
        exit 1
    fi

    # Extract token from response
    JWT=$(echo "$response" | jq -r '.token // empty')

    if [[ -z "$JWT" ]]; then
        log_error "No token in response"
        log_error "Response: $response"
        exit 1
    fi

    log_success "Token exchange succeeded"
    log_verbose "JWT (first 50 chars): ${JWT:0:50}..."

    # Decode and show JWT claims if verbose
    if [[ "$VERBOSE" == "true" ]]; then
        local payload
        payload=$(echo "$JWT" | cut -d'.' -f2 | base64 -d 2>/dev/null || true)
        if [[ -n "$payload" ]]; then
            log_verbose "JWT claims: $payload"
        fi
    fi
}

# Call priority nearest endpoint
call_priority_nearest() {
    log_info "Calling /v2/priority/nearest/$SERVICE..."
    log_verbose "Locate URL: $LOCATE_URL/v2/priority/nearest/$SERVICE"

    local response
    local http_code

    # Make request and capture both response and HTTP code
    response=$(curl -s -w "\n%{http_code}" \
        "$LOCATE_URL/v2/priority/nearest/$SERVICE" \
        -H "Authorization: Bearer $JWT")

    http_code=$(echo "$response" | tail -n1)
    response=$(echo "$response" | sed '$d')

    log_verbose "HTTP status: $http_code"

    if [[ "$http_code" != "200" ]]; then
        log_error "Priority nearest failed with HTTP $http_code"
        log_error "Response: $response"
        exit 1
    fi

    # Parse and validate response
    LOCATE_RESPONSE="$response"

    # Check for results array
    local results_count
    results_count=$(echo "$response" | jq '.results | length')

    if [[ "$results_count" == "0" ]] || [[ "$results_count" == "null" ]]; then
        log_error "No results in response"
        log_error "Response: $response"
        exit 1
    fi

    log_success "Priority nearest succeeded with $results_count result(s)"

    # Show results if verbose
    if [[ "$VERBOSE" == "true" ]]; then
        log_verbose "Full response:"
        echo "$response" | jq .
    else
        # Show summary
        echo "$response" | jq -r '.results[] | "  - \(.machine) (\(.location.city // "unknown"), \(.location.country // "unknown"))"'
    fi

    # Validate URLs contain access tokens
    local first_url
    first_url=$(echo "$response" | jq -r '.results[0].urls | to_entries[0].value // empty')

    if [[ -z "$first_url" ]]; then
        log_error "No URLs in first result"
        exit 1
    fi

    if [[ "$first_url" != *"access_token="* ]]; then
        log_warn "URL does not contain access_token parameter"
        log_verbose "URL: $first_url"
    else
        log_success "URLs contain access tokens"
    fi
}

# Run actual NDT test (optional)
run_ndt_test() {
    if [[ "$RUN_TEST" != "true" ]]; then
        return
    fi

    log_info "Running NDT test..."

    # Check for ndt7-client
    if ! command -v ndt7-client &> /dev/null; then
        log_warn "ndt7-client not found, skipping NDT test"
        log_info "Install with: go install github.com/m-lab/ndt7-client-go/cmd/ndt7-client@latest"
        return
    fi

    log_verbose "Using locate URL: $LOCATE_URL"
    log_verbose "Using JWT token for authentication"

    # Run ndt7-client with locate URL and token
    # The client will call the priority endpoint directly with the JWT
    ndt7-client -locate.url "$LOCATE_URL/v2/priority/nearest/" -locate.token "$JWT" -quiet || {
        log_error "NDT test failed"
        exit 1
    }

    log_success "NDT test completed"
}

# Main function
main() {
    parse_args "$@"
    get_urls

    echo ""
    log_info "E2E Priority Nearest Test"
    log_info "Environment: $ENVIRONMENT"
    log_info "Service: $SERVICE"
    echo ""

    check_prerequisites
    exchange_token
    call_priority_nearest
    run_ndt_test

    echo ""
    log_success "All tests passed!"
}

main "$@"

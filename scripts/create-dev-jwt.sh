#!/bin/bash
#
# create-dev-jwt.sh - Generate an unsigned JWT for local development
#
# This script creates a JWT token for use with the heartbeat service in local
# development. The token is unsigned and only works when Locate is running in
# insecure mode (ALLOW_INSECURE_JWT=true, -jwt-auth-mode=insecure).
#
# Usage:
#   ./scripts/create-dev-jwt.sh [org]
#
# Arguments:
#   org - Organization name (default: mlab-sandbox)
#         This must match the organization in your test hostname.
#         For example, "mlab2-lga1t.mlab-sandbox.measurement-lab.org"
#         has org "mlab-sandbox".
#
# Example:
#   export DEV_JWT=$(./scripts/create-dev-jwt.sh)
#   echo $DEV_JWT

set -euo pipefail

# Parse arguments
ORG="${1:-mlab}"

# Calculate expiration (24 hours from now)
EXP=$(($(date +%s) + 86400))

# JWT Header (unsigned, no signature algorithm)
HEADER='{"alg":"none","typ":"JWT"}'

# JWT Payload with required claims
PAYLOAD="{\"org\":\"$ORG\",\"sub\":\"local-dev\",\"exp\":$EXP}"

# Base64url encode function
# Note: Standard base64 uses '+' and '/', base64url uses '-' and '_'
# Also, remove padding '=' characters
base64url_encode() {
    # Use openssl for base64 encoding if available, otherwise use base64 command
    if command -v openssl >/dev/null 2>&1; then
        openssl base64 -e -A | tr '+/' '-_' | tr -d '='
    else
        base64 | tr -d '\n' | tr '+/' '-_' | tr -d '='
    fi
}

# Encode header and payload
HEADER_B64=$(echo -n "$HEADER" | base64url_encode)
PAYLOAD_B64=$(echo -n "$PAYLOAD" | base64url_encode)

# Create unsigned JWT (signature is empty, hence the trailing dot)
JWT="${HEADER_B64}.${PAYLOAD_B64}."

# Output the JWT
echo "$JWT"

# Print debug info to stderr if verbose
if [[ "${VERBOSE:-}" == "1" ]]; then
    >&2 echo "Generated JWT for local development:"
    >&2 echo "  Organization: $ORG"
    >&2 echo "  Subject: local-dev"
    >&2 echo "  Expires: $(date -d @$EXP 2>/dev/null || echo $EXP)"
    >&2 echo "  Token: $JWT"
fi

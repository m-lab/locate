# Local Testing with ESPv1

This guide explains how to test the Locate service locally with Cloud Endpoints ESPv1 proxy.

## Architecture

```
Client → ESP (ESPv1) → Locate Service
                ↓
              Redis
```

- **ESP**: Validates JWTs and sets `X-Endpoint-API-UserInfo` header
- **Locate**: Receives requests with JWT claims already extracted by ESP
- **Redis**: Stores rate limiting data and heartbeat information

## Quick Start

1. **Generate local OpenAPI config**:
   ```bash
   cat openapi.yaml | sed -e 's/{{PROJECT}}/mlab-sandbox/g' -e 's/{{PLATFORM_PROJECT}}/mlab-sandbox/g' -e 's/{{DEPLOYMENT}}/local/g' > openapi.local.yaml
   ```

2. **Start all services**:
   ```bash
   docker compose up --build
   ```

3. **Test without JWT** (regular /v2/nearest endpoint):
   ```bash
   curl "http://localhost:8080/v2/nearest/ndt/ndt7"
   ```

4. **Test with JWT** (priority endpoint with tier-based rate limiting):

   First, you need a valid JWT from the M-Lab token exchange service. For local testing, you can create a test JWT with the required claims.

## Testing with Real JWTs

To test the `/v2/priority/nearest` endpoint with tier-based rate limiting, you need a JWT with:
- `org` claim: Your organization name
- `tier` claim: Your tier number (0, 1, 2, or 3)

The JWT must be signed by the token exchange service and ESP will validate it using the public keys from:
```
https://auth.mlab-sandbox.measurementlab.net/.well-known/jwks.json
```

Example request:
```bash
curl "http://localhost:8080/v2/priority/nearest/ndt/ndt7" \
  -H "Authorization: Bearer YOUR_JWT_HERE"
```

ESP will validate the JWT and set the `X-Endpoint-API-UserInfo` header before forwarding to Locate.

## Testing Heartbeat with JWT

To send heartbeat data with organization validation:

```bash
# Use a JWT from token exchange service
websocat -H "Authorization: Bearer YOUR_JWT_HERE" \
  ws://localhost:8080/v2/platform/heartbeat-jwt
```

Then send heartbeat messages over the WebSocket connection.

## Tier-Based Rate Limiting

The `/v2/priority/nearest` endpoint applies different rate limits based on the `tier` claim in the JWT:

| Tier | Limit (per org+IP) |
|------|-------------------|
| 0    | 100 req/hour      |
| 1    | 500 req/hour      |
| 2    | 1000 req/hour     |
| 3    | 5000 req/hour     |

Configuration is in `limits/config.yaml`.

## Monitoring

- **View Locate logs**: `docker compose logs -f locate`
- **View ESP logs**: `docker compose logs -f esp`
- **Check Redis keys**: `docker compose exec redis redis-cli KEYS 'locate:ratelimit:org:*'`
- **Check rate limit data**: `docker compose exec redis redis-cli ZRANGE locate:ratelimit:org:ORGNAME:IP 0 -1 WITHSCORES`

## Troubleshooting

### ESP can't fetch metadata
This is expected locally. ESP will timeout trying to reach the metadata service but will continue to work for JWT validation.

### JWT validation fails
- Check that the JWT issuer matches the config in `openapi.yaml`
- Verify the JWT audience is `locate` or `autojoin`
- Ensure the JWT is properly signed by the token exchange service

### No heartbeat data
The local instance won't have any M-Lab server data unless you send heartbeats. Use the regular `/v2/platform/heartbeat` endpoint (no JWT required) or send test heartbeats with a valid JWT to `/v2/platform/heartbeat-jwt`.

## Differences from Production

- ESP tries to contact Google Service Control but times out (this is expected)
- No real M-Lab server heartbeat data unless you send it manually
- Prometheus metrics endpoint may not have real data

## JWT Authentication Modes

The Locate service supports three JWT authentication modes for testing and deployment flexibility:

### 1. ESPv1 Mode (Production Default)

Uses Cloud Endpoints ESPv1 to validate JWTs. ESP sets the `X-Endpoint-API-UserInfo` header after validation.

```bash
go run ./locate.go -jwt-auth-mode=espv1
```

**Defense-in-depth**: This mode always verifies that the `Authorization` header (if present) matches the `X-Endpoint-API-UserInfo` header, logging warnings if there's a mismatch.

### 2. Direct Mode (Integration Testing)

Validates JWTs directly by fetching JWKS from a URL. Useful for integration testing without ESP.

```bash
go run ./locate.go \
  -jwt-auth-mode=direct \
  -jwt-jwks-url=https://auth.mlab-sandbox.measurementlab.net/.well-known/jwks.json
```

**Note**: Fetches JWKS on every request (no caching). Not intended for production use.

### 3. Insecure Mode (Development Only)

Parses JWTs without signature verification. **Only for local development/testing**.

```bash
# Requires environment variable as safety check
ALLOW_INSECURE_JWT=true go run ./locate.go -jwt-auth-mode=insecure
```

**Warning**: Never use in production. JWTs are accepted without validation.

### Testing JWT Endpoints Locally

**With Insecure Mode** (easiest for local testing):
```bash
# Start Locate in insecure mode
ALLOW_INSECURE_JWT=true go run ./locate.go \
  -jwt-auth-mode=insecure \
  -redis-address=localhost:6379

# Test with any JWT (signature not checked)
curl "http://localhost:8080/v2/priority/nearest/ndt/ndt7" \
  -H "Authorization: Bearer YOUR_JWT_HERE"
```

**With Direct Mode** (for integration testing):
```bash
# Start Locate with JWKS validation
go run ./locate.go \
  -jwt-auth-mode=direct \
  -jwt-jwks-url=https://auth.mlab-sandbox.measurementlab.net/.well-known/jwks.json \
  -redis-address=localhost:6379

# Test with a real JWT from token exchange service
curl "http://localhost:8080/v2/priority/nearest/ndt/ndt7" \
  -H "Authorization: Bearer YOUR_REAL_JWT_HERE"
```

**With ESPv1 Mode** (requires ESP proxy):
See the Quick Start section above for ESP proxy setup.

## Cleanup

```bash
docker compose down
```

# Multicheck - AI Coding Agent Instructions

## Project Overview
Multicheck is a Go-based REST API service that checks domain and IP reputation against DNS-based blacklists (DNSBL). It uses Redis for caching results and provides concurrent DNS lookups for fast blacklist checking.

## Architecture

### Core Components
- **[main.go](../main.go)** - HTTP server with Gorilla Mux router, defines all API endpoints and request handlers
- **[functions.go](../functions.go)** - DNS blacklist checking logic with concurrent goroutines and WaitGroups
- **[db.go](../db.go)** - Redis connection pool management using redigo
- **[credentialstore/](../credentialstore/)** - Environment-based config for Redis (REDIS_HOST, REDIS_PORT)
- **[config.toml](../config.toml)** - Blacklist configuration, cache TTL, nameservers, listen port

### Data Flow
1. Client requests `/ip/{ip}` or `/domain/{domain}`
2. Handler checks Redis cache first (`getRedisKey`)
3. On cache miss: concurrent DNS lookups against all blacklists using goroutines (`checkBlacklistIP`/`checkBlacklistDomain`)
4. Results cached in Redis with TTL (`setRedisKey`)
5. JSON response with blacklist status, timing, and cache hit indicator

## Key Patterns

### Concurrent DNS Lookups
All blacklist checks use `sync.WaitGroup` with goroutines for parallel DNS queries. Each goroutine:
- Performs DNS lookup via custom resolver with random nameserver selection
- Filters out `127.0.0.1` and `127.255.255.255` from results (false positives)
- Updates shared map `blacklistsActive` when blacklisted
- Reports errors via buffered channel `errorCh`

Example: `checkIPDNS()` and `checkDomainDNS()` in [functions.go](../functions.go#L100-L200)

### Custom DNS Resolver
Uses `net.Resolver` with custom dialer to randomly select nameservers from [config.toml](../config.toml). This avoids system DNS and provides redundancy.

### Redis Caching Strategy
- Cache key: IP address or domain name (as-is)
- TTL: `redisCacheTTL` seconds from config (default 300s)
- Cached response includes all fields: `BlackListed`, `BlackList`, `ValidIP`/`ValidDomain`, `TimeTaken`
- Set `Cached: true` in response when served from Redis

### Configuration Loading
Uses Viper to read [config.toml](../config.toml). Supports `GSS_CONFIG_PATH` env var to override config location. All multiline strings in TOML are space-separated lists.

### Structured JSON Logging
Every request logs to stdout in JSON format with: `CurrentTime`, `Method`, `Param`, `MemoryAlloc`, `NumGC`, `TimeTaken`, `Cached`, `ClientIP`, `Redis` status, `RedisConnections`. Parse with `jq` for debugging.

## Development Workflow

### Build & Run
```bash
make build    # Compiles to bin/multicheck with stripped binary
make run      # Runs binary piping output through jq
make test     # Runs go test -v
```

### Docker Deployment
```bash
docker-compose up    # Starts multicheck + redis containers
```
- Service exposed on port 8080
- Depends on Redis at `redis:6379` (container network)
- Dockerfile uses multi-stage build: Go builder → Alpine runtime

### Testing
[main_test.go](../main_test.go) tests `/health` endpoint. Expects Redis to be running locally. Add tests for blacklist checking by mocking DNS resolver or Redis responses.

## API Endpoints
- `GET /` - Lists all endpoints and configuration
- `GET /health` - Health check with Redis status and uptime
- `GET /ip/{ip}` - Check IP against blacklists (validates with `net.ParseIP`)
- `GET /domain/{domain}` - Check domain against blacklists (validates with `validator.IsValidDomain`)
- `GET /clear-cache/{key}` - Delete specific cache entry by key

## Important Implementation Notes

### Adding New Blacklists
Update `ipBlacklist` or `domainBlacklist` arrays in [config.toml](../config.toml). Space-separate entries. No code changes needed.

### Response Headers
All endpoints set `Content-Type: application/json` and `Cache-Control: max-age=<cacheControlMaxAge>` from config.

### Error Handling
- Redis failures append to `Errors[]` array but don't block request
- Invalid input sets `Status: false` and returns early
- DNS lookup errors filtered (ignore "no such host" as expected for non-blacklisted)

### Global State
Package-level variables: `ip`, `domain`, `health`, `clearCache`, `configuration`, `c` (Redis pool), `resolver`. These are shared across requests but mostly read-only after initialization except for result structs.

### IP Reversal for DNSBL
IPs must be reversed for DNSBL queries: `1.2.3.4` becomes `4.3.2.1.dnsbl.example.org`. See `reverseIP()` function.

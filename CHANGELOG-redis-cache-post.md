# Changelog - Redis Cache for POST Endpoints

## Date

January 17, 2026

## Summary

Implemented Redis caching for POST endpoints (`/ip/check` and `/domain/check`) to improve performance when checking custom blacklists. Cache keys are generated using a hash of the sorted blacklist array, ensuring consistent caching regardless of blacklist order or nameserver selection.

## Changes

### Core Features

#### Redis Caching for POST Endpoints

- **POST /ip/check** now uses Redis cache with key format: `post:ip:<ip>:<hash>`
- **POST /domain/check** now uses Redis cache with key format: `post:domain:<domain>:<hash>`
- Hash is generated from SHA256 of sorted blacklist array (truncated to 16 characters)
- Cache TTL respects `redisCacheTTL` configuration (default 300 seconds)
- Response includes `Cached: true` when served from Redis

#### Cache Key Strategy

- **GET endpoints**: Simple keys (IP or domain as-is)
- **POST endpoints**: Composite keys with hash to avoid collisions
- **Cache independence**: Nameservers don't affect cache keys (DNS results should be consistent)
- **Order independence**: Blacklist array order doesn't matter (sorted before hashing)

### New Functions

#### `generateBlacklistHash()` in functions.go

```go
func generateBlacklistHash(blacklists []string) string
```

- Sorts blacklist array for consistent ordering
- Creates SHA256 hash of comma-separated sorted blacklists
- Returns truncated 16-character hash for readability

#### `buildPostCacheKey()` in functions.go

```go
func buildPostCacheKey(keyType, identifier string, blacklists []string) string
```

- Generates complete cache key for POST endpoints
- Format: `post:ip:<ip>:<hash>` or `post:domain:<domain>:<hash>`
- Uses `generateBlacklistHash()` internally

### Modified Functions

#### `PostCheckIp()` in main.go

- Added cache lookup before DNS queries
- Returns cached result when available
- Saves result to cache after DNS check
- Logs cache hit/miss status

#### `PostCheckDomain()` in main.go

- Added cache lookup before DNS queries
- Returns cached result when available
- Saves result to cache after DNS check
- Logs cache hit/miss status

### Updated Imports

- Added `crypto/sha256` and `encoding/hex` to main.go
- Added `sort` to main.go for blacklist sorting
- Added corresponding imports to functions.go

## Benefits

1. **Performance**: Repeated requests with same blacklists are served from cache (typically <1ms vs 200-500ms for DNS lookups)
2. **Efficiency**: Reduces DNS query load for popular blacklist combinations
3. **Flexibility**: Different nameservers with same blacklists → cache hit
4. **Consistency**: Blacklist array order doesn't matter → same hash
5. **No collision**: POST and GET caches are separate (different key prefixes)

## Examples

### Cache Hit Scenario

```bash
# First request (cache miss)
curl -X POST http://localhost:8080/ip/check \
  -d '{"ip":"1.2.3.4","blacklists":["zen.spamhaus.org","bl.spamcop.net"]}'
# Response: "Cached": false, "TimeTaken": 0.312

# Second identical request (cache hit)
curl -X POST http://localhost:8080/ip/check \
  -d '{"ip":"1.2.3.4","blacklists":["zen.spamhaus.org","bl.spamcop.net"]}'
# Response: "Cached": true, "TimeTaken": 0.002
```

### Order Independence

```bash
# Request 1
curl -X POST http://localhost:8080/ip/check \
  -d '{"ip":"1.2.3.4","blacklists":["bl1.org","bl2.org"]}'

# Request 2 (different order, same blacklists → cache hit)
curl -X POST http://localhost:8080/ip/check \
  -d '{"ip":"1.2.3.4","blacklists":["bl2.org","bl1.org"]}'
```

### Nameserver Independence

```bash
# Request 1 with nameserver A
curl -X POST http://localhost:8080/ip/check \
  -d '{"ip":"1.2.3.4","blacklists":["bl1.org"],"nameservers":["8.8.8.8"]}'

# Request 2 with nameserver B (same blacklists → cache hit)
curl -X POST http://localhost:8080/ip/check \
  -d '{"ip":"1.2.3.4","blacklists":["bl1.org"],"nameservers":["1.1.1.1"]}'
```

## Testing

Existing tests continue to pass:

- `TestPostCheckIP` - Verifies POST /ip/check functionality
- `TestPostCheckDomain` - Verifies POST /domain/check functionality

## Documentation Updates

- Updated README.md with cache behavior for POST endpoints
- Updated copilot-instructions.md with new cache key formats
- Added clear-cache examples for POST endpoint keys

## Configuration

No configuration changes required. Uses existing `redisCacheTTL` setting from config.toml.

## Breaking Changes

None. This is a backward-compatible enhancement that adds caching without changing API behavior or response format.

## Migration Notes

- Old POST requests without cache will now be cached automatically
- No changes needed to client code
- Cache will populate naturally as requests are made
- Optional: Clear all Redis keys with pattern `post:*` to start fresh

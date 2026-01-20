# Changelog: Cache Key Exposure in API Responses

**Date:** January 20, 2026  
**Feature:** Expose Redis cache key in all API responses

## Problem

When using POST endpoints (`/ip/check` and `/domain/check`) with custom blacklists, the cache key format is `post:ip:<ip>:<hash>` or `post:domain:<domain>:<hash>`, where the hash is generated from the sorted blacklist array (SHA256 truncated to 16 characters).

**Issue:** Users and the frontend could not delete cached entries for POST requests because:
1. The hash is not predictable without knowing the exact blacklist array
2. The complete cache key was not exposed in API responses
3. The `/clear-cache/{key}` endpoint requires the exact cache key

This made cache invalidation impossible for POST endpoint results when using custom blacklists.

## Solution

Added a `CacheKey` field to all API responses that contains the exact Redis key used for caching. This enables:

- **Frontend integration**: The web interface can now show and use the cache key to clear entries
- **API transparency**: Users know exactly which Redis key is storing their cached result
- **Easy cache invalidation**: Simple copy-paste of the `CacheKey` value to `/clear-cache/{key}`

## Changes

### Backend (Go)

1. **Struct updates** ([main.go](main.go)):
   - Added `CacheKey string` field to `Ip` struct
   - Added `CacheKey string` field to `Domain` struct

2. **Handler updates** ([main.go](main.go)):
   - `GetIp`: Populates `CacheKey` with simple key (IP address)
   - `GetDomain`: Populates `CacheKey` with simple key (domain name)
   - `PostCheckIp`: Populates `CacheKey` with composite key `post:ip:<ip>:<hash>`
   - `PostCheckDomain`: Populates `CacheKey` with composite key `post:domain:<domain>:<hash>`

3. **Tests** ([main_test.go](main_test.go)):
   - `TestPostCheckIpCacheKey`: Verifies cache key format and consistency for IP POST requests
   - `TestPostCheckDomainCacheKey`: Verifies cache key format and consistency for domain POST requests
   - `TestPostCheckIpCacheDeletionWithCacheKey`: Verifies cache deletion using exposed CacheKey

### Documentation

Updated [README.md](README.md) with:
- All response examples now include `CacheKey` field
- Enhanced `/clear-cache/{key}` documentation explaining the use of exposed cache keys
- Note explaining that `CacheKey` should be used directly from API responses

## API Response Examples

### GET /ip/{ip}

```json
{
  "IP": "1.2.3.4",
  "ValidIP": true,
  "BlackListed": false,
  "Status": true,
  "BlackList": {},
  "Errors": [],
  "TimeTaken": 0.245,
  "Cached": false,
  "CacheKey": "1.2.3.4"
}
```

### POST /ip/check

```json
{
  "IP": "1.2.3.4",
  "ValidIP": true,
  "BlackListed": false,
  "Status": true,
  "BlackList": {},
  "Errors": [],
  "TimeTaken": 0.312,
  "Cached": false,
  "CacheKey": "post:ip:1.2.3.4:a3f5c8d12e9b7f6a"
}
```

### GET /domain/{domain}

```json
{
  "Domain": "example.com",
  "ValidDomain": true,
  "BlackListed": false,
  "Status": true,
  "BlackList": {},
  "Errors": [],
  "TimeTaken": 0.198,
  "Cached": false,
  "CacheKey": "example.com"
}
```

### POST /domain/check

```json
{
  "Domain": "example.com",
  "ValidDomain": true,
  "BlackListed": false,
  "Status": true,
  "BlackList": {},
  "Errors": [],
  "TimeTaken": 0.245,
  "Cached": false,
  "CacheKey": "post:domain:example.com:f7b2d9e4c1a8f563"
}
```

## Usage: Clearing Cache

### Simple GET endpoints
```bash
# Get response with CacheKey
curl http://localhost:8080/ip/1.2.3.4
# Response includes: "CacheKey": "1.2.3.4"

# Clear cache using exposed key
curl http://localhost:8080/clear-cache/1.2.3.4
```

### POST endpoints with custom blacklists
```bash
# POST request with custom blacklists
curl -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{"ip":"1.2.3.4","blacklists":["zen.spamhaus.org","bl.spamcop.net"]}'
# Response includes: "CacheKey": "post:ip:1.2.3.4:a3f5c8d12e9b7f6a"

# Clear cache using the exact CacheKey from response
curl http://localhost:8080/clear-cache/post:ip:1.2.3.4:a3f5c8d12e9b7f6a
```

## Test Results

All tests pass successfully:

```bash
$ go test -v -run "TestPostCheckIpCacheKey"
=== RUN   TestPostCheckIpCacheKey
    main_test.go:617: CacheKey verified: post:ip:2.0.0.127:9cec28bb3fb09ee0 (Cached: false -> true)
--- PASS: TestPostCheckIpCacheKey (0.10s)
PASS

$ go test -v -run "TestPostCheckDomainCacheKey"
=== RUN   TestPostCheckDomainCacheKey
    main_test.go:696: CacheKey verified: post:domain:test.uribl.com:6e1d90c239c51b2f (Cached: false -> true)
--- PASS: TestPostCheckDomainCacheKey (0.01s)
PASS

$ go test -v -run "TestPostCheckIpCacheDeletionWithCacheKey"
=== RUN   TestPostCheckIpCacheDeletionWithCacheKey
    main_test.go:777: Cache deletion with CacheKey verified: post:ip:2.0.0.127:9aa07d980f663860 (Cached: true -> deleted -> false)
--- PASS: TestPostCheckIpCacheDeletionWithCacheKey (0.01s)
PASS
```

## Impact

- **No breaking changes**: Existing API consumers will simply see a new field in responses
- **Backward compatible**: All existing functionality remains unchanged
- **Frontend ready**: The web interface can now implement cache clearing for POST requests with custom blacklists
- **Better UX**: Users have full visibility into caching behavior and can manage cache entries effectively

## Next Steps

The frontend can now be updated to:
1. Display the `CacheKey` in the results UI
2. Add a "Clear Cache" button that uses the exposed `CacheKey`
3. Show cache status indicators based on the `Cached` field
4. Implement cache management features in the history panel

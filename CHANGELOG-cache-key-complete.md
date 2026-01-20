# Changelog - Cache Key Exposure (Backend + Frontend)

**Date:** January 20, 2026  
**Version:** Post-1.0.0  
**Type:** Feature Enhancement

## Overview

This update exposes the Redis cache key in API responses and updates the frontend to use it. This fixes a critical issue where cache deletion was impossible for POST endpoints with custom blacklists.

## Problem Statement

### Before This Update ❌

**Backend:**
- API responses did NOT include the cache key
- POST endpoints used composite keys: `post:ip:<ip>:<hash>` 
- Hash was generated from sorted blacklist array (SHA256, truncated to 16 chars)
- Users had no way to know the exact cache key

**Frontend:**
- Constructed cache keys manually using only IP/domain
- Cache deletion worked for GET endpoints (simple keys)
- Cache deletion FAILED for POST endpoints (composite keys with hash)
- No visibility into actual cache keys being used

**Impact:**
- Users with custom blacklists (Advanced Options) couldn't clear cache
- Frontend displayed "Cache cleared" but nothing actually happened
- No transparency about caching behavior

### After This Update ✅

**Backend:**
- All API responses include `CacheKey` field
- Exposes the exact Redis key used for caching
- Works for both GET and POST endpoints

**Frontend:**
- Uses `CacheKey` from API response
- Cache deletion works for ALL endpoints
- Displays cache key in collapsible section for transparency

## Changes Summary

### Backend Changes

**Files Modified:**
- `main.go` - Added `CacheKey` field to `Ip` and `Domain` structs
- `main.go` - Updated all handlers to populate `CacheKey`
- `main_test.go` - Added 3 new tests for cache key verification
- `README.md` - Updated API response examples

**New Tests:**
- `TestPostCheckIpCacheKey` - Verifies IP POST endpoint exposes correct cache key
- `TestPostCheckDomainCacheKey` - Verifies domain POST endpoint exposes correct cache key  
- `TestPostCheckIpCacheDeletionWithCacheKey` - Verifies cache deletion works with exposed key

**Test Results:**
```bash
✅ TestPostCheckIpCacheKey (0.10s)
✅ TestPostCheckDomainCacheKey (0.01s)
✅ TestPostCheckIpCacheDeletionWithCacheKey (0.01s)
✅ All existing tests still pass (no regressions)
```

### Frontend Changes

**Files Modified:**
- `src/lib/types.ts` - Added `CacheKey` field to `IpResponse` and `DomainResponse`
- `src/lib/components/ResultsCard.svelte` - Updated cache clearing logic and UI

**Key Improvements:**
1. Cache clearing now uses `result.CacheKey` instead of manually constructing key
2. Added collapsible "Cache Key" section in result metadata
3. Added copy-to-clipboard for cache key
4. Added validation check if cache key is missing

## API Response Changes

### GET Endpoints

**Before:**
```json
{
  "IP": "1.2.3.4",
  "BlackListed": false,
  "Cached": false
}
```

**After:**
```json
{
  "IP": "1.2.3.4",
  "BlackListed": false,
  "Cached": false,
  "CacheKey": "1.2.3.4"  ← NEW FIELD
}
```

### POST Endpoints

**Before:**
```json
{
  "IP": "1.2.3.4",
  "BlackListed": false,
  "Cached": false
}
```

**After:**
```json
{
  "IP": "1.2.3.4",
  "BlackListed": false,
  "Cached": false,
  "CacheKey": "post:ip:1.2.3.4:a3f5c8d12e9b7f6a"  ← NEW FIELD
}
```

## Usage Examples

### Using CacheKey with curl

```bash
# 1. Make a POST request with custom blacklists
RESPONSE=$(curl -s -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{"ip":"8.8.8.8","blacklists":["zen.spamhaus.org","bl.spamcop.net"]}')

# 2. Extract the cache key
CACHE_KEY=$(echo $RESPONSE | jq -r '.CacheKey')
echo "Cache Key: $CACHE_KEY"
# Output: Cache Key: post:ip:8.8.8.8:9cec28bb3fb09ee0

# 3. Clear the cache using the exact key
curl http://localhost:8080/clear-cache/$CACHE_KEY
# Output: {"Status":true,"Key":"post:ip:8.8.8.8:9cec28bb3fb09ee0","Errors":null,"TimeTaken":0.0004}
```

### Frontend Usage

**Old Code (Broken for POST):**
```typescript
async function handleClearCache() {
  const key = result.IP;  // ❌ Wrong for POST endpoints
  await clearCache(key);
}
```

**New Code (Works for All):**
```typescript
async function handleClearCache() {
  const cacheKey = result.CacheKey;  // ✅ Always correct
  await clearCache(cacheKey);
}
```

## Cache Key Formats

### GET Endpoints (Default Blacklists)

| Endpoint | Example Input | Cache Key Format | Example |
|----------|--------------|------------------|---------|
| GET /ip/{ip} | 1.2.3.4 | IP address | `1.2.3.4` |
| GET /domain/{domain} | example.com | Domain name | `example.com` |

### POST Endpoints (Custom Blacklists)

| Endpoint | Example Input | Cache Key Format | Example |
|----------|--------------|------------------|---------|
| POST /ip/check | 1.2.3.4 + blacklists | `post:ip:<ip>:<hash>` | `post:ip:1.2.3.4:a3f5c8d1` |
| POST /domain/check | example.com + blacklists | `post:domain:<domain>:<hash>` | `post:domain:example.com:f7b2d9e4` |

**Hash Generation:**
- Blacklists are sorted alphabetically
- SHA256 hash computed from sorted array
- Hash truncated to 16 characters
- Same blacklists in different order = same cache key

## Testing

### Automated Tests

```bash
# Run all tests
make test

# Run specific cache key tests
go test -v -run "CacheKey"
```

### Manual Testing

**1. Test GET endpoint cache key:**
```bash
curl -s http://localhost:8080/ip/8.8.8.8 | jq '{IP, CacheKey}'
```

**2. Test POST endpoint cache key:**
```bash
curl -s -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{"ip":"8.8.8.8","blacklists":["zen.spamhaus.org"]}' \
  | jq '{IP, CacheKey}'
```

**3. Test cache deletion:**
```bash
# Get the cache key from previous request
CACHE_KEY="post:ip:8.8.8.8:9aa07d980f663860"

# Clear the cache
curl http://localhost:8080/clear-cache/$CACHE_KEY | jq
```

**4. Test frontend (Advanced Options):**
1. Open http://localhost:5173
2. Enter IP: 8.8.8.8
3. Enable "Advanced Options"
4. Add custom blacklist: zen.spamhaus.org
5. Submit check
6. Click "Clear cache" button → Should work ✅
7. Expand "Cache Key" section → Should show: `post:ip:8.8.8.8:xxxxx`

## Migration Guide

### For API Consumers

**No breaking changes** - this is a backward-compatible addition:
- Existing code continues to work
- New `CacheKey` field is always present
- Old cache deletion methods still work (for GET endpoints)

**Recommended updates:**
```typescript
// Old way (still works for GET)
await clearCache(ipAddress);

// New way (works for ALL endpoints)
const response = await checkIp(ipAddress);
await clearCache(response.CacheKey);
```

### For Frontend Developers

**Required changes:**
1. Update TypeScript interfaces to include `CacheKey: string`
2. Update cache clearing logic to use `result.CacheKey`
3. Test with both GET and POST endpoints

See [CHANGELOG-frontend-cache-key.md](CHANGELOG-frontend-cache-key.md) for detailed frontend changes.

## Benefits

### ✅ Fixed Critical Bug
- Cache deletion now works for POST endpoints with custom blacklists
- Users can clear cache regardless of which endpoint they use

### ✅ Improved Transparency
- Users can see exact cache keys being used
- Easier debugging of caching behavior
- Better understanding of how cache works

### ✅ API Consistency
- All endpoints follow same pattern
- No special handling needed for different endpoint types

### ✅ Future-Proof
- Backend controls cache key generation
- Frontend doesn't need to replicate hash logic
- Easy to change cache key format in future

## Related Documentation

- [CHANGELOG-expose-cache-key.md](CHANGELOG-expose-cache-key.md) - Backend implementation details
- [CHANGELOG-frontend-cache-key.md](CHANGELOG-frontend-cache-key.md) - Frontend implementation details
- [README.md](README.md) - Updated API documentation with examples

## Technical Notes

### Why Hash in Cache Key?

The hash in POST endpoint cache keys serves several purposes:

1. **Uniqueness:** Different blacklist combinations get separate cache entries
2. **Consistency:** Same blacklists in different order share cache (sorted before hashing)
3. **Compact:** 16-char hash instead of full blacklist list in key
4. **Security:** Prevents Redis key injection attacks

### Cache Key Structure

```
Format: post:<type>:<identifier>:<hash>

Examples:
- post:ip:1.2.3.4:a3f5c8d12e9b7f6a
  ├─ prefix: "post"
  ├─ type: "ip"
  ├─ identifier: "1.2.3.4"
  └─ hash: "a3f5c8d12e9b7f6a" (16 chars, from sorted blacklists)

- example.com
  └─ Simple key for GET endpoint (no prefix/hash)
```

## Future Enhancements

Potential improvements based on this foundation:

- [ ] Batch cache deletion (multiple keys at once)
- [ ] Cache key pattern search (e.g., all POST keys for an IP)
- [ ] Cache key statistics endpoint
- [ ] Cache key expiration time display
- [ ] Frontend: Show cache key in history items
- [ ] Frontend: Filter history by cache status

## Version Compatibility

- **Backend:** Compatible with Go 1.25+
- **Frontend:** Compatible with Node.js 18+
- **API:** Backward compatible (no breaking changes)
- **Redis:** No schema changes required

## Contributors

Implementation by AI Coding Agent based on user feedback about cache deletion issues with Advanced Options.

---

**Last Updated:** January 20, 2026

# Testing Cache Key Functionality

This document explains how to test the cache key functionality implemented in both backend and frontend.

## Quick Test

Run the automated integration test:

```bash
make test-cache-key
```

Or directly:

```bash
./test-cache-key.sh
```

## What Gets Tested

The test script verifies:

1. ✅ GET endpoints expose simple cache keys (`"1.1.1.1"`, `"example.com"`)
2. ✅ POST endpoints expose composite cache keys (`"post:ip:1.2.3.4:abc123"`)
3. ✅ Cache deletion works for POST endpoints with custom blacklists
4. ✅ Different blacklists produce different cache keys
5. ✅ Same blacklists in different order produce the same cache key (cache hit)

## Manual Testing

### Test GET Endpoint

```bash
# Check IP with default blacklists
curl -s http://localhost:8080/ip/1.1.1.1 | jq '{IP, Cached, CacheKey}'

# Expected output:
{
  "IP": "1.1.1.1",
  "Cached": false,
  "CacheKey": "1.1.1.1"
}
```

### Test POST Endpoint (Advanced Options)

```bash
# Check IP with custom blacklists
curl -s -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{"ip":"8.8.8.8","blacklists":["zen.spamhaus.org","bl.spamcop.net"]}' \
  | jq '{IP, Cached, CacheKey}'

# Expected output:
{
  "IP": "8.8.8.8",
  "Cached": false,
  "CacheKey": "post:ip:8.8.8.8:9cec28bb3fb09ee0"
}
```

### Test Cache Deletion

```bash
# 1. Make a request and save the cache key
RESPONSE=$(curl -s -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{"ip":"8.8.8.8","blacklists":["zen.spamhaus.org"]}')

CACHE_KEY=$(echo $RESPONSE | jq -r '.CacheKey')
echo "Cache Key: $CACHE_KEY"

# 2. Clear the cache using the key
curl -s http://localhost:8080/clear-cache/$CACHE_KEY | jq

# Expected output:
{
  "Status": true,
  "Key": "post:ip:8.8.8.8:9aa07d980f663860",
  "Errors": null,
  "TimeTaken": 0.0004
}
```

## Frontend Testing

### Test in Browser

1. Open http://localhost:5173
2. Enter IP: `8.8.8.8`
3. Click "Advanced Options"
4. Add custom blacklist: `zen.spamhaus.org`
5. Click "Check"
6. In results:
   - Verify "Cached: No" on first request
   - Expand "Cache Key" section
   - Verify key format: `post:ip:8.8.8.8:xxxxx`
   - Copy cache key to clipboard
7. Click "Check" again
   - Verify "Cached: Yes" on second request
   - Verify same cache key
8. Click trash icon (Clear cache)
   - Should show success toast
9. Click "Check" again
   - Verify "Cached: No" (cache was cleared)

### Expected Behavior

**Before this update (BROKEN):**
- Cache deletion with custom blacklists didn't work
- Frontend used wrong key (only IP/domain, missing hash)
- Cache appeared cleared but remained in Redis

**After this update (FIXED):**
- Cache deletion works for all endpoints
- Frontend uses exact key from API response
- Cache is actually cleared from Redis

## Cache Key Formats

### GET Endpoints
- Format: `<ip>` or `<domain>`
- Examples:
  - `1.2.3.4`
  - `example.com`

### POST Endpoints
- Format: `post:<type>:<identifier>:<hash>`
- Hash: 16-char truncated SHA256 of sorted blacklists
- Examples:
  - `post:ip:1.2.3.4:a3f5c8d12e9b7f6a`
  - `post:domain:example.com:f7b2d9e4c1a8f563`

## Troubleshooting

### Cache Key is null/undefined

**Problem:** API response shows `"CacheKey": null`

**Solution:**
- Rebuild backend: `make build`
- Restart containers: `make build-compose`
- Ensure using latest code

### Cache Deletion Fails

**Problem:** Clear cache shows success but item remains cached

**Solution:**
- Check you're using the exact `CacheKey` from API response
- Don't construct cache key manually (that was the old broken method)
- Verify backend is returning `CacheKey` field

### Frontend Not Updated

**Problem:** Frontend still showing old behavior

**Solution:**
```bash
cd frontend
npm install  # Update dependencies if needed
npm run build  # Rebuild
# Restart dev server
pkill -f "vite dev"
npm run dev
```

### Clear Browser Cache

If seeing stale frontend code:
1. Open DevTools (F12)
2. Right-click refresh button
3. Select "Empty Cache and Hard Reload"

## Related Documentation

- [CHANGELOG-cache-key-complete.md](../CHANGELOG-cache-key-complete.md) - Complete implementation details
- [CHANGELOG-expose-cache-key.md](../CHANGELOG-expose-cache-key.md) - Backend changes
- [CHANGELOG-frontend-cache-key.md](../CHANGELOG-frontend-cache-key.md) - Frontend changes
- [README.md](../README.md) - API documentation

## Test Checklist

Use this checklist when testing:

- [ ] Backend tests pass (`make test`)
- [ ] Integration tests pass (`make test-cache-key`)
- [ ] Frontend compiles without errors (`cd frontend && npm run build`)
- [ ] GET endpoints return simple cache keys
- [ ] POST endpoints return composite cache keys with hash
- [ ] Cache clearing works in browser (Advanced Options)
- [ ] Cache key is visible in UI (collapsible section)
- [ ] Second request with same params shows "Cached: Yes"
- [ ] After clearing cache, next request shows "Cached: No"

## Performance Notes

The cache key functionality adds minimal overhead:
- Hash computation: ~0.001ms (done once per request)
- JSON serialization includes one extra string field
- No impact on DNS lookup performance
- Cache hit rate remains the same

## Security Notes

The cache key is safe to expose:
- Contains no sensitive information
- IP/domain are already in request/response
- Hash is one-way (can't reverse to get blacklist array)
- No risk of cache poisoning attacks

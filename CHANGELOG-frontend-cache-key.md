# Changelog - Frontend: Support for CacheKey Field

**Date:** January 20, 2026

## Summary

Updated the frontend to use the new `CacheKey` field exposed by the backend API. This fixes cache deletion for POST endpoints (Advanced Options) and provides transparency about cache keys.

## Problem Fixed

Before this update, the frontend could not correctly clear the cache for requests made with custom blacklists (Advanced Options using POST endpoints) because:

1. The frontend constructed cache keys manually using only IP/domain
2. For POST endpoints, the backend uses composite keys like `post:ip:<ip>:<hash>`
3. The hash was not accessible to the frontend, making cache deletion impossible

## Changes

### TypeScript Interfaces (`src/lib/types.ts`)

**Updated `IpResponse` interface:**
```typescript
export interface IpResponse {
	// ... existing fields
	CacheKey: string;  // ← NEW FIELD
}
```

**Updated `DomainResponse` interface:**
```typescript
export interface DomainResponse {
	// ... existing fields
	CacheKey: string;  // ← NEW FIELD
}
```

### ResultsCard Component (`src/lib/components/ResultsCard.svelte`)

**1. Updated `handleClearCache()` function:**

**Before:**
```typescript
async function handleClearCache() {
	const key = checkType === 'ip' 
		? (result as IpResponse).IP 
		: (result as DomainResponse).Domain;
	
	if (!confirm(`Clear cache for ${key}?`)) return;
	
	await clearCache(key);  // ❌ Wrong key for POST endpoints
}
```

**After:**
```typescript
async function handleClearCache() {
	// Use CacheKey from API response instead of constructing manually
	const cacheKey = result.CacheKey;
	const displayKey = checkType === 'ip' 
		? (result as IpResponse).IP 
		: (result as DomainResponse).Domain;
	
	if (!cacheKey) {
		toast.error('No cache key available');
		return;
	}
	
	if (!confirm(`Clear cache for ${displayKey}?`)) return;
	
	await clearCache(cacheKey);  // ✅ Correct key from API
}
```

**2. Added Cache Key Display (collapsible):**

Added a collapsible `<details>` section in the metadata area that shows:
- The exact cache key used by the backend
- Copy-to-clipboard button for the cache key
- Useful for debugging and advanced users

Example display:
```
▶ Cache Key
  post:ip:8.8.8.8:9cec28bb3fb09ee0  [📋 Copy]
```

## Benefits

### ✅ Fixes Cache Deletion for Advanced Options
- POST requests with custom blacklists can now have their cache cleared correctly
- Frontend uses the exact cache key provided by the backend

### ✅ Transparency
- Users can see the actual cache key being used
- Useful for debugging and understanding caching behavior

### ✅ Consistency
- Both GET and POST endpoints work the same way
- No more manual key construction in frontend

## Testing

### Manual Testing Checklist

1. **GET endpoint cache deletion:**
   - Check IP/domain with default options
   - Verify cache key is simple: `"1.1.1.1"` or `"example.com"`
   - Clear cache - should work ✅

2. **POST endpoint cache deletion (Advanced Options):**
   - Check IP/domain with custom blacklists
   - Verify cache key is composite: `"post:ip:1.2.3.4:abc123def456"`
   - Clear cache - should work ✅ (this was broken before)

3. **Cache Key Display:**
   - Verify cache key is visible in collapsible section
   - Test copy-to-clipboard functionality

### Example Test Scenarios

**Scenario 1: GET request (default blacklists)**
```bash
curl http://localhost:8080/ip/8.8.8.8 | jq '.CacheKey'
# Returns: "8.8.8.8"
# Frontend clear cache: works ✅
```

**Scenario 2: POST request (custom blacklists)**
```bash
curl -X POST http://localhost:8080/ip/check \
  -d '{"ip":"8.8.8.8","blacklists":["zen.spamhaus.org"]}' | jq '.CacheKey'
# Returns: "post:ip:8.8.8.8:9aa07d980f663860"
# Frontend clear cache: NOW works ✅ (was broken before)
```

## Developer Notes

### Why CacheKey is Important

The backend generates cache keys differently for GET vs POST endpoints:

- **GET endpoints:** Simple key = IP or domain
  - Example: `"1.2.3.4"` or `"example.com"`

- **POST endpoints:** Composite key = prefix + identifier + hash
  - Format: `post:ip:<ip>:<hash>` or `post:domain:<domain>:<hash>`
  - Hash is SHA256 of sorted blacklist array (truncated to 16 chars)
  - Example: `"post:ip:1.2.3.4:a3f5c8d12e9b7f6a"`

The hash ensures that:
1. Different blacklist combinations get separate cache entries
2. Same blacklists in different order share cache (sorted before hashing)
3. Frontend doesn't need to replicate hash generation logic

### Future Improvements

- [ ] Add "View Cache Key" button directly in the UI (not just in collapsible)
- [ ] Show cache key in request history items
- [ ] Add cache key to JSON copy output
- [ ] Implement batch cache clearing for multiple history items

## Related Changes

This frontend update complements the backend changes in:
- [CHANGELOG-expose-cache-key.md](CHANGELOG-expose-cache-key.md) - Backend implementation

## Files Changed

- `frontend/src/lib/types.ts` - Added `CacheKey` field to interfaces
- `frontend/src/lib/components/ResultsCard.svelte` - Updated cache clearing logic and UI

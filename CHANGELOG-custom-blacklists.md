# Changelog - Custom Blacklist Feature

## Summary

Added POST endpoints to allow clients to specify custom blacklists for IP and domain reputation checks, with comprehensive input validation and resource protection. Enhanced JSON logging to include HTTP method and request body for better debugging and auditing.

## New Features

### Custom Blacklist Endpoints

### POST /ip/check
Check an IP address against a custom list of DNS blacklists.

**Example:**
```bash
curl -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{
    "ip": "1.2.3.4",
    "blacklists": ["zen.spamhaus.org", "bl.spamcop.net"]
  }'
```

### POST /domain/check
Check a domain against a custom list of DNS blacklists.

**Example:**
```bash
curl -X POST http://localhost:8080/domain/check \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "example.com",
    "blacklists": ["dbl.spamhaus.org", "multi.uribl.com"]
  }'
```

## Security Features

### Input Validation
- IP/domain format validation
- Blacklist DNS syntax validation:
  - Must be valid DNS names (e.g., `bl.example.org`)
  - No invalid characters
  - No consecutive dots
  - Cannot start/end with `.` or `-`
  - Cannot be empty or whitespace

### Resource Protection
- Maximum 20 blacklists per request (configurable via `maxCustomBlacklists` in config.toml)
- JSON decoder rejects unknown fields
- Proper HTTP status codes (400 for validation errors)

## Implementation Details

### New Files
- `curl-post-examples.sh` - Script with usage examples and test cases

### Modified Files
- `main.go`:
  - Added `CheckIpRequest` and `CheckDomainRequest` structs
  - Added `PostCheckIp()` and `PostCheckDomain()` handlers
  - Added `logRequest()` helper function (DRY principle)
  - Added `MaxCustomBlacklists` to `Config` struct
  - Registered new routes in router

- `functions.go`:
  - Added `validateBlacklists()` for comprehensive input validation
  - Refactored `checkBlacklistIP()` to use new `checkBlacklistIPWithCustomList()`
  - Refactored `checkBlacklistDomain()` to use new `checkBlacklistDomainWithCustomList()`
  - Modified `checkIPDNS()` and `checkDomainDNS()` to use pointer for blacklisted flag (thread-safe)
  - Added support for loading `maxCustomBlacklists` from config

- `config.toml`:
  - Added `maxCustomBlacklists = 20` parameter

- `README.md`:
  - Documented new POST endpoints with examples
  - Added validation rules and error response examples
  - Added security considerations section

- `ARCHITECTURE.md`:
  - Added "Custom Blacklist Endpoints (POST)" section
  - Updated architecture diagram with POST routes
  - Documented validation and security features

## Backward Compatibility

✅ All existing GET endpoints remain unchanged
✅ Default behavior preserved (uses config.toml blacklists)
✅ Existing tests pass without modification
✅ No breaking changes to API responses

## Testing

Run the example script to test all scenarios:
```bash
./curl-post-examples.sh
```

Test cases include:
- Valid requests with custom blacklists
- Invalid IP/domain formats
- Exceeding maximum blacklist limit
- Invalid blacklist DNS syntax
- Empty blacklist arrays
- Malformed JSON

## Configuration

Add to `config.toml` to adjust the maximum limit:
```toml
maxCustomBlacklists = 20  # Adjust as needed
```

## Notes

- POST endpoints do NOT use Redis caching (always fresh results)
- Custom blacklist queries use the same concurrent DNS resolution
- All validation errors return HTTP 400 with descriptive messages
- Compatible with all existing monitoring and logging

## Enhanced Logging (v1.1)

### New Log Fields

Added two new fields to JSON logs for improved debugging and monitoring:

- **`HTTPMethod`**: The HTTP method used (GET, POST, PUT, DELETE, etc.)
- **`RequestBody`**: The full JSON request body for POST requests

### Example Logs

**GET request:**
```json
{
   "CurrentTime": "2026-01-15T22:45:00Z",
   "HTTPMethod": "GET",
   "Method": "/ip",
   "Param": "1.2.3.4",
   "RequestBody": "",
   "MemoryAlloc": 2048,
   "TimeTaken": 0.245,
   "ClientIP": "192.168.1.100:54321"
}
```

**POST request:**
```json
{
   "CurrentTime": "2026-01-15T22:45:30Z",
   "HTTPMethod": "POST",
   "Method": "/ip/check",
   "Param": "1.2.3.4",
   "RequestBody": "{\"ip\":\"1.2.3.4\",\"blacklists\":[\"zen.spamhaus.org\"]}",
   "MemoryAlloc": 2560,
   "TimeTaken": 0.312
}
```

### Benefits

- **Debugging**: See exact payloads that caused errors
- **Auditing**: Track what data was sent to the API
- **Monitoring**: Filter logs by HTTP method
- **Analysis**: Parse request bodies with jq

### Usage Examples

```bash
# Filter only POST requests
make run | jq 'select(.HTTPMethod == "POST")'

# Show failed requests with their payloads
make run | jq 'select(.Errors | length > 0) | {HTTPMethod, Method, RequestBody, Errors}'

# Extract all POST request bodies
make run | jq 'select(.HTTPMethod == "POST") | .RequestBody'
```

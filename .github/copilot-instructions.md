# Multicheck - AI Coding Agent Instructions

## Coding Standards & Documentation
- **All code comments, documentation, and commit messages MUST be in English**
- **All changes MUST be documented** in:
  - Code comments (explain why, not just what)
  - README.md (for user-facing features)
  - CHANGELOG files (when present)
  - Git commit messages (clear and descriptive)
- Follow Go best practices and idiomatic patterns
- Use structured error handling with descriptive messages
- Every input must be validated and sanitized to prevent injection attacks and resource exhaustion
- Use context-aware logging with structured JSON output for easy parsing
- Write unit tests for all new functionality and edge cases
- Security First: Always consider security implications of changes, especially for input handling and external dependencies

## Project Overview
Multicheck is a Go-based REST API service that checks domain and IP reputation against DNS-based blacklists (DNSBL). It uses Redis for caching results and provides concurrent DNS lookups for fast blacklist checking.

**The project includes a modern web frontend** built with SvelteKit, TypeScript, and Tailwind CSS located in the `frontend/` directory.

## Architecture

### Core Components
- **[main.go](../main.go)** - HTTP server with Gorilla Mux router, defines all API endpoints and request handlers
- **[functions.go](../functions.go)** - DNS blacklist checking logic with concurrent goroutines and WaitGroups
- **[db.go](../db.go)** - Redis connection pool management using redigo
- **[config.toml](../config.toml)** - Blacklist configuration, cache TTL, nameservers, listen port: the configuration file

### Data Flow
1. Client requests GET to `/ip/{ip}`, `/domain/{domain}`, or POST to `/ip/check` or `/domain/check`
2. Handler checks Redis cache first (`getRedisKey`):
   - GET endpoints: use simple key (IP or domain)
   - POST endpoints: use composite key `post:ip:<ip>:<hash>` or `post:domain:<domain>:<hash>` where hash is SHA256 of sorted blacklist array
3. On cache miss: concurrent DNS lookups against all blacklists using goroutines (`checkBlacklistIP`/`checkBlacklistDomain`)
4. Results cached in Redis with TTL (`setRedisKey`) for both GET and POST endpoints
5. JSON response with blacklist status, timing, and cache hit indicator

### Version Information
- Version is stored in `main.go` as package-level variable `version`
- Default value: `"1.0.0"` (change this for releases)
- Can be overridden at build time: `go build -ldflags "-X main.version=x.y.z"`
- Displayed in `/health` endpoint response

## Key Patterns

### Concurrent DNS Lookups
All blacklist checks use `sync.WaitGroup` with goroutines for parallel DNS queries. Each goroutine:
- Performs DNS lookup via custom resolver with random nameserver selection
- Filters out `127.0.0.1` and `127.255.255.255` from results (DNSBL-specific false positives)
- Updates shared map `blacklistsActive` when blacklisted (protected by `sync.Mutex`)
- Reports errors via buffered channel `errorCh`

Functions:
- `checkIPDNS()` - Goroutine for IP blacklist lookup
- `checkDomainDNS()` - Goroutine for domain blacklist lookup
- `checkBlacklistIPWithCustomList()` - Main IP check with optional custom resolver
- `checkBlacklistDomainWithCustomList()` - Main domain check with optional custom resolver

##**Cache key**: 
  - **GET endpoints**: IP address or domain name (as-is)
  - **POST endpoints**: `post:ip:<ip>:<hash>` or `post:domain:<domain>:<hash>` where hash is 16-char truncated SHA256 of sorted blacklist array
- **TTL**: `redisCacheTTL` seconds from config (default 300s)
- **What's cached**: Complete response including `BlackListed`, `BlackList`, `ValidIP`/`ValidDomain`, `TimeTaken`, `Errors`
- **Cache indicator**: Set `Cached: true` in response when served from Redis
- **Cache independence**: POST endpoints cache based on blacklist array only (nameservers don't affect cache key since DNS results should be consistent)

### Caching Levels
1. **Client-side**: `cacheControlMaxAge` (default 3600s) - HTTP header tells clients how long to cache
2. **Server-side**: `redisCacheTTL` (default 300s) - How long results stay in Redis before DNS re-checker()` when POST endpoints specify custom nameservers

Benefits: Avoids system DNS limitations, provides redundancy, allows per-request nameserver selection.

### Redis Caching Strategy
- Cache key: IP address or domain name (as-is)
- TTL: `redisCacheTTL` seconds from config (default 300s)

**Key configuration parameters:**
- `ipBlacklist` / `domainBlacklist` - Space-separated lists of DNSHTTPMethod`, `Method`, `Param`, `RequestBody` (for POST), `MemoryAlloc`, `NumGC`, `TimeTaken`, `Cached`, `ClientIP`, `Redis` status, `RedisConnections`. Parse with `jq` for debugging.

Use `logRequest()` helper function for consistent logging across all handlers
- `redisCacheTTL` - Server-side cache expiration (seconds)
- `cacheControlMaxAge` - Client-side cache hint via HTTP header (seconds)
- `maxCustomBlacklists` - Maximum blacklists allowed in POST requests (default 20)
- `maxCustomNameservers` - Maximum nameservers allowed in POST requests (default 3)
- `nameServers` - Space-separated list of DNS resolver IPs
- `listenPort` - HTTP server port (default ":8080")
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
make test     # Runs go test with colored output and icons (verbose)
make test-quiet # Runs tests with summary only (minimal output, no JSON logs)
```

### Test Output
Tests now include visual enhancements for better readability:
- **Icons**: ▶ (running), ✓ (pass), ✗ (fail)
- **Colors**: Blue for running, green for pass, red for fail
- **Two modes**:
  - `make test`: Full verbose output with JSON logs and colors
  - `make test-quiet`: Clean summary showing only test names and results
- Output uses ANSI escape codes and works in most modern terminals
contains comprehensive tests:
- `TestHealthCheckHandler` - Verifies /health endpoint and Redis connectivity
- `TestDomainBlacklist` - Tests GET /domain with test.uribl.com (expects 127.0.0.14 from multi.uribl.com)
- `TestIPBlacklist` - Tests GET /ip with 2.0.0.127 (expects 127.0.0.11 from zen.spamhaus.org)
- `TestPostCheckDomain` - Tests POST /domain/che, uptime, Go version, and software version
- `GET /ip/{ip}` - Check IP against default blacklists (validates with `net.ParseIP`)
- `GET /domain/{domain}` - Check domain against default blacklists (validates with `validator.IsValidDomain`)
- `POST /ip/check` - Check IP against custom blacklists with optional custom nameservers
- `POST /domain/check` - Check domain against custom blacklists with optional custom nameservers
- `GET /clear-cache/{key}` - Delete specific cache entry by key

### POST Endpoints Request Body
```json
{
  "ip": "1.2.3.4",           // or "domain": "example.com"
  "blacklists": ["bl1.org"],  // Required: 1-20 blacklists (configurable)
  "nameservers": ["8.8.8.8"]  // Optional: 1-3 nameservers (configurable), must be valid IPs
}
```

**VaDNSBL Response Codes
- DNSBL servers return IPs in `127.0.0.x` range to indicate positive matches
- Different codes indicate different types of listings (e.g., 127.0.0.2 = spam, 127.0.0.11 = XBL)
- `127.0.0.1` and `127.255.255.255` are filtered as they're sometimes false positives
- Always preserve other `127.x.x.x` responses - they're valid blacklist indicators

### Validation:**
- `validateBlacklists()` - Checks format, count limit, DNS name validity
- `validateNameservers()` - Checks valid IP format, count limit
- Both return `(valid bool, errorMsg string)` for HTTP 400 response (degrade gracefully)
- Invalid input sets `Status: false` and returns HTTP 400
- DNS lookup errors filtered (ignore "no such host" as expected for non-blacklisted)
- POST endpoints: validate input before processing, return specific error messages

### Request Body Parsing (POST endpoints)
1. Read body with `io.ReadAll()` for logging
2. Use `json.NewDecoder(bytes.NewReader(bodyBytes))` for parsing
3. Enable `decoder.DisallowUnknownFields()` to catch typos
4. Log request body in JSON logs for debugging
- Verify specific DNSBL response codes (not just presence in blacklist)
- Fail if expected IP code doesn't match actual response

### Docker Deployment
```bash
docker-compose up    # Starts multicheck + redis containers
```
- Service exposed on port 8080
- Depends on Redis at `redis:6379` (container network)
- Dockerfile uses multi-stage build: Go builder → Alpine runtime

### Concurrency Safety
- Use `sync.Mutex` to protect shared map writes in goroutines
- Use buffered channels for error collection (size = number of goroutines)
- Always use `defer wg.Done()` at start of goroutine functions
- Close error channels after `wg.Wait()` completes

## When Making Changes
1. **Update tests** - Add/modify tests in main_test.go for new functionality
2. **Update README.md** - Document new endpoints, parameters, configuration options
3. **Update this file** - Keep copilot-instructions.md synchronized with architecture changes
4. **Add comments** - Explain complex logic, especially concurrency patterns and DNSBL specifics
5. **Version bump** - Consider updating `version` variable in main.go for releases

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

## Frontend Architecture

### Directory Structure
The `frontend/` directory contains a SvelteKit 5 application with the following structure:
- **src/lib/** - Shared code and components
  - `api.ts` - API client functions with fetch wrappers
  - `types.ts` - TypeScript interfaces matching Go API responses
  - `validators.ts` - Zod schemas for input validation
  - **components/** - Svelte components
    - `CheckForm.svelte` - Main form with IP/domain input, validation, and advanced options
    - `ResultsCard.svelte` - Displays check results with blacklist details
    - `HistoryPanel.svelte` - Sidebar with recent checks (max 20 items in state)
- **src/routes/** - SvelteKit pages
  - `+layout.svelte` - App shell with header, navigation, dark mode toggle
  - `+page.svelte` - Home page with CheckForm and HistoryPanel
  - `health/+page.svelte` - Health dashboard with auto-refresh (5s interval)
- **Configuration files:**
  - `vite.config.ts` - Vite dev server with API proxy (`/api/*` → `http://localhost:8080`)
  - `tailwind.config.js` - Tailwind with custom color tokens for dark/light themes
  - `svelte.config.js` - SvelteKit adapter configuration

### Frontend Tech Stack
- **SvelteKit 5** - Framework with file-based routing and SSR support
- **Svelte 5 Runes** - Modern reactive syntax (`$state`, `$effect`, `$props`)
- **TypeScript** - Strict mode enabled for type safety
- **Tailwind CSS** - Utility-first CSS with dark mode support (`class` strategy)
- **Lucide Svelte** - Icon library (open source, MIT licensed)
- **Svelte Sonner** - Toast notifications (success/error feedback)
- **Zod** - Runtime validation for user inputs

### Frontend Key Patterns

#### State Management
- Component-level state using Svelte 5 runes (`$state`, `$effect`)
- No global state management needed (small app)
- History items stored in parent component state (max 20 items)
- Dark mode preference stored in localStorage

#### API Communication
- All API calls go through functions in `src/lib/api.ts`
- Development: Vite proxy routes `/api/*` to backend (avoids CORS)
- Production: Configure proxy or serve frontend from same domain as API
- Error handling with try/catch and toast notifications

#### Form Validation
- Real-time validation using Zod schemas
- Visual feedback with error messages and border colors
- Validates IP format (4 octets, 0-255 range) and domain format (DNS standard)
- Custom blacklist/nameserver validation (count limits, format checks)

#### Component Communication
- Props passed down: `{selectedItem}` and callbacks like `onCheckComplete()`
- Parent component manages history state and selection
- Child components emit events via callback props

### Frontend Development Workflow

```bash
cd frontend
npm install              # First time: install dependencies
npm run dev              # Start dev server on http://localhost:5173
npm run build            # Production build
npm run preview          # Test production build
npm run check            # TypeScript type checking
npm run format           # Format code with Prettier
```

### Frontend-Backend Integration
- Frontend expects backend API on `http://localhost:8080`
- Vite dev server proxies `/api/*` requests to backend
- API responses match TypeScript interfaces in `src/lib/types.ts`
- Frontend handles both GET (default blacklists) and POST (custom blacklists) endpoints

### When Working on Frontend
- Always update TypeScript types if API response structure changes
- Follow Svelte 5 runes syntax (avoid legacy `let` with `$:` reactivity)
- Use Tailwind utility classes (avoid custom CSS unless necessary)
- Test dark mode for all new UI components
- Keep components small and focused (CheckForm, ResultsCard, HistoryPanel pattern)
- Add comments for complex reactive logic using `$effect()`
- Validate all user inputs before sending to API

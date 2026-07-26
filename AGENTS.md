# Multicheck - AI Coding Agent Instructions

## Coding Standards & Documentation

- **All code comments, documentation, and commit messages MUST be in English**
- **All changes MUST be documented** in:
  - Code comments (explain why, not just what)
  - README.md: update when any API endpoint, request/response field, configuration parameter, or CLI behavior changes
  - CHANGELOG files (when present)
  - ARCHITECTURE.md: update when adding/removing components, changing data flow between components, or modifying concurrency patterns
- Use consistent formatting (gofmt for Go, Prettier for JS/TS)
- Write clear, descriptive commit messages following Conventional Commits:
  - Format: `<type>(<scope>): <description>`
  - Example: `feat(api): add POST /ip/check endpoint for custom blacklists`
- Types and interfaces MUST have clear, descriptive names
- Functions should be no longer than 50 lines excluding comments, perform exactly one logical operation, and have names that describe that operation as a verb phrase (e.g., reverseIP, validateBlacklists)
- Use idiomatic patterns for the language (e.g., error handling in Go)
- Ensure proper input validation and error handling throughout
  - Git commit messages (clear and descriptive)
- Follow Go best practices and idiomatic patterns
- Use structured error handling with descriptive messages
- Every input must be validated and sanitized to prevent injection attacks and resource exhaustion
- Use context-aware logging with structured JSON output for easy parsing
- Write unit tests for all new functionality and edge cases
- Security First: Always consider security implications of changes, especially for input handling and external dependencies
- Use only stable and/or LTS versions when available

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
- Default value: `"1.6.0"` (change this for releases)
- Can be overridden at build time: `go build -ldflags "-X main.version=x.y.z"`
- Displayed in `/health` endpoint response

## Key Patterns

### Concurrent DNS Lookups

All blacklist checks use `sync.WaitGroup` with goroutines for parallel DNS queries. Each goroutine:

- Performs DNS lookup via custom resolver with random nameserver selection
- Filters the DNSBL sentinel replies out of the result via `removeIPFromSlice()` (see "DNSBL Response Codes"), and turns a refusal code into an error rather than a listing
- Updates the shared map `blacklistsActive` **and** the shared `blacklisted` flag when blacklisted — both writes happen under the same `sync.Mutex`, since several goroutines reach that branch concurrently
- Reports errors via buffered channel `errorCh`

Functions:

- `checkIPDNS()` - Goroutine for IP blacklist lookup
- `checkDomainDNS()` - Goroutine for domain blacklist lookup
- `checkBlacklistIPWithCustomList()` - Main IP check with optional custom resolver
- `checkBlacklistDomainWithCustomList()` - Main domain check with optional custom resolver

### DNS Resolver

`createCustomResolver()` builds every resolver, including the global one in
`main()`. Two rules for its `Dial` closure:

- Forward the `network` argument to `DialContext`. The Go resolver retries over TCP when a UDP answer comes back truncated, and hardcoding `"udp"` silently defeats that retry.
- It returns an `error` rather than calling `log.Fatal`: it is reachable from a request path, where killing the process is never an acceptable failure mode.

## Caching

| Endpoint | Cache Key Format | TTL Source | Fields Cached |
|----------|-----------------|------------|---------------|
| GET /ip/{ip} | IP address (as-is) | `redisCacheTTL` | `BlackListed`, `BlackList`, `ValidIP`, `TimeTaken`, `Errors` |
| GET /domain/{domain} | Domain name (as-is) | `redisCacheTTL` | `BlackListed`, `BlackList`, `ValidDomain`, `TimeTaken`, `Errors` |
| POST /ip/check | `post:ip:<ip>:<hash>` (hash = 16-char truncated SHA256 of sorted blacklist array) | `redisCacheTTL` | `BlackListed`, `BlackList`, `ValidIP`, `TimeTaken`, `Errors` |
| POST /domain/check | `post:domain:<domain>:<hash>` (hash = 16-char truncated SHA256 of sorted blacklist array) | `redisCacheTTL` | `BlackListed`, `BlackList`, `ValidDomain`, `TimeTaken`, `Errors` |

- **Cache indicator**: Set `Cached: true` in response when served from Redis
- **Cache independence**: POST endpoints cache based on blacklist array only; nameservers do not affect the cache key
- **Never cache a partial result**: when the DNS fan-out returns any error (timeout, resolver failure), skip the cache write. Keep those errors in a variable separate from the response-wide error list — the latter also holds Redis errors, which must not block caching
- **Single write command**: `setRedisKey()` uses `SET key value EX ttl`. Do not split it back into `SET` + `EXPIRE`: the pipelined version could leave a key without a TTL and swallowed the `SET` error
- **Redis down**: skip both the cache read and the cache write instead of letting them fail
- **Client-side**: `cacheControlMaxAge` (default 3600s) — HTTP `Cache-Control` header sent to clients
- **Server-side**: `redisCacheTTL` (default 300s) — how long results stay in Redis before a live DNS lookup is performed

### Redis Caching Strategy

**Key configuration parameters:**

- `ipBlacklist` / `domainBlacklist` - Space-separated lists of DNSBL hostnames to check against (e.g., `zen.spamhaus.org bl.spamcop.net`)

Use `logRequest()` helper function for consistent logging across all handlers

- `redisCacheTTL` - Server-side cache expiration (seconds)
- `cacheControlMaxAge` - Client-side cache hint via HTTP header (seconds)
- `maxCustomBlacklists` - Maximum blacklists allowed in POST requests (default 20)
- `maxCustomNameservers` - Maximum nameservers allowed in POST requests (default 3)
- `nameServers` - Space-separated list of DNS resolver IPs. Empty (the shipped default) selects the system resolver, which is what lets a fresh clone work anywhere
- `trustProxyHeaders` - Whether `X-Forwarded-For` / `X-Real-IP` may be trusted for the logged client IP (default `false`)
- `listenPort` - HTTP server port (default ":8080")
- `redisMaxIdle` / `redisMaxActive` - Connection pool sizing (defaults 8 / 64)
- `redisConnTimeout` - Connect/read/write timeout for Redis in seconds (default 2)
- `redisHealthCheckInterval` / `memStatsInterval` - Background monitor intervals in seconds (defaults 5 / 10)

Every parameter except the blacklists is optional: `applyConfigDefaults()` fills
in the tuning keys so a `config.toml` written before they existed stays valid.

**`applyConfigDefaults()` must cover every key, not just recent additions.** A
missing key reads back as zero, and several of those zeros disable the service
rather than degrading it: `maxStringLength = 0` rejects every IP and domain as "too
long", `dnsQueryTimeout = 0` hands an already-expired context to every lookup,
`maxRequestBodySize = 0` rejects every POST body, `redisCacheTTL = 0` makes
`SET ... EX` fail so nothing is ever cached, and `listenPort = ""` silently binds
`:80`. When adding a config key, add its default in the same commit.

`validateConfig()` runs at startup and refuses to start on a configuration that
cannot produce correct results (no blacklists at all, or a `nameServers` entry that
is not an IP). Failing loudly beats serving nonsense from a broken resolver.
- Cached response includes all fields: `BlackListed`, `BlackList`, `ValidIP`/`ValidDomain`, `TimeTaken`
- Set `Cached: true` in response when served from Redis

### Configuration Loading

Uses Viper to read [config.toml](../config.toml). Supports `GSS_CONFIG_PATH` env var to override config location. All multiline strings in TOML are space-separated lists.

### Background Monitors

`startBackgroundMonitors()` (in functions.go, guarded by `sync.Once`) starts two
tickers that cache process state for the handlers:

- **Redis status** — a `PING` every `redisHealthCheckInterval` seconds stores availability, active connections and the last error into atomics. Handlers read them through `redisStatus()`; **never add a `pingRedis()` call to a request path.** `/health` is the sole exception: it pings live because reporting the current state is its purpose.
- **Memory** — `runtime.ReadMemStats` every `memStatsInterval` seconds. This call stops the world, so it must never run per request; `MemUsage()` only reads the cached sample.

Both are primed synchronously before the goroutines start, so the first request
sees real values. Tests must call `startBackgroundMonitors()` in their setup.

### Structured JSON Logging

Every request logs to stdout in JSON format with: `CurrentTime`, `Method`, `Param`, `MemoryAlloc`, `NumGC`, `TimeTaken`, `Cached`, `ClientIP`, `Redis` status, `RedisConnections`. Parse with `jq` for debugging. The `MemoryAlloc`, `NumGC`, `Redis` and `RedisConnections` fields come from the background monitors and may be up to one interval old.

**One entry per line: use `json.Marshal`, never `json.MarshalIndent`.** Log
collectors (Loki, Fluent Bit, CloudWatch) parse newline-delimited JSON, and the
indented form split every entry into a dozen unrelated lines.

**Never log a full request body.** `logRequest` passes it through
`truncateForLog()` (`maxLoggedBodyBytes`), and the rejected-request call sites pass
`""`: the body is up to `maxRequestBodySize` of unsanitised client input, so logging
it whole enables log injection and stores arbitrary user data. The specific
rejection reason is already in `errors[]`.

## Development Workflow

### Build & Run

```bash
make build    # Compiles to bin/multicheck with stripped binary
make run      # Runs binary piping output through jq
make test     # Runs the unit tests with colored output and icons (verbose)
make test-quiet # Runs the unit tests with summary only (minimal output, no JSON logs)
make test-race  # Runs the unit tests under the race detector
make test-integration # Runs the integration tests (needs Redis and live DNS)
```

### Test Output

Tests now include visual enhancements for better readability:

- **Icons**: ▶ (running), ✓ (pass), ✗ (fail)
- **Colors**: Blue for running, green for pass, red for fail
- **Two modes**:
  - `make test`: Full verbose output with JSON logs and colors
  - `make test-quiet`: Clean summary showing only test names and results
- Output uses ANSI escape codes and works in most modern terminals
[main_integration_test.go](../main_integration_test.go) contains the endpoint tests (build tag `integration`):

| Test Function | Endpoint | Input | Expected Assertion |
|--------------|----------|-------|-------------------|
| `TestHealthCheckHandler` | GET /health | — | HTTP 200, Redis connected, uptime and version fields present |
| `TestDomainBlacklist` | GET /domain/{domain} | `test.uribl.com` | Response contains 127.0.0.14 from `multi.uribl.com` |
| `TestIPBlacklist` | GET /ip/{ip} | `2.0.0.127` | Response contains 127.0.0.11 from `zen.spamhaus.org` |
| `TestPostCheckDomain` | POST /domain/check | custom blacklist body | Domain listed on target blacklist with correct DNSBL response code |

### POST Endpoints Request Body

```json
{
  "ip": "1.2.3.4",           // or "domain": "example.com"
  "blacklists": ["bl1.org"],  // Required: 1-20 blacklists (configurable)
  "nameservers": ["8.8.8.8"]  // Optional: 1-3 nameservers (configurable), must be valid IPs
}
```

### DNSBL Response Codes

- DNSBL servers return IPs in `127.0.0.x` range to indicate positive matches
- Different codes indicate different types of listings (e.g., 127.0.0.2 = spam, 127.0.0.11 = XBL)
- `removeIPFromSlice()` filters the sentinel replies listed in `dnsblSentinelCodes` and returns them separately from the genuine listings
- `127.0.0.1` and `127.255.255.255` are filtered as they're sometimes false positives
- `127.255.255.252-254` mean **the query was refused** (public resolver, or over quota) — not "listed". `isQueryRefused()` turns them into an entry in `errorList` so the result is reported as unknown instead of as a confident false positive
- Always preserve other `127.x.x.x` responses - they're valid blacklist indicators

### Validation:**

- `validateBlacklists()` - Checks format, count limit, DNS name validity
- `validateNameservers()` - Checks valid IP format, count limit
- Both return `(valid bool, errorMsg string)` for HTTP 400 response (degrade gracefully)
- Invalid input sets `Status: false` and returns HTTP 400
- DNS lookup errors filtered (ignore "no such host" as expected for non-blacklisted)
- POST endpoints: validate input before processing, return specific error messages
- Validation is all-or-nothing for POST requests: if any blacklist entry or nameserver entry is invalid, reject the entire request with HTTP 400 and a specific error message identifying the first invalid entry. Do not process partial lists.

### Request Body Parsing (POST endpoints)

1. Read body with `io.ReadAll()` for logging
2. Use `json.NewDecoder(bytes.NewReader(bodyBytes))` for parsing
3. Enable `decoder.DisallowUnknownFields()` to catch typos
4. Log request body in JSON logs for debugging

- Verify specific DNSBL response codes (not just presence in blacklist)
- Fail if expected IP code doesn't match actual response

### Continuous Integration

`.github/workflows/docker-publish.yml` gates image publishing behind two parallel
verification jobs: `verify-backend` (gofmt, `go vet` with and without the
`integration` tag, `go test -race`) and `verify-frontend` (`npm ci`, `check`,
`lint`, `build`). The `build` job carries `needs: [verify-backend, verify-frontend]`
— **do not remove it**: images were previously published from code no automated
check had ever seen.

Integration tests are deliberately excluded from CI: they need a Redis instance and
live DNS access to third-party DNSBL servers whose answers the project does not
control.

### Docker Deployment

```bash
docker compose up    # Starts multicheck + valkey containers
```

- Service exposed on port 8080
- The cache is **Valkey** (Redis-compatible), not Redis
- All services use `network_mode: host`, so the backend reaches the cache at `127.0.0.1:6379` — there is no container network and no `redis:6379` hostname
- Dockerfile uses multi-stage build: Go builder → Alpine runtime

### Concurrency Safety

- Use `sync.Mutex` to protect shared map writes in goroutines
- Use buffered channels for error collection (size = number of goroutines)
- Always use `defer wg.Done()` at start of goroutine functions
- Close error channels after `wg.Wait()` completes

## When Making Changes

1. **Update tests** - Add unit tests for new logic; extend the integration suite only when the behaviour genuinely needs Redis or live DNS
2. **Update README.md** - Document new endpoints, parameters, configuration options
3. **Update this file** - Keep AGENTS.md synchronized with architecture changes
4. **Add comments** - Explain complex logic, especially concurrency patterns and DNSBL specifics
5. **Version bump** - Update the `version` variable in main.go when changes affect any public API endpoint, response schema, or configuration format. Do not update it for internal refactors or test-only changes.

### Testing

The suite is split in two, and the split must be preserved:

- **Unit tests** — [functions_test.go](../functions_test.go), [handlers_test.go](../handlers_test.go), with the shared environment in [testsupport_test.go](../testsupport_test.go). `TestMain` builds `configuration` in memory, so these need **no `config.toml`, no Redis server and no DNS access**. They cover the pure logic and every request path rejected before DNS or Redis work begins. `make test` runs only these, which is what makes them usable in CI.
- **Integration tests** — [main_integration_test.go](../main_integration_test.go), behind the `//go:build integration` tag. These read the real `config.toml`, need a reachable Redis instance and query third-party DNSBL servers, asserting on sentinel codes those servers control. `setupTestWithResolver(t)` calls `t.Skip` when Redis is unreachable, so they never fail for environmental reasons. Run with `make test-integration`.

**Put a new test in the unit file unless it genuinely cannot work without Redis or live DNS.** A test added to the integration file is a test CI will never run.

## API Endpoints

- `GET /` - Lists all endpoints and configuration
- `GET /health` - Health check with Redis status and uptime. Always answers `200`: the service is designed to run without Redis
- `GET /ip/{ip}` - Check IP against blacklists (validates with `isIPv4`, **not** `net.ParseIP`: IPv6 has no valid DNSBL reversal)
- `GET /domain/{domain}` - Check domain against blacklists (validates with `validator.IsValidDomain`)
- `POST /ip/check` - Check IP against a caller-supplied blacklist array
- `POST /domain/check` - Check domain against a caller-supplied blacklist array
- `DELETE /clear-cache/{key}` - Delete specific cache entry by key. **Not GET**: the operation is destructive and over GET it is reachable by browser prefetch and cross-site requests. The key must be one this service could have written (`isOwnCacheKey`), otherwise `400`

## Important Implementation Notes

### Adding New Blacklists

Update `ipBlacklist` or `domainBlacklist` arrays in [config.toml](../config.toml). Space-separate entries. No code changes needed.

### Response Headers

All endpoints set `Content-Type: application/json`.

`Cache-Control: max-age=<cacheControlMaxAge>` is set via `setCacheControl(w)` **only
on the success path** of `GET /` , `GET /ip/{ip}` and `GET /domain/{domain}`. Never
set it before the validation branches: doing so declared `400` responses cacheable,
so a client that once sent a malformed request kept replaying the error from its own
cache for a whole hour. POST responses carry no `Cache-Control` at all — the header
is meaningless for a non-idempotent request.

### Error Handling

- Redis failures append to `Errors[]` array but don't block request
- Build the message with `redisErrorMessage(reply, err)`: a PING can return an unexpected reply with a `nil` error, and calling `err.Error()` on it would panic the server
- Accumulate errors with `append`, never with `=`: assigning the result of a check function over the error slice discards everything collected earlier (Redis status, corrupted cache entries)
- If Redis is unavailable at startup, log a warning and continue — the service must operate without caching (all requests will perform live DNS lookups). Do not exit on Redis connection failure.
- If Redis becomes unavailable mid-request, skip cache read/write, set `Cached: false`, and append a descriptive error to `Errors[]`.
- Invalid input sets `Status: false` and returns early
- DNS lookup errors filtered (ignore "no such host" as expected for non-blacklisted)

### Global State

Package-level variables: `configuration`, `c` (Redis pool), `resolver`, `nameservers`, `startTime`, plus the atomics fed by the background monitors. All are read-only after initialization; the atomics are the only mutable ones and are written exclusively by the monitor goroutines.

**Never add a package-level variable that a handler writes.** Response structs and per-request values (`endpoints`, `uptime`, …) must be locals: concurrent requests would otherwise race on them. Run `go test -race` after touching anything shared.

### IP Reversal for DNSBL

IPs must be reversed for DNSBL queries: `1.2.3.4` becomes `4.3.2.1.dnsbl.example.org`. See `reverseIP()` function.

**IPv4 only.** Validate with `isIPv4()`, never with a bare `net.ParseIP() != nil`:
`net.ParseIP` accepts IPv6, which has no valid reversal into the configured
(IPv4-only) DNSBL zones. Before this was enforced, an IPv6 address came back
`ValidIP: true` with a confident "not blacklisted" derived from a nonsensical query.
`reverseIP()` returns an empty string for non-IPv4 input as a second line of defence.

### Cache Clearing Security

The `/clear-cache/{key}` endpoint has no authentication.

- It is registered as **`DELETE`**, not `GET`. A destructive operation over GET is reachable by browser prefetch and by a cross-site request. Do not move it back.
- `isOwnCacheKey()` restricts deletion to keys this service could have written (an IPv4 address, a valid domain, or a `post:ip:` / `post:domain:` prefix). Without it, any client could delete arbitrary keys from a Redis database shared with other applications.
- Do not extend it to support wildcard or bulk deletion. Consider adding rate limiting or restricting to localhost if deploying publicly.

### Graceful Shutdown

`main()` serves in a goroutine and waits on `signal.NotifyContext` for `SIGINT`/`SIGTERM`, then calls `srv.Shutdown` with a bounded `shutdownGracePeriod` and closes the Redis pool. Do not go back to a bare blocking `srv.ListenAndServe()`: it truncates in-flight requests on every Docker or Kubernetes stop.

### Client IP

Use `clientIPFrom(r)`, not `r.RemoteAddr`: behind the nginx front end in this repo the latter is always the proxy. It honours `X-Forwarded-For` / `X-Real-IP` only when `trustProxyHeaders` is enabled — trusting them unconditionally lets any client forge its own address.

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
  - `vite.config.ts` - Vite dev server with API proxy (`/api/*` → `http://localhost:8080`) and `@tailwindcss/vite` plugin
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
npm run lint             # Prettier check + ESLint
```

`eslint.config.js` is an ESLint 9 flat config. The two prettier entries must stay
last: they switch off the stylistic rules that would otherwise contradict
`npm run format`. `package.json` and `package-lock.json` are in `.prettierignore`
because npm owns their formatting — without that, every `npm run format` rewrote
them and turned a version bump into a full-file diff.

### Frontend-Backend Integration

- Frontend expects backend API on `http://localhost:8080`
- Vite dev server proxies `/api/*` requests to backend
- API responses match TypeScript interfaces in `src/lib/types.ts`
- Frontend handles both GET (default blacklists) and POST (custom blacklists) endpoints

### When Changing API Response Structure

When modifying any Go response struct that is serialized to JSON:

1. Update the Go struct in main.go
2. Update the TypeScript interface in `src/lib/types.ts`
3. Update the Zod schema in `src/lib/validators.ts` if the field is user-supplied
4. Update README.md API documentation
5. Add or update tests in functions_test.go / handlers_test.go (unit) or main_integration_test.go (needs Redis or live DNS)

### When Working on Frontend

- Always update TypeScript types if API response structure changes
- Follow Svelte 5 runes syntax (avoid legacy `let` with `$:` reactivity)
- Use Tailwind utility classes (avoid custom CSS unless necessary)
- Test dark mode for all new UI components
- Keep components small and focused (CheckForm, ResultsCard, HistoryPanel pattern)
- Add comments for complex reactive logic using `$effect()`
- Validate all user inputs before sending to API

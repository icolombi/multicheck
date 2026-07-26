# Multicheck Architecture

## Overview

Multicheck is a REST API service developed in Go that implements a reputation verification system for domains and IP addresses through DNS blacklist (DNSBL) queries. The architecture is designed to maximize performance through concurrent operations and an intelligent caching system.

## Diagramma dell'Architettura

```txt
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ HTTP Request (GET/POST)
       ▼
┌─────────────────────────────────────────┐
│     Gorilla Mux Router (main.go)        │
│  ┌─────────────────────────────────┐   │
│  │  GET /                          │   │
│  │  GET /health                    │   │
│  │  GET /ip/{ip}                   │   │
│  │  GET /domain/{domain}           │   │
│  │  POST /ip/check                 │   │
│  │  POST /domain/check             │   │
│  │  DELETE /clear-cache/{key}      │   │
│  └─────────────────────────────────┘   │
└──────┬──────────────────────────────────┘
       │
       ├───────────────────┬─────────────────────┐
       ▼                   ▼                     ▼
┌─────────────┐    ┌──────────────┐    ┌─────────────────┐
│   Validator │    │ Redis Cache  │    │  DNS Resolver   │
│             │    │   (db.go)    │    │  (functions.go) │
└─────────────┘    └──────────────┘    └─────────────────┘
                           │                     │
                           │                     │
                           ▼                     ▼
                    ┌─────────────┐    ┌─────────────────┐
                    │    Redis    │    │  DNS Blacklists │
                    │   Server    │    │   (Parallel)    │
                    └─────────────┘    └─────────────────┘
```

## Main Components

### 1. HTTP Server and Routing (`main.go`)

The heart of the service is implemented using the **Gorilla Mux** router, which handles all incoming HTTP requests.

#### Main Handlers

- **`RootHandler()`**: Endpoint `/` that returns documentation of available endpoints and current configuration
- **`HealthCheckHandler()`**: Endpoint `/health` for monitoring service status and Redis connectivity
- **`GetIp()`**: Endpoint `/ip/{ip}` for IP verification against configured blacklists
- **`GetDomain()`**: Endpoint `/domain/{domain}` for domain verification against configured blacklists
- **`PostCheckIp()`**: Endpoint `/ip/check` (POST) for IP verification with custom blacklists
- **`PostCheckDomain()`**: Endpoint `/domain/check` (POST) for domain verification with custom blacklists
- **`DelCache()`**: Endpoint `DELETE /clear-cache/{key}` to manually invalidate a cache entry. Restricted by `isOwnCacheKey()` to keys this service could have written

#### Custom Blacklist Endpoints (POST)

The POST endpoints `/ip/check` and `/domain/check` allow clients to override the default blacklist configuration by providing a custom list of DNS blacklists and optional custom nameservers. This feature includes:

**Security and Validation:**

- **Input validation**: Strict validation of IP/domain format, blacklist DNS syntax, and nameserver IPs
- **Resource protection**:
  - Configurable maximum blacklists (default 20 via `maxCustomBlacklists`)
  - Configurable maximum nameservers (default 3 via `maxCustomNameservers`)
  - Maximum request body size (default 1MB via `maxRequestBodySize`)
  - Maximum string length for domains/IPs (253 chars - DNS standard)
  - DNS query timeout (default 5s via `dnsQueryTimeout`)
  - HTTP read/write timeouts (default 30s via `httpReadTimeout`/`httpWriteTimeout`)
- **DNS format checking**: Validates blacklist names against DNS naming rules:
  - Must contain at least one dot
  - Only alphanumeric, dots, and hyphens allowed
  - Cannot start/end with dot or hyphen
  - No consecutive dots
  - No empty entries or whitespace-only strings
- **Nameserver validation**: Ensures valid IP format for custom nameservers

**Request Structure:**

```json
{
  "ip": "1.2.3.4",
  "blacklists": [
    "zen.spamhaus.org",
    "bl.spamcop.net",
    "cbl.abuseat.org"
  ],
  "nameservers": [
    "8.8.8.8",
    "1.1.1.1"
  ]
}
```

**Note:** The `nameservers` field is optional. If omitted, the default nameservers from `config.toml` are used.

**Error Responses:**

- `400 Bad Request` for invalid input (IP/domain format, blacklist syntax, limit exceeded, invalid nameservers)
- `Status: false` with descriptive error messages in response JSON

**Implementation:**

- Uses `json.Decoder` with `DisallowUnknownFields()` to reject malformed requests
- `validateBlacklists()` function performs comprehensive blacklist validation
- `validateNameservers()` function validates custom nameserver IPs
- `createCustomResolver()` creates a custom DNS resolver when nameservers are provided
- `checkBlacklistIPWithCustomList()` and `checkBlacklistDomainWithCustomList()` accept custom resolver
- **Redis caching enabled** for POST endpoints (as of January 17, 2026)

#### Data Structures

Each endpoint has its own data structure for the response:

```go
type Ip struct {
    IP          string              // Input IP
    ValidIP     bool                // IP validity
    BlackListed bool                // Presence in blacklist
    Status      bool                // Verification status
    BlackList   map[string][]net.IP // Blacklists that detected the IP
    Errors      []string            // Any errors
    TimeTaken   float64             // Execution time
    Cached      bool                // From cache
    CacheKey    string              // Redis cache key (added Jan 20, 2026)
}
```

Similar structure for `Domain`, `Health`, and `ClearCache`. The `CacheKey` field exposes the exact Redis key used for caching, enabling cache invalidation for POST endpoints with custom blacklists.

### 2. DNS Blacklist Checking (`functions.go`)

#### Concurrent Logic

The most critical component for performance is the concurrent DNS lookup system:

```go
func checkBlacklistIP(ipAddress string) (bool, map[string][]net.IP, []string) {
    blackLists := configuration.ipBlacklist
    max := len(blackLists)
    
    var wg sync.WaitGroup
    errorCh := make(chan string, max)
    
    wg.Add(max)
    for _, blacklist := range blackLists {
        go checkIPDNS(&wg, blacklist, reverseIP, blacklistsActive, errorCh)
    }
    
    wg.Wait()
    close(errorCh)
    
    return ip.BlackListed, blacklistsActive, errorList
}
```

**Key features:**

1. **Goroutine per blacklist**: Each blacklist is queried in a separate goroutine
2. **WaitGroup**: `sync.WaitGroup` coordinates the completion of all goroutines
3. **Buffered Channel**: `errorCh` collects errors in a thread-safe manner
4. **Shared Map**: `blacklistsActive` aggregates positive results

#### Custom DNS Resolver

The service uses a custom DNS resolver that:

- Bypasses the system DNS when `nameServers` is configured; falls back to the system resolver (`/etc/resolv.conf`, the cluster DNS under Kubernetes) when it is empty, which is the shipped default
- Randomly selects from a pool of configurable nameservers
- Uses `net.Resolver` with custom dialer
- Supports automatic failover between nameservers

```go
resolver = &net.Resolver{
    PreferGo:     true,
    StrictErrors: true,
    Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
        d := net.Dialer{}
        randomIndex := rand.Intn(len(nameservers))
        nameserver := nameservers[randomIndex]
        return d.DialContext(ctx, "udp", net.JoinHostPort(nameserver, "53"))
    },
}
```

#### IP Reversal for DNSBL

To query IP blacklists, the address must be reversed:

```txt
Input:  1.2.3.4
Output: 4.3.2.1.dnsbl.example.org
```

Implemented through `reverseIP()`:

```go
func reverseIP(ipAddress string) string {
    parts := strings.Split(ipAddress, ".")
    reversedParts := make([]string, len(parts))
    for i := 0; i < len(parts); i++ {
        reversedParts[i] = parts[len(parts)-1-i]
    }
    return strings.Join(reversedParts, ".")
}
```

#### Sentinel Reply Filtering

Not every `127.x.x.x` answer means "listed". `removeIPFromSlice()` separates the
sentinel replies from the genuine listings and returns both, so the caller can tell
the difference between "clean" and "we could not find out":

```go
var dnsblSentinelCodes = []string{
    "127.0.0.1",       // long-standing false positive
    "127.255.255.252", // query refused
    "127.255.255.253", // query refused
    "127.255.255.254", // query refused
    "127.255.255.255", // long-standing false positive
}

func removeIPFromSlice(slice []net.IP) (result []net.IP, sentinels []net.IP)
```

The `127.255.255.252-254` range is how Spamhaus (and others) signal that the query
itself was rejected, typically because the resolver is a large public one or is over
its quota. `isQueryRefused()` turns those into an entry in `errorList`; counting them
as listings produced confident false positives exactly when the service was being
rate limited.

### 3. Redis Caching System (`db.go`)

#### Connection Pooling

Multicheck uses **redigo** with a connection pool to efficiently manage Redis connections:

```go
func redisConnect() *redis.Pool {
    return &redis.Pool{
        MaxIdle:         configuration.RedisMaxIdle,
        MaxActive:       configuration.RedisMaxActive,
        IdleTimeout:     240 * time.Second,
        MaxConnLifetime: 30 * time.Minute,
        Wait:            true,
        Dial: func() (redis.Conn, error) {
            return redis.Dial("tcp", connString,
                redis.DialConnectTimeout(timeout),
                redis.DialReadTimeout(timeout),
                redis.DialWriteTimeout(timeout),
                redis.DialPassword(configuration.RedisPassword),
                redis.DialDatabase(configuration.RedisDatabase),
            )
        },
        TestOnBorrow: /* PING connections idle for more than a minute */,
    }
}
```

**Pool parameters:**

- `MaxIdle` (default 8) - Idle connections kept open
- `MaxActive` (default 64) - Maximum simultaneous connections
- `Wait: true` - Callers block instead of receiving `ErrPoolExhausted`. A failed `Get` is indistinguishable from a cache miss for the handlers, so returning an error would trigger a redundant DNS fan-out exactly under peak load
- `redisConnTimeout` (default 2s) - Applied to connect, read and write, so an unresponsive Redis cannot hold a handler until the HTTP write timeout
- `IdleTimeout` / `MaxConnLifetime` / `TestOnBorrow` - Recycle and validate pooled connections that went stale server-side

Cache writes use a single `SET key value EX ttl` command: the previous
`SET` + `EXPIRE` pipeline discarded the `SET` error and could leave a key without
an expiration.

#### Caching Strategy

**Cache Keys:**

1. **GET endpoints**: Simple keys (IP or domain as-is)
   - Example: `"1.2.3.4"` or `"example.com"`

2. **POST endpoints**: Composite keys with hash (since January 17, 2026)
   - Format: `post:ip:<ip>:<hash>` or `post:domain:<domain>:<hash>`
   - Hash is SHA256 of sorted blacklist array (truncated to 16 characters)
   - Example: `"post:ip:1.2.3.4:a3f5c8d12e9b7f6a"`
   - **Cache independence**: Nameservers don't affect cache keys (DNS results should be consistent)
   - **Order independence**: Blacklist array order doesn't matter (sorted before hashing)

**Cache Value:** Entire serialized JSON response structure

**TTL:** Configurable via `redisCacheTTL` (default: 300 seconds)

**Exposed to API:** The `CacheKey` field in responses contains the exact Redis key used

#### Cache Key Generation Functions

```go
// Generate hash from sorted blacklist array
func generateBlacklistHash(blacklists []string) string

// Build complete cache key for POST endpoints
func buildPostCacheKey(keyType, identifier string, blacklists []string) string
```

#### Redis Operations

```go
// Read
func getRedisKey(key string) (string, error)

// Write with TTL
func setRedisKey(key string, value string) error {
    conn.Send("SET", key, value)
    conn.Send("EXPIRE", key, configuration.RedisCacheTTL)
    conn.Flush()
}

// Delete
func delRedisKey(key string) error

// Get count of keys in database
func getRedisKeysCount() int

// Get active connection count
func getRedisConnections() int
```

### 4. Configuration Management (`config.toml`)

The service uses **Viper** to manage configurations in TOML format:

```toml
ipBlacklist = """
b.barracudacentral.org
bl.spamcop.net
zen.spamhaus.org"""

domainBlacklist = """
multi.uribl.com
dbl.spamhaus.org"""

cacheControlMaxAge = 3600       # Client-side cache hint (seconds)
redisCacheTTL = 300             # Server-side cache TTL (seconds)
maxCustomBlacklists = 20        # Max blacklists in POST requests
maxCustomNameservers = 3        # Max nameservers in POST requests
maxRequestBodySize = 1048576    # Max request body (1MB)
maxStringLength = 253           # Max domain/IP length (DNS standard)
dnsQueryTimeout = 5             # DNS query timeout (seconds)
httpReadTimeout = 30            # HTTP read timeout (seconds)
httpWriteTimeout = 30           # HTTP write timeout (seconds)

nameServers = """
8.8.4.4
8.8.8.8"""

listenPort = ":8080"

# Redis configuration
redisHost = "127.0.0.1"
redisPort = 6379
redisDatabase = 0
redisPassword = ""              # Leave empty if no password
```

**Note**: Multiline strings in TOML are parsed as space-separated lists.

#### Environment Variables

The configuration path can be customized:

```bash
export GSS_CONFIG_PATH=/custom/path
```

Redis credentials can be configured via environment variables or in config.toml.

### 5. Logging System

Each request generates a structured log in JSON format on `stdout`:

```go
type Log struct {
    CurrentTime      time.Time
    HTTPMethod       string   // HTTP method (GET, POST, etc.) - added Jan 2026
    HTTPStatusCode   int      // HTTP response status code - added Jan 2026
    Method           string   // Endpoint path
    Param            string   // IP/domain parameter
    RequestBody      string   // JSON body for POST requests - added Jan 2026
    Errors           []string
    MemoryAlloc      uint64
    NumGC            uint32
    TimeTaken        float64
    Cached           bool
    ClientIP         string
    Redis            bool
    RedisConnections int
}
```

**Included metrics:**

- HTTP method and status code (for better API monitoring)
- Request body for POST endpoints (for debugging and auditing)
- Memory usage (`MemoryAlloc`)
- Garbage collection count (`NumGC`)
- Execution time (`TimeTaken`)
- Cache status (`Cached`)
- Redis status (`Redis`, `RedisConnections`)

**Centralized logging** via `logRequest()` helper function (DRY principle) ensures consistent logging across all handlers.

## Execution Flow

### IP/Domain Verification

```txt
1. Client sends GET request /ip/{ip} or /domain/{domain}
   OR POST request /ip/check or /domain/check with custom blacklists
   ↓
2. Handler validates input (net.ParseIP or validator.IsValidDomain)
   ↓
3. If invalid → Immediate response with Status: false (HTTP 400)
   ↓
4. For POST endpoints: validate blacklists and nameservers
   - validateBlacklists(): check DNS syntax, count limits
   - validateNameservers(): check IP format, count limits
   - If invalid → HTTP 400 with error message
   ↓
5. Build cache key:
   - GET: Simple key (IP or domain)
   - POST: Composite key with hash (post:ip:<ip>:<hash>)
   ↓
6. Check Redis cache (getRedisKey)
   ↓
7a. CACHE HIT → Deserialize JSON and return (Cached: true, includes CacheKey)
   ↓
7b. CACHE MISS → Proceed to step 8
   ↓
8. Create DNS resolver:
   - GET: Use global resolver (from config.toml nameservers)
   - POST with nameservers: Create custom resolver
   - POST without nameservers: Use global resolver
   ↓
9. Launch goroutine for each blacklist
   ↓
10. Each goroutine:
   - Performs DNS lookup via resolver (with timeout)
   - Filters sentinel replies (see dnsblSentinelCodes); a refusal code becomes an error, not a listing
   - Updates shared map if blacklisted (mutex-protected)
   - Sends errors on errorCh
   ↓
11. WaitGroup waits for completion of all goroutines
   ↓
12. Aggregate results (BlackListed, BlackList map)
   ↓
13. Serialize response to JSON (includes CacheKey field)
   ↓
14. Save in Redis with TTL (setRedisKey), only if the check returned no errors
   ↓
15. Return JSON response to client (includes CacheKey for cache invalidation)
   ↓
16. Generate JSON log on stdout (includes HTTPMethod, RequestBody, HTTPStatusCode)
```

### Background Monitors

Two tickers started by `startBackgroundMonitors()` keep expensive calls out of
the request path. Handlers read their results from atomics, via `redisStatus()`
and `MemUsage()`.

```txt
Every redisHealthCheckInterval (default 5s): PING Redis
   → store availability, active connections, last error

Every memStatsInterval (default 10s): runtime.ReadMemStats
   → store allocated memory and GC count
```

`runtime.ReadMemStats` stops the world and the Redis PING costs a round-trip, so
neither may be called per request. The consequence is that the `Redis`,
`RedisConnections`, `MemoryAlloc` and `NumGC` log fields can be up to one
interval old.

### Health Check

```txt
1. Client sends GET /health
   ↓
2. Performs PING on Redis (live: this endpoint reports the current state)
   ↓
3. Gets active connection count
   ↓
4. Gets cached items count (DBSIZE)
   ↓
5. Calculates uptime (time.Since(startTime))
   ↓
6. Collects memory usage and Go version
   ↓
7. Returns JSON with status (includes software version)
```

## Frontend Architecture

### Overview

The frontend is a modern, responsive web application built with **SvelteKit 5** that provides a user-friendly interface for the Multicheck API. Located in the `frontend/` directory, it offers both basic and advanced blacklist checking capabilities.

### Tech Stack

- **SvelteKit 5**: Framework with file-based routing and SSR support
- **Svelte 5 Runes**: Modern reactive syntax (`$state`, `$effect`, `$props`)
- **TypeScript**: Strict mode enabled for type safety
- **Tailwind CSS**: Utility-first CSS with dark mode support (class strategy)
- **Vite**: Dev server with API proxy configuration
- **Lucide Svelte**: Icon library (MIT licensed)
- **Svelte Sonner**: Toast notifications for user feedback
- **Zod**: Runtime validation for user inputs

### Directory Structure

```txt
frontend/
├── src/
│   ├── lib/
│   │   ├── api.ts                 # API client functions
│   │   ├── types.ts               # TypeScript interfaces matching backend
│   │   ├── validators.ts          # Zod schemas for input validation
│   │   └── components/
│   │       ├── CheckForm.svelte   # Main form with IP/domain input
│   │       ├── ResultsCard.svelte # Results display component
│   │       └── HistoryPanel.svelte# Recent checks sidebar
│   ├── routes/
│   │   ├── +layout.svelte         # App layout with header/nav
│   │   ├── +page.svelte           # Home page (check interface)
│   │   └── health/
│   │       └── +page.svelte       # Health dashboard
│   └── app.css                    # Global styles (Tailwind)
├── vite.config.ts                 # Vite dev server + API proxy
├── tailwind.config.js             # Tailwind with custom color tokens
└── package.json                   # Dependencies and scripts
```

### Key Components

#### CheckForm.svelte

Main form component that handles:

- IP/domain input with real-time validation (Zod schemas)
- Tab switching between IP and Domain checks
- Advanced Options (collapsible):
  - Custom blacklists (textarea, one per line, max 20)
  - Custom nameservers (textarea, one per line, max 3)
- Automatic API endpoint selection (GET for defaults, POST for custom)
- Toast notifications for success/error feedback
- Integration with history tracking

**Validation:**

- IP: 4 octets, each 0-255 range
- Domain: Standard DNS format validation
- Real-time error messages with visual feedback

#### ResultsCard.svelte

Displays check results with:

- Status indicator (✓ Clean / ✗ Blacklisted)
- Response time and cache status
- Blacklist detections with IP codes (expandable)
- Error messages if any
- Cache key display (collapsible details)
- Actions:
  - Copy full JSON response to clipboard
  - Clear cache button (uses `CacheKey` from API response)

**Cache Key Integration (since January 20, 2026):**

- Uses exact `CacheKey` field from API response
- Fixes cache deletion for POST requests with custom blacklists
- Displays cache key in collapsible section for transparency

#### HistoryPanel.svelte

Recent checks sidebar (max 20 items) with:

- Chronological list of recent checks
- Click to reload previous check
- Visual status indicators (icons + colors)
- Relative timestamps ("5m ago", "2h ago")
- Clear all history button

### API Integration

#### API Client (`src/lib/api.ts`)

Six main functions wrapping fetch calls:

```typescript
async function checkIp(ip: string): Promise<IpResponse>
async function checkDomain(domain: string): Promise<DomainResponse>
async function postCheckIp(data: CheckIpRequest): Promise<IpResponse>
async function postCheckDomain(data: CheckDomainRequest): Promise<DomainResponse>
async function getHealth(): Promise<HealthResponse>
async function clearCache(key: string): Promise<{ Status: boolean; Errors: string[] }>
```

**Configuration:**

- All requests go through `/api/*` prefix
- Development: Vite proxy routes to `http://localhost:8080`
- Production: Configure reverse proxy or serve from same domain
- All requests use `cache: 'no-store'` to prevent browser caching

#### TypeScript Interfaces (`src/lib/types.ts`)

Interfaces match backend structs exactly:

```typescript
interface IpResponse {
  IP: string;
  ValidIP: boolean;
  BlackListed: boolean;
  Status: boolean;
  BlackList: Record<string, string[]>;
  Errors: string[];
  TimeTaken: number;
  Cached: boolean;
  CacheKey: string;  // Added January 20, 2026
}

interface DomainResponse {
  Domain: string;
  ValidDomain: boolean;
  BlackListed: boolean;
  Status: boolean;
  BlackList: Record<string, string[]>;
  Errors: string[];
  TimeTaken: number;
  Cached: boolean;
  CacheKey: string;  // Added January 20, 2026
}
```

### State Management

- **Component-level state**: Uses Svelte 5 runes (`$state`, `$effect`)
- **No global state management**: Small app, props and callbacks sufficient
- **History storage**: Parent component state (max 20 items)
- **Dark mode**: localStorage persistence
- **Reactive updates**: `$effect()` for side effects (e.g., selectedItem changes)

### Development Workflow

```bash
cd frontend
npm install              # Install dependencies
npm run dev              # Start dev server (http://localhost:5173)
npm run build            # Production build
npm run preview          # Test production build
npm run check            # TypeScript type checking
npm run format           # Format code with Prettier
```

### API Proxy Configuration

**Development** (`vite.config.ts`):

```typescript
server: {
  proxy: {
    '/api': {
      target: 'http://localhost:8080',
      changeOrigin: true,
      rewrite: (path) => path.replace(/^\/api/, '')
    }
  }
}
```

**Production:**

- Configure Nginx/Apache reverse proxy
- Or serve frontend from same domain as API
- Or enable CORS on backend

### Dark Mode Support

- Tailwind `class` strategy for dark mode
- Toggle in header navigation
- Preference saved in localStorage
- All components designed for both themes
- Custom color tokens in `tailwind.config.js`

### Form Validation

**Client-side validation** using Zod schemas:

```typescript
// IP validation (4 octets, 0-255 each)
const ipSchema = z.string().min(1).refine(/* custom logic */)

// Domain validation (standard DNS format)
const domainSchema = z.string().min(1).refine(/* regex check */)

// Blacklist validation (1-20 items)
const blacklistSchema = z.array(z.string().min(1)).min(1).max(20)

// Nameserver validation (IP format, max 3)
const nameserverSchema = z.array(ipSchema).max(3).optional()
```

**Validation feedback:**

- Real-time validation on input change
- Visual indicators (border colors, icons)
- Error messages below fields
- Prevents submission if invalid

### Recent Updates (January 2026)

1. **CacheKey Integration:**
   - Updated types to include `CacheKey` field
   - Modified `ResultsCard` to use exact cache key from API
   - Fixed cache deletion for POST requests
   - Added cache key display (collapsible section)

2. **Advanced Options Support:**
   - Custom blacklists with validation
   - Custom nameservers with validation
   - Automatic POST endpoint usage when custom options provided

3. **Improved UX:**
   - Toast notifications for all actions
   - Copy-to-clipboard functionality
   - History panel for quick re-checks
   - Responsive design (mobile, tablet, desktop)

### Health Dashboard (`/health`)

Dedicated page for system monitoring:

- API status with live indicator
- Redis connectivity status
- Cached items count
- System uptime (formatted: Xh Ym Zs)
- Memory usage (MB)
- Go version and software version
- Auto-refresh every 5 seconds
- Visual status cards with icons

## Performance and Scalability

### Concurrency

- **Parallel DNS lookups**: Each blacklist is queried simultaneously via goroutines
- **Response time**: O(1) relative to the number of blacklists (instead of O(n) sequential)
- **Example**: 10 blacklists queried in ~200-300ms instead of 2-3 seconds
- **Context-aware**: DNS queries use context with configurable timeout (default 5s)

### Caching

- **High hit rate**: Frequent IPs/domains served from cache (typically <1ms vs 200-500ms)
- **Configurable TTL**: Balance between freshness and performance
- **Complete serialization**: Cache includes all response data including CacheKey
- **POST endpoint caching**: Since January 17, 2026, POST requests are also cached
- **Intelligent key generation**: Hash-based keys for POST endpoints ensure cache consistency
- **Order independence**: Blacklist array order doesn't affect cache key

### Connection Pooling

- **Redis pool**: Reuse of Redis connections (MaxActive: 64 by default, configurable)
- **DNS resolver**: Custom resolver with nameserver pool for redundancy
- **Custom resolvers**: POST endpoints can specify custom nameservers per request

### Limits and Considerations

1. **`redisMaxActive`** - Maximum simultaneous Redis connections (default 64); callers wait for a free one rather than failing
2. **Buffered Channel**: Size = number of blacklists (prevents goroutine blocking)
3. **Global State**: Package-level variables are read-only after initialization, except the atomics written by the background monitors. Handlers must never write a global
4. **Memory**: Allocations sampled by the background monitor and read via `MemUsage()`
5. **Request limits**:
   - Max 20 custom blacklists per POST request (configurable)
   - Max 3 custom nameservers per POST request (configurable)
   - Max 1MB request body size (configurable)
   - Max 253 chars for domain/IP strings (DNS standard)
6. **Timeouts**:
   - DNS query timeout: 5 seconds (configurable)
   - HTTP read timeout: 30 seconds (configurable)
   - HTTP write timeout: 30 seconds (configurable)

## Error Handling

### Multi-Layer Strategy

1. **Input Validation**: IP/domain validated before proceeding
2. **Redis Failures**: Non-blocking, service continues without cache
3. **DNS Errors**:
   - "no such host" ignored (expected behavior for non-blacklisted)
   - Other errors aggregated in `Errors[]` array
4. **Graceful Degradation**: Each component can fail without crashing

### Error Propagation

```txt
Redis Error → Added to Errors[] → Service continues
DNS Error   → Filtered if "no such host" → Aggregated in errorCh
Invalid IP  → Status: false → Immediate response
```

## Deployment

### Docker Multi-Stage Build

```dockerfile
# Stage 1: Builder
FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o multicheck

# Stage 2: Runtime
FROM alpine:latest
COPY --from=builder /app/multicheck .
CMD ["./multicheck"]
```

### Docker Compose

```yaml
services:
  multicheck:
    build: .
    depends_on:
         valkey:
            condition: service_healthy
    environment:
         - VALKEY_HOST=127.0.0.1
         - VALKEY_PORT=6379
      network_mode: host
  
   valkey:
      image: docker.io/valkey/valkey:9
      network_mode: host
      command: valkey-server /usr/local/etc/valkey/valkey.conf
      volumes:
         - ./valkey.conf:/usr/local/etc/valkey/valkey.conf:ro
      healthcheck:
         test: ["CMD", "valkey-cli", "ping"]
         interval: 5s
         timeout: 3s
         retries: 10
         start_period: 5s

   frontend:
      build:
         context: ./frontend
         dockerfile: Dockerfile
      network_mode: host
      depends_on:
         - multicheck
      profiles:
         - frontend-prod

   frontend-dev:
      image: node:24-alpine
      network_mode: host
      working_dir: /app
      depends_on:
         - multicheck
      profiles:
         - frontend-dev
      volumes:
         - ./frontend:/app
         - frontend_node_modules:/app/node_modules
      command: sh -c "if [ ! -d node_modules ]; then npm ci; fi; npm run dev -- --host 0.0.0.0 --port 5173"

volumes:
   frontend_node_modules:
```

**Networking**: Multicheck connects to Valkey via host networking to match local development settings.

## Testing

### Backend Test Coverage

The suite is split so the everyday command needs nothing but a Go toolchain.

**Unit tests** — `functions_test.go`, `handlers_test.go`, sharing the hermetic
environment built by `TestMain` in `testsupport_test.go`. No Redis server, no DNS
traffic, no `config.toml` on disk. They cover the pure logic (address reversal,
cache-key construction, sentinel filtering, input validation, configuration
defaults) and every request path rejected before DNS or Redis work begins.

**Integration tests** — `main_integration_test.go`, behind the `//go:build
integration` tag. These read the real `config.toml`, need a reachable Redis instance
and query third-party DNSBL servers. `setupTestWithResolver(t)` calls `t.Skip` when
Redis is unreachable, so they never fail for environmental reasons.

`main_integration_test.go` includes comprehensive tests for:

**Basic Functionality:**

- Health check endpoint (`TestHealthCheckHandler`)
- Redis connectivity verification
- HTTP status codes for all endpoints
- Root handler endpoint listing (`TestRootHandler`)

**Blacklist Detection:**

- IP blacklist detection with known blacklisted IP (`TestIPBlacklist`)
- Domain blacklist detection with known blacklisted domain (`TestDomainBlacklist`)
- Verification of specific DNSBL response codes (e.g., 127.0.0.2, 127.0.0.11, 127.0.0.14)
- Clean IP/domain detection (no false positives)

**POST Endpoints:**

- Custom blacklist IP checking (`TestPostCheckIP`)
- Custom blacklist domain checking (`TestPostCheckDomain`)
- Custom nameserver support

**Input Validation:**

- Invalid IP format rejection (`TestGetIpInvalid`)
- Invalid domain format rejection (`TestGetDomainInvalid`)
- Empty blacklist array rejection
- Too many blacklists rejection
- Invalid nameserver rejection
- Malformed JSON rejection
- Too-long input strings rejection

**Caching:**

- Cache hit verification (`TestGetIpCacheHit`, `TestGetDomainCacheHit`)
- Cache consistency verification (identical responses)
- Cache key format and exposure (`TestPostCheckIpCacheKey`, `TestPostCheckDomainCacheKey`)
- Cache deletion verification (`TestPostCheckIpCacheDeletionWithCacheKey`)
- Cache independence (order, nameservers)

**Test Utilities:**

- Colored output with icons (▶ running, ✓ pass, ✗ fail)
- Two modes:
  - `make test`: Verbose with full output and colors
  - `make test-quiet`: Summary only (clean output)
- `setupTestWithResolver()`: Ensures resolver is properly initialized

### Frontend Testing

**Manual Testing:**

- Real-time form validation
- API endpoint switching (GET vs POST)
- Cache key display and copy
- Cache deletion with correct keys
- Toast notifications
- Dark mode toggle
- History panel functionality

### Manual Testing Tools

Utility scripts in root directory:

- `curl.sh`: Request examples for all endpoints
- `curl-post-examples.sh`: POST endpoint examples with custom blacklists
- `health.sh`: Quick health check (continuous loop)
- `test-cache-key.sh`: Cache key integration tests
- `ips.txt` / `domains.txt`: Batch testing lists

### Test Execution

```bash
# Backend unit tests (no Redis, no network)
make test              # Verbose colored output
make test-quiet        # Summary only
make test-race         # Under the race detector

# Backend integration tests (needs Redis and live DNS)
docker compose up -d valkey
make test-integration
make test-cache-key    # Cache key integration tests (shell based)

# Frontend tests
cd frontend
npm run check          # TypeScript type checking
npm run lint           # Linting
```

## Security

### Considerations

1. **Input Validation**: All input validated before processing
2. **DNS Poisoning**: Custom resolver reduces risk
3. **Rate Limiting**: Not implemented (to consider for production)
4. **Authentication**: Redis without password in basic setup (configurable)

### Production Recommendations

- Implement rate limiting (per IP, per API key)
- Add API authentication (API keys, OAuth, JWT)
- Configure Redis password and TLS
- Use HTTPS/TLS for all connections
- Monitoring and alerting on errors
- Implement circuit breaker for slow blacklists
- Set up proper logging infrastructure (centralized)
- Configure firewall rules and network security
- Regular security audits and dependency updates
- Implement request size limits (already configured)
- Use environment-specific configurations

## Monitoring and Observability

### Structured Logs

All logs in JSON format allow:

- Parsing with `jq`
- Ingestion into centralized systems (ELK, Loki)
- Performance analysis and debugging

### Available Metrics

- Execution time per request
- Cache hit/miss ratio
- Allocated memory and GC
- Active Redis connections
- DNS and Redis errors

### Health Check Endpoint

Usable for:

- Kubernetes liveness/readiness probes
- Load balancer health checks
- External monitoring (Prometheus, Nagios, etc.)

## Possible Improvements

### Performance

- [ ] Implement circuit breaker for slow/unresponsive blacklists
- [ ] Distributed cache (Redis Cluster/Sentinel) for high availability
- [ ] Prometheus metrics endpoint for monitoring
- [ ] Request batching for multiple simultaneous queries
- [x] Connection pooling optimization (tune MaxActive/MaxIdle)
- [ ] HTTP/2 support for better performance
- [ ] Propagate `r.Context()` into the DNS lookups, so a disconnected client stops the fan-out
- [ ] Collapse concurrent lookups of the same key with `singleflight` to prevent cache stampedes
- [ ] Graceful shutdown (SIGTERM + `srv.Shutdown()`)

### Features

- [ ] API key authentication system
- [ ] Rate limiting per IP/API key with Redis
- [ ] Webhook notifications for blacklist detections
- [ ] Historical data tracking and analytics
- [ ] CSV/PDF report export
- [ ] Email/Slack notifications
- [ ] Multi-language support (i18n)
- [ ] Bulk check API (multiple IPs/domains at once)
- [ ] Whitelist management
- [ ] Custom DNSBL server support
- [ ] Scheduled checks (cron-like)

### Frontend Enhancements

- [ ] Advanced search/filter in history
- [ ] Export history to CSV/JSON
- [ ] Visualization charts (detection trends)
- [ ] Comparison mode (multiple results side-by-side)
- [ ] Saved configurations/presets
- [ ] Keyboard shortcuts
- [ ] Progressive Web App (PWA) support
- [ ] Mobile app (React Native/Flutter)

### Operations

- [ ] Graceful shutdown with connection draining
- [ ] Configuration hot reload (without restart)
- [ ] Redis backup/restore automation
- [ ] Metrics dashboard (Grafana integration)
- [ ] Distributed tracing (Jaeger/Zipkin)
- [ ] Log aggregation (ELK/Loki stack)
- [ ] Health check webhooks
- [ ] Automatic scaling based on load
- [ ] Docker Swarm/Kubernetes manifests
- [ ] CI/CD pipeline automation
- [ ] Automated security scanning

### Developer Experience

- [ ] OpenAPI/Swagger documentation
- [ ] SDK generation for popular languages
- [ ] Postman collection
- [ ] GraphQL API alternative
- [ ] WebSocket support for real-time updates
- [ ] CLI tool for command-line usage
- [ ] Browser extension
- [ ] VS Code extension

## Recent Changes

### July 26, 2026 (v1.6.0 — correctness and security fixes)

Backend:

- `applyConfigDefaults()` now covers **every** optional key. It previously defaulted only five Redis/monitor settings, so a `config.toml` missing the others left them at zero — which rejects every input (`maxStringLength`), fails every DNS lookup (`dnsQueryTimeout`), rejects every POST body (`maxRequestBodySize`), disables caching (`redisCacheTTL`) and binds `:80` (`listenPort`)
- Added `validateConfig()`: the service refuses to start with no blacklists, or with a `nameServers` entry that is not an IP
- IPv6 is rejected explicitly (`isIPv4`). `net.ParseIP` accepted it, `reverseIP` could not reverse it, and the resulting nonsensical query answered "not blacklisted"
- `/clear-cache/{key}` moved from `GET` to `DELETE`, restricted to keys this service could have written (`isOwnCacheKey`), and it now returns real status codes (`400`/`503`/`500`) instead of always `200`
- `Cache-Control` moved to the success path: it was set before the validation branches, declaring `400` responses cacheable for an hour. Removed entirely from POST responses
- Added graceful shutdown on `SIGINT`/`SIGTERM` (`srv.Shutdown` + Redis pool close). In-flight requests were previously truncated on every container stop
- DNSBL refusal codes `127.255.255.252-254` are no longer counted as listings; they become an error entry, so a rate-limited resolver no longer produces confident false positives
- Logs are single-line JSON (`json.Marshal`), and request bodies are truncated (`truncateForLog`) and omitted entirely from rejected-request logs
- Client IP resolution via `clientIPFrom()`, honouring proxy headers only when `trustProxyHeaders` is set
- `nameServers` now ships empty, selecting the system resolver, so a fresh clone works on any machine

Frontend:

- Fixed `bg-card`, which generated no CSS at all: the Tailwind v4 `@theme` block was missing the `card`/`popover` colour mappings, leaving ten cards transparent
- Fixed "clear history", which assigned to a non-bindable prop and therefore did nothing; history items now carry a UUID so two checks in the same millisecond cannot collide on the `{#each}` key
- API client rewritten around a single `request()` helper: request timeouts, backend `Errors[]` surfaced to the user, URL-encoded path parameters, and non-JSON responses handled
- Theme applied by a blocking script in `app.html`, removing the flash of light theme on every load; toasts now follow the theme
- Health dashboard keeps the last known-good data on a failed poll instead of blanking, and stops polling while the tab is hidden
- Stricter client-side IPv4 validation (`parseInt` accepted `"1x.2.3.4"`), and Zod `safeParse` in place of the deprecated `.errors` alias

Testing:

- Test suite split: unit tests (`functions_test.go`, `handlers_test.go`, `testsupport_test.go`) run with no Redis, no DNS and no `config.toml`; endpoint tests moved to `main_integration_test.go` behind the `integration` build tag and skip themselves when Redis is unreachable

### January 20, 2026

- Added `CacheKey` field to all API responses (backend + frontend)
- Fixed cache deletion for POST endpoints with custom blacklists
- Enhanced frontend ResultsCard with cache key display
- Updated TypeScript interfaces to include CacheKey

### January 17, 2026

- Implemented Redis caching for POST endpoints
- Added hash-based cache key generation for custom blacklists
- Cache independence from nameserver selection
- Order-independent blacklist caching

### January 2026 (Custom Blacklists Feature)

- Added POST endpoints for custom blacklist checking
- Implemented comprehensive input validation
- Added custom nameserver support
- Enhanced security with resource limits
- Added `HTTPMethod`, `HTTPStatusCode`, `RequestBody` to logging
- Centralized logging with `logRequest()` helper

## Version History

- **v1.6.0** (Current): Correctness and security fixes; IPv4-only checks, `DELETE /clear-cache`, graceful shutdown, complete configuration defaults, unit/integration test split
- **v1.5.0**: Data races and blocking calls removed from the request path; background monitors for Redis status and memory
- **v1.4.0**: Production images for Kubernetes, GHCR publishing workflow
- **v1.3.0**: `CacheKey` exposed in every response; cache deletion for POST endpoints
- **v1.2.0**: Redis caching for POST endpoints with hash-based, order-independent keys
- **v1.1.0**: POST endpoints for custom blacklists and nameservers, with input validation and resource limits
- **v1.0.0**: Full-featured DNSBL checker with caching
  - Backend: Go with Gorilla Mux, Redis caching, concurrent DNS lookups
  - Frontend: SvelteKit 5 with TypeScript and Tailwind CSS
  - Features: GET/POST endpoints, custom blacklists, custom nameservers, cache management

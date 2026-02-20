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
│  │  GET /clear-cache/{key}         │   │
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
- **`DelCache()`**: Endpoint `/clear-cache/{key}` to manually invalidate a cache entry

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

- Bypasses the system DNS
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

#### False Positive Filtering

The responses `127.0.0.1` and `127.255.255.255` are considered false positives and are filtered out:

```go
func removeIPFromSlice(slice []net.IP) []net.IP {
    var result []net.IP
    for _, ip := range slice {
        if !ip.Equal(net.ParseIP("127.0.0.1")) &&
           !ip.Equal(net.ParseIP("127.255.255.255")) {
            result = append(result, ip)
        }
    }
    return result
}
```

### 3. Redis Caching System (`db.go`)

#### Connection Pooling

Multicheck uses **redigo** with a connection pool to efficiently manage Redis connections:

```go
func redisConnect() *redis.Pool {
    return &redis.Pool{
        MaxIdle:   1,
        MaxActive: 8,
        Dial: func() (redis.Conn, error) {
            c, err := redis.Dial("tcp", connString)
            // ... authentication and database selection
            return c, err
        },
    }
}
```

**Pool parameters:**

- `MaxIdle: 1` - One idle connection kept open
- `MaxActive: 8` - Maximum 8 simultaneous connections

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
   - Filters false positives (127.0.0.1, 127.255.255.255)
   - Updates shared map if blacklisted (mutex-protected)
   - Sends errors on errorCh
   ↓
11. WaitGroup waits for completion of all goroutines
   ↓
12. Aggregate results (BlackListed, BlackList map)
   ↓
13. Serialize response to JSON (includes CacheKey field)
   ↓
14. Save in Redis with TTL (setRedisKey)
   ↓
15. Return JSON response to client (includes CacheKey for cache invalidation)
   ↓
16. Generate JSON log on stdout (includes HTTPMethod, RequestBody, HTTPStatusCode)
```

### Health Check

```txt
1. Client sends GET /health
   ↓
2. Performs PING on Redis
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

- **Redis pool**: Reuse of Redis connections (MaxActive: 8)
- **DNS resolver**: Custom resolver with nameserver pool for redundancy
- **Custom resolvers**: POST endpoints can specify custom nameservers per request

### Limits and Considerations

1. **MaxActive: 8** - Maximum 8 simultaneous Redis connections
2. **Buffered Channel**: Size = number of blacklists (prevents goroutine blocking)
3. **Global State**: Shared package-level variables (read-mostly, thread-safe where needed)
4. **Memory**: Allocations monitored via `MemUsage()`
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
      image: node:22-alpine
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

`main_test.go` includes comprehensive tests for:

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
# Backend tests
make test              # Verbose colored output
make test-quiet        # Summary only
make test-cache-key    # Cache key integration tests
go test -v ./...       # Direct Go test execution

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
- [ ] Connection pooling optimization (tune MaxActive/MaxIdle)
- [ ] HTTP/2 support for better performance

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

- **v1.0.0** (Current): Full-featured DNSBL checker with caching and custom blacklists
  - Backend: Go with Gorilla Mux, Redis caching, concurrent DNS lookups
  - Frontend: SvelteKit 5 with TypeScript and Tailwind CSS
  - Features: GET/POST endpoints, custom blacklists, custom nameservers, cache management

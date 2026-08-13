# Multicheck

Multicheck is a high-performance REST API service written in Go to check the reputation of domains and IP addresses against DNS blacklists (DNSBL). The service uses concurrent DNS lookups to query multiple blacklists simultaneously and implements a Redis caching system to optimize response times.

**Includes a modern SvelteKit web frontend** for easy interaction with the API through a beautiful, responsive interface.

## 📋 Key Features

### Backend API

- **IP Verification**: Checks IP addresses against configurable DNS blacklists (DNSBL)
- **Domain Verification**: Checks domains against specialized domain blacklists
- **Concurrent Lookups**: Uses goroutines for high-speed parallel DNS queries
- **Intelligent Caching**: Redis caching system for both GET and POST endpoints to reduce response times
- **Custom Blacklists**: POST endpoints allow specifying custom blacklists per request
- **Health Check**: Monitoring endpoint to verify service and Redis status
- **Configurable**: Customizable blacklists and parameters via TOML file
- **JSON Logging**: Structured logs in JSON format for easy parsing and analysis
- **Custom DNS Resolver**: Uses configurable nameservers to avoid local DNS limitations

### Web Frontend

- **Modern UI**: Built with SvelteKit, Tailwind CSS, and TypeScript
- **Real-time Validation**: Instant feedback on IP/domain input
- **Dark/Light Mode**: Automatic theme switching with localStorage persistence
- **Check History**: Keep track of recent checks with quick re-check functionality
- **Advanced Options**: Custom blacklists and nameservers configuration
- **Health Dashboard**: Real-time monitoring of API and Redis status
- **Responsive Design**: Mobile-first design that works on all devices
- **Copy to Clipboard**: Easy copying of results and JSON responses

## 🚀 Quick Start

### Prerequisites

**Backend:**

- Go 1.26+
- Redis server
- Docker (optional)

**Frontend (optional):**

- Node.js 22+
- npm

### Installation and Startup

#### Backend API Only

**With Docker Compose:**

```bash
docker compose --profile frontend-prod up --build
```

The API will be available at `http://localhost:8080`
The frontend will be available at `http://localhost:5173` when a frontend profile is enabled.

Note: Docker Compose waits for the Valkey healthcheck before starting Multicheck.
Note: The Compose frontend uses `npm run build` and `npm run preview` for startup.

**Frontend profiles:**

```bash
# Production preview (cached Dockerfile build)
docker compose --profile frontend-prod up --build

# Development server (no build, faster reloads)
docker compose --profile frontend-dev up
```

If you do not pass a profile, the frontend service will not start.

**Manual Build:**

```bash
# Build
make build

# Run
make run

# Test
make test
```

#### Frontend Web Interface

**Using Make (recommended):**

```bash
make install-frontend  # First time: install dependencies
make run-frontend      # Start development server
make build-frontend    # Build for production
```

**Or using npm directly:**

```bash
cd frontend
npm install          # First time: install dependencies
npm run dev          # Start development server
npm run build        # Build for production
```

The frontend will be available at `http://localhost:5173` and will automatically proxy API requests to the backend.

## 📡 API Endpoints

### Root - List Endpoints

```bash
GET /
```

Returns the list of all available endpoints and the current configuration.

### IP Verification

```bash
GET /ip/{ip}
```

Checks if an IP address is present in any of the configured blacklists.

Only IPv4 is accepted. The configured DNSBL zones are IPv4-only, and an IPv6
address has no valid reversal into one of them, so it is rejected with `400`
rather than answered with a meaningless "not listed".

**Example:**

```bash
curl http://localhost:8080/ip/1.2.3.4
```

**Response:**

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

### Domain Verification

```bash
GET /domain/{domain}
```

Checks if a domain is present in any of the configured blacklists.

**Example:**

```bash
curl http://localhost:8080/domain/example.com
```

**Response:**

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

### Custom Blacklist Check - IP (POST)

```bash
POST /ip/check
Content-Type: application/json
```

Checks an IP address against a custom list of blacklists, overriding the default configuration. Results are cached in Redis using a key format `post:ip:<ip>:<hash>` where the hash is generated from the sorted blacklist array.

**Request Body:**

```json
{
  "ip": "1.2.3.4",
  "blacklists": [
    "zen.spamhaus.org",
    "bl.spamcop.net",
    "cbl.abuseat.org"
  ],
  "nameservers": ["8.8.8.8", "1.1.1.1"]
}
```

**Parameters:**

- `ip` (required): IP address to check
- `blacklists` (required): Array of DNS blacklist domains to query
- `nameservers` (optional): Array of custom DNS nameservers to use (default: uses config.toml nameservers)

**Response:**

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

**Note:** The `Cached` field will be `true` when the response is served from Redis cache. The `CacheKey` field contains the Redis key used for caching - use this value with the `/clear-cache/{key}` endpoint to delete the cached entry. Requests with identical IP and blacklist array (regardless of order) will result in cache hits. The `nameservers` parameter does not affect cache keys since DNS results should be consistent across different nameservers.

**Validation Rules:**

- `ip`: Must be a valid IPv4 address. IPv6 is rejected: the configured blacklists
  are IPv4-only zones
- `blacklists`: Array of DNS blacklist domains
  - Cannot be empty
  - Maximum 20 blacklists (configurable via `maxCustomBlacklists` in config.toml)
  - Each entry must be a valid DNS name format
  - No invalid DNS characters allowed
  - Cannot start/end with `.` or `-`
  - No consecutive dots allowed
- `nameservers` (optional): Array of custom DNS nameservers
  - Must be valid IP addresses
  - Maximum 3 nameservers (configurable via `maxCustomNameservers` in config.toml)
  - If omitted, uses nameservers from config.toml

**Error Responses:**

```json
{
  "IP": "invalid",
  "ValidIP": false,
  "Status": false,
  "Errors": ["invalid IP address format"],
  "TimeTaken": 0.001,
  "Cached": false
}
```

```json
{
  "IP": "1.2.3.4",
  "ValidIP": true,
  "Status": false,
  "Errors": ["too many blacklists: maximum 20 allowed, received 25"],
  "TimeTaken": 0.001,
  "Cached": false
}
```

```json
{
  "IP": "1.2.3.4",
  "ValidIP": true,
  "Status": false,
  "Errors": ["invalid nameserver: 'notanip' is not a valid IP address"],
  "TimeTaken": 0.001,
  "Cached": false
}
```

### Custom Blacklist Check - Domain (POST)

```bash
POST /domain/check
Content-Type: application/json
```

Checks a domain against a custom list of blacklists, overriding the default configuration. Results are cached in Redis using a key format `post:domain:<domain>:<hash>` where the hash is generated from the sorted blacklist array.

**Request Body:**

```json
{
  "domain": "example.com",
  "blacklists": [
    "dbl.spamhaus.org",
    "multi.uribl.com"
  ],
  "nameservers": ["8.8.8.8", "1.1.1.1"]
}
```

**Parameters:**

- `domain` (required): Domain to check
- `blacklists` (required): Array of DNS blacklist domains to query
- `nameservers` (optional): Array of custom DNS nameservers to use (default: uses config.toml nameservers)

```txt
**Response:**

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

**Note:** The `CacheKey` field contains the Redis key used for caching - use this value with the `/clear-cache/{key}` endpoint to delete the cached entry.

**Validation Rules:**

- `domain`: Must be a valid domain name format
- `blacklists`: Same validation rules as IP check endpoint
- `nameservers`: Same validation rules as IP check endpoint

**Error Responses:**

```json
{
  "Domain": "invalid domain!",
  "ValidDomain": false,
  "Status": false,
  "Errors": ["invalid domain format"],
  "TimeTaken": 0.001,
  "Cached": false
}
```

```json
{
  "Domain": "example.com",
  "ValidDomain": true,
  "Status": false,
  "Errors": ["invalid blacklist format: 'AAAA' (must be a valid DNS name)"],
  "TimeTaken": 0.001,
  "Cached": false
}
```

### Health Check

```bash
GET /health
```

Checks the service status, Redis connectivity, and uptime.

**Response:**

```json
{
  "Alive": true,
  "Redis": true,
  "RedisConnections": 1,
  "CachedItems": 42,
  "Uptime": 3600000000000,
  "GoVersion": "go1.26.6",
  "Version": "1.6.0",
  "MemoryAlloc": 2048
}
```

`Uptime` is a `time.Duration` in nanoseconds; `MemoryAlloc` is in KB and comes from
the background memory monitor, so it may be up to one sampling interval old.

The endpoint always answers `200` and reports Redis availability in the body: the
service is designed to operate without Redis, serving every request with a live DNS
lookup.

### Clear Cache

```bash
DELETE /clear-cache/{key}
```

Removes a specific key from the Redis cache.

**Examples:**

```bash
# Clear GET endpoint cache (simple key)
curl -X DELETE http://localhost:8080/clear-cache/1.2.3.4
curl -X DELETE http://localhost:8080/clear-cache/example.com

# Clear POST endpoint cache using CacheKey from response
curl -X DELETE http://localhost:8080/clear-cache/post:ip:1.2.3.4:a3f5c8d12e9b7f6a
curl -X DELETE http://localhost:8080/clear-cache/post:domain:example.com:f7b2d9e4c1a8f563
```

**Status codes:**

| Code | Meaning |
|------|---------|
| `200` | The key was deleted |
| `400` | The key is not one this service could have written (see below) |
| `500` | Redis returned an error on the delete |
| `503` | Redis is unavailable |

**Accepted keys:** an IPv4 address, a valid domain name, or a key prefixed with
`post:ip:` / `post:domain:`. Anything else is rejected with `400`. The endpoint is
unauthenticated and deletes by exact key, so this restriction is what keeps it from
being used to delete entries belonging to other applications sharing the same Redis
database.

**Note:** All API responses include a `CacheKey` field containing the exact Redis key used for caching. Use this value directly with the `/clear-cache/{key}` endpoint to delete cached entries. For POST endpoints, the cache key includes a hash generated from the sorted blacklist array.

## ⚙️ Configuration

The configuration is located in `config.toml`:

```toml
# IP address blacklist
ipBlacklist = """
b.barracudacentral.org
bl.spamcop.net
zen.spamhaus.org
..."""

# Domain blacklist
domainBlacklist = """
multi.uribl.com
dbl.spamhaus.org
..."""

# HTTP cache (seconds)
cacheControlMaxAge = 3600

# Redis cache TTL (seconds)
redisCacheTTL = 300

# Request limits
maxCustomBlacklists = 20     # Max blacklists in a POST request
maxCustomNameservers = 3     # Max nameservers in a POST request
maxRequestBodySize = 1048576 # Max POST body in bytes
maxStringLength = 253        # Max length of a domain name (DNS standard)

# Timeouts (seconds)
dnsQueryTimeout = 5
httpReadTimeout = 30
httpWriteTimeout = 30
httpIdleTimeout = 60
httpReadHeaderTimeout = 10

# DNS nameservers to use. Empty uses the system resolver (/etc/resolv.conf).
nameServers = ""

# Trust X-Forwarded-For / X-Real-IP for the logged client IP.
# Enable ONLY behind a reverse proxy that overwrites those headers.
trustProxyHeaders = false

# Redis configuration
redisHost = "127.0.0.1"
redisPort = 6379
redisDatabase = 0
redisPassword = "" # Leave empty if no password is required

# Redis connection pool
redisMaxIdle = 8      # Idle connections kept in the pool
redisMaxActive = 64   # Upper bound on concurrent Redis connections
redisConnTimeout = 2  # Connect/read/write timeout for Redis in seconds

# Background monitors
redisHealthCheckInterval = 5 # Redis availability probe interval in seconds
memStatsInterval = 10        # Memory sampling interval in seconds

# Listen port
listenPort = ":8080"
```

### Defaults

Every parameter except the blacklists is optional. A missing key falls back to the
value below, so a `config.toml` written before a key existed keeps working after an
upgrade:

| Parameter | Default | Parameter | Default |
|-----------|---------|-----------|---------|
| `cacheControlMaxAge` | `3600` | `httpReadTimeout` | `30` |
| `redisCacheTTL` | `300` | `httpWriteTimeout` | `30` |
| `maxCustomBlacklists` | `20` | `httpIdleTimeout` | `60` |
| `maxCustomNameservers` | `3` | `httpReadHeaderTimeout` | `10` |
| `maxRequestBodySize` | `1048576` | `redisHost` | `127.0.0.1` |
| `maxStringLength` | `253` | `redisPort` | `6379` |
| `dnsQueryTimeout` | `5` | `redisMaxIdle` | `8` |
| `listenPort` | `:8080` | `redisMaxActive` | `64` |
| `trustProxyHeaders` | `false` | `redisConnTimeout` | `2` |
| `redisHealthCheckInterval` | `5` | `memStatsInterval` | `10` |

**Startup validation:** the service refuses to start when both blacklists are empty
or when `nameServers` contains something that is not an IP address. Failing loudly
at startup is preferable to serving results from a resolver that cannot answer.

**Nameservers:** leaving `nameServers` empty selects the system resolver, which is
what makes the shipped configuration work on any machine (and, in Kubernetes,
resolves through the cluster DNS). Prefer a dedicated resolver in production: DNSBL
providers rate-limit and eventually block queries coming from large public
resolvers, and a blocked resolver answers with a refusal code rather than a real
result.

**Background monitors:** Redis availability and memory usage are sampled on a
timer instead of per request. This keeps a Redis `PING` round-trip and
`runtime.ReadMemStats` (which stops the world) off the request path. The
`Redis`, `RedisConnections` and `MemoryAlloc` fields in the logs can therefore be
up to one interval old; the `/health` endpoint always pings live.

**Running without Redis:** Redis is a cache, not a dependency. If it is unreachable
at startup the service logs a warning and continues; if it becomes unreachable
later, cache reads and writes are skipped, `Cached` stays `false` and a descriptive
entry is added to `Errors[]`. Every request then performs a live DNS lookup, which
is slower but correct.

### Environment Variables

You can also specify a custom path for the configuration:

- `GSS_CONFIG_PATH`: path to the directory containing `config.toml`

## 🏗️ Architecture

For a detailed description of the project architecture, see [ARCHITECTURE.md](ARCHITECTURE.md).

## 📊 Logging

Multicheck generates structured logs in JSON format on stdout. Each request is logged with:

- Timestamp
- HTTP Method (GET, POST, PUT, etc.)
- Method/Endpoint
- Requested parameter
- Request body (for POST requests)
- Allocated memory and garbage collection
- Execution time
- Cache status (hit/miss)
- Client IP
- Redis status and active connections
- Any errors

**Log example:**

```json
{
   "CurrentTime": "2026-01-12T10:30:45Z",
   "HTTPMethod": "GET",
   "Method": "/ip",
   "Param": "1.2.3.4",
   "RequestBody": "",
   "Errors": [],
   "MemoryAlloc": 2048,
   "NumGC": 5,
   "TimeTaken": 0.245,
   "Cached": false,
   "ClientIP": "192.168.1.100:54321",
   "Redis": true,
   "RedisConnections": 1
}
```

**Log example for POST requests:**

```json
{
   "CurrentTime": "2026-01-15T22:45:30Z",
   "HTTPMethod": "POST",
   "Method": "/ip/check",
   "Param": "1.2.3.4",
   "RequestBody": "{\"ip\":\"1.2.3.4\",\"blacklists\":[\"zen.spamhaus.org\",\"bl.spamcop.net\"]}",
   "Errors": [],
   "MemoryAlloc": 2560,
   "NumGC": 7,
   "TimeTaken": 0.312,
   "Cached": false,
   "ClientIP": "192.168.1.100:54322",
   "Redis": true,
   "RedisConnections": 1
}
```

You can use `jq` to analyze the logs:

```bash
make run | jq
```

## 🧪 Testing

The suite is split in two, so the everyday command needs nothing but a Go toolchain.

### Unit tests

```bash
make test        # verbose, with colors and icons
make test-quiet  # summary only
make test-race   # under the race detector
```

These run against a hermetic environment (`testsupport_test.go`): no Redis server,
no DNS traffic, no `config.toml` on disk. They cover the pure logic — address
reversal, cache-key construction, DNSBL sentinel filtering, input validation,
configuration defaults — and every request path that is rejected before any DNS or
Redis work happens. Safe to run anywhere, including CI.

Output formatting:

- 🔵 **▶** Running test indicator
- ✅ **✓** Green checkmark for passed tests
- ❌ **✗** Red cross for failed tests
- Color-coded summary (green for PASS, red for FAIL)

### Integration tests

```bash
make test-integration
```

Guarded by the `integration` build tag. They need a reachable Redis instance and
live DNS access, and they assert on sentinel codes that third-party DNSBL servers
control. Individual tests skip themselves — rather than failing — when Redis is
unavailable, so the tag is the only thing standing between you and a full run:

```bash
docker compose up -d valkey
make test-integration
```

## 🔄 Continuous Integration

`.github/workflows/docker-publish.yml` runs on pushes to `master` and on `v*` tags:

1. **`verify-backend`** — `gofmt` check, `go vet` (with and without the `integration` tag), and `go test -race`
2. **`verify-frontend`** — `npm ci`, `npm run check`, `npm run lint`, `npm run build`
3. **`build`** — builds and pushes the backend and frontend images to GHCR, and runs **only if both verification jobs pass**

The two verification jobs run in parallel. The integration tests are not part of CI:
they need a Redis instance and live DNS access to third-party DNSBL servers, whose
answers are outside the project's control.

## 🐳 Docker Deployment

### Building the image

```bash
docker build -t multicheck .
```

### Run with Docker Compose

```bash
docker compose up -d
```

The `docker-compose.yml` file starts Multicheck alongside [Valkey](https://valkey.io)
(a Redis-compatible cache). All services use `network_mode: host`, so the backend
reaches the cache at `127.0.0.1:6379`.

## 📝 Utility Files

- `Makefile`: Build automation with commands for backend, frontend, and Docker
- `curl.sh`: Script with curl request examples
- `health.sh`: Script for quick health check
- `ips.txt` / `domains.txt`: Example files with lists of IPs/domains to test

## 🖥️ Frontend Directory Structure

The `frontend/` directory contains a complete SvelteKit application:

```txt
frontend/
├── src/
│   ├── lib/
│   │   ├── api.ts                 # API client functions
│   │   ├── types.ts               # TypeScript interfaces
│   │   ├── validators.ts          # Zod validation schemas
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
├── package.json                   # Dependencies and scripts
├── vite.config.ts                 # Vite config with API proxy and Tailwind plugin
└── README.md                      # Frontend-specific documentation
```

**Key Frontend Commands:**

**Using Make:**

```bash
make install-frontend  # Install dependencies (~2-3 minutes first time)
make run-frontend      # Development server with hot-reload
make build-frontend    # Production build
```

**Using npm directly:**

```bash
cd frontend
npm install          # Install dependencies (~2-3 minutes first time)
npm run dev          # Development server with hot-reload
npm run build        # Production build
npm run preview      # Test production build
npm run check        # TypeScript type checking
npm run format       # Format code with Prettier
npm run lint         # Prettier check + ESLint
```

**Frontend Features:**

- API proxy configured in `vite.config.ts` (`/api/*` → `http://localhost:8080`)
- Dark/light mode toggle with localStorage persistence
- Real-time form validation using Zod schemas
- Toast notifications for user feedback (svelte-sonner)
- Responsive design with Tailwind CSS utility classes
- Recent checks stored in component state (last 20 items)
- Health dashboard with auto-refresh every 5 seconds

## 🤝 Contributing

Contributions, issues, and feature requests are welcome!

## Support the project ❤️

If this project is useful to you, you can buy me a coffee ☕  
👉 <https://paypal.me/IgorColombi>

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

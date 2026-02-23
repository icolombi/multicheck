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

- `ip`: Must be a valid IP address format
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
  "Uptime": 3600000000000,
  "GoVersion": "go1.25.5"
}
```

### Clear Cache

```bash
GET /clear-cache/{key}
```

Removes a specific key from the Redis cache.

**Examples:**

```bash
# Clear GET endpoint cache (simple key)
curl http://localhost:8080/clear-cache/1.2.3.4
curl http://localhost:8080/clear-cache/example.com

# Clear POST endpoint cache using CacheKey from response
curl http://localhost:8080/clear-cache/post:ip:1.2.3.4:a3f5c8d12e9b7f6a
curl http://localhost:8080/clear-cache/post:domain:example.com:f7b2d9e4c1a8f563
```

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

# Maximum custom blacklists allowed in POST requests
maxCustomBlacklists = 20

# DNS nameservers to use
nameServers = """
8.8.4.4
8.8.8.8
"""

# Redis configuration
redisHost = "127.0.0.1"
redisPort = 6379
redisDatabase = 0
redisPassword = "" # Leave empty if no password is required

# Listen port
listenPort = ":8080"
```

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

Multicheck provides two test output formats:

### Verbose output with colors and icons

```bash
make test
```

This command runs all tests with detailed output including:

- 🔵 **▶** Running test indicator
- ✅ **✓** Green checkmark for passed tests
- ❌ **✗** Red cross for failed tests
- Color-coded summary (green for PASS, red for FAIL)
- Full JSON logs for debugging

### Minimal output (summary only)

```bash
make test-quiet
```

This command shows only test names and results without verbose JSON logs, perfect for quick checks.

Tests verify the correct functioning of endpoints and connectivity with Redis.

## 🐳 Docker Deployment

### Building the image

```bash
docker build -t multicheck .
```

### Run with Docker Compose

```bash
docker-compose up -d
```

The `docker-compose.yml` file automatically configures Multicheck and Redis with internal networking.

## 🔧 Using with Podman

The `podman-compose.sh` script is also available for use with Podman:

```bash
./podman-compose.sh
```

## 📝 Utility Files

- `Makefile`: Build automation with commands for backend, frontend, and Docker/Podman
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

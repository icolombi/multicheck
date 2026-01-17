# Multicheck

Multicheck is a high-performance REST API service written in Go to check the reputation of domains and IP addresses against DNS blacklists (DNSBL). The service uses concurrent DNS lookups to query multiple blacklists simultaneously and implements a Redis caching system to optimize response times.

## 📋 Key Features

- **IP Verification**: Checks IP addresses against configurable DNS blacklists (DNSBL)
- **Domain Verification**: Checks domains against specialized domain blacklists
- **Concurrent Lookups**: Uses goroutines for high-speed parallel DNS queries
- **Intelligent Caching**: Redis caching system to reduce response times
- **Health Check**: Monitoring endpoint to verify service and Redis status
- **Configurable**: Customizable blacklists and parameters via TOML file
- **JSON Logging**: Structured logs in JSON format for easy parsing and analysis
- **Custom DNS Resolver**: Uses configurable nameservers to avoid local DNS limitations

## 🚀 Quick Start

### Prerequisites

- Go 1.25+
- Redis server
- Docker (optional)

### Installation and Startup

#### With Docker Compose

```bash
docker-compose up
```

The service will be available at `http://localhost:8080`

#### Manual Build

```bash
# Build
make build

# Run
make run

# Test
make test
```

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
  "Cached": false
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
  "Cached": false
}
```

### Custom Blacklist Check - IP (POST)

```bash
POST /ip/check
Content-Type: application/json
```

Checks an IP address against a custom list of blacklists, overriding the default configuration.

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
  "Cached": false
}
```

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

```json
,
  "nameservers": ["8.8.8.8", "1.1.1.1"]
}
```

**Parameters:**

- `domain` (required): Domain to check
- `blacklists` (required): Array of DNS blacklist domains to query
- `nameservers` (optional): Array of custom DNS nameservers to use (default: uses config.toml nameservers)
**Request Body:**

```json
{
  "domain": "example.com",
  "blacklists": [
    "dbl.spamhaus.org",
    "multi.uribl.com"
  ]
}
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
  "TimeTaken": 0.245,
  "Cached": false
- `nameservers`: Same validation rules as IP check endpoint
}
```

**Validation Rules:**

- `domain`: Must be a valid domain name format
- `blacklists`: Same validation rules as IP check endpoint

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

Removes a specific key from the Redis cache (key = IP or domain).

**Example:**

```bash
curl http://localhost:8080/clear-cache/1.2.3.4
```

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

# Listen port
listenPort = ":8080"
```

### Environment Variables

Configure Redis through variables in `credentialstore/credentialstore.go`:

- `REDIS_HOST`: Redis server hostname (default: `localhost`)
- `REDIS_PORT`: Redis port (default: `6379`)

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

```bash
make test
```

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

- `curl.sh`: Script with curl request examples
- `health.sh`: Script for quick health check
- `ips.txt` / `domains.txt`: Example files with lists of IPs/domains to test

## 🤝 Contributing

Contributions, issues, and feature requests are welcome!

## Support the project ❤️

If this project is useful to you, you can buy me a coffee ☕  
👉 <https://paypal.me/IgorColombi>

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

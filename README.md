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

- Go 1.16+
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
  "Uptime": 3600000000000
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
- Method/Endpoint
- Requested parameter
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
   "Method": "/ip",
   "Param": "1.2.3.4",
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
👉 https://paypal.me/IgorColombi

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

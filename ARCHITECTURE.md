# Multicheck Architecture

## Overview

Multicheck is a REST API service developed in Go that implements a reputation verification system for domains and IP addresses through DNS blacklist (DNSBL) queries. The architecture is designed to maximize performance through concurrent operations and an intelligent caching system.

## Diagramma dell'Architettura

```
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

The POST endpoints `/ip/check` and `/domain/check` allow clients to override the default blacklist configuration by providing a custom list of DNS blacklists. This feature includes:

**Security and Validation:**
- **Input validation**: Strict validation of IP/domain format and blacklist DNS syntax
- **Resource protection**: Configurable maximum limit (default 20) to prevent resource exhaustion
- **DNS format checking**: Validates blacklist names against DNS naming rules:
  - Must contain at least one dot
  - Only alphanumeric, dots, and hyphens allowed
  - Cannot start/end with dot or hyphen
  - No consecutive dots
  - No empty entries or whitespace-only strings

**Request Structure:**
```json
{
  "ip": "1.2.3.4",
  "blacklists": [
    "zen.spamhaus.org",
    "bl.spamcop.net",
    "cbl.abuseat.org"
  ]
}
```

**Error Responses:**
- `400 Bad Request` for invalid input (IP/domain format, blacklist syntax, limit exceeded)
- `Status: false` with descriptive error messages in response JSON

**Implementation:**
- Uses `json.Decoder` with `DisallowUnknownFields()` to reject malformed requests
- `validateBlacklists()` function performs comprehensive validation
- `checkBlacklistIPWithCustomList()` and `checkBlacklistDomainWithCustomList()` separate functions for custom list checking
- No caching for custom blacklist requests (always fresh results)

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
}
```

Similar structure for `Domain`, `Health`, and `ClearCache`.

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

```
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
            // ... gestione autenticazione e selezione database
            return c, err
        },
    }
}
```

**Pool parameters:**

- `MaxIdle: 1` - One idle connection kept open
- `MaxActive: 8` - Maximum 8 simultaneous connections

#### Caching Strategy

1. **Cache Key**: IP or domain in string format (as-is)
2. **Cache Value**: Entire serialized JSON response structure
3. **TTL**: Configurable via `redisCacheTTL` (default: 300 seconds)

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

cacheControlMaxAge = 3600
redisCacheTTL = 300

nameServers = """
8.8.4.4
8.8.8.8"""

listenPort = ":8080"
```

**Note**: Multiline strings in TOML are parsed as space-separated lists.

#### Environment Variables

The configuration path can be customized:

```bash
export GSS_CONFIG_PATH=/custom/path
```

Redis credentials are managed in `credentialstore/credentialstore.go`:

```go
var RedisHost = getEnv("REDIS_HOST", "localhost")
var RedisPort = getEnvAsInt("REDIS_PORT", 6379)
var RedisDatabase = 0
```

### 5. Logging System

Each request generates a structured log in JSON format on `stdout`:

```go
type Log struct {
    CurrentTime      time.Time
    Method           string
    Param            string
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

- Memory usage (`MemoryAlloc`)
- Garbage collection count (`NumGC`)
- Execution time (`TimeTaken`)
- Cache status (`Cached`)
- Redis status (`Redis`, `RedisConnections`)

## Execution Flow

### IP/Domain Verification

```
1. Client sends GET request /ip/{ip} or /domain/{domain}
   ↓
2. Handler validates input (net.ParseIP or validator.IsValidDomain)
   ↓
3. If invalid → Immediate response with Status: false
   ↓
4. Check Redis cache (getRedisKey)
   ↓
5a. CACHE HIT → Deserialize JSON and return (Cached: true)
   ↓
5b. CACHE MISS → Proceed to step 6
   ↓
6. Launch goroutine for each configured blacklist
   ↓
7. Each goroutine:
   - Performs DNS lookup via custom resolver
   - Filters false positives (127.0.0.1, 127.255.255.255)
   - Updates shared map if blacklisted
   - Sends errors on errorCh
   ↓
8. WaitGroup waits for completion of all goroutines
   ↓
9. Aggregate results (BlackListed, BlackList map)
   ↓
10. Serialize response to JSON
    ↓
11. Save in Redis with TTL (setRedisKey)
    ↓
12. Return JSON response to client
    ↓
13. Generate JSON log on stdout
```

### Health Check

```
1. Client sends GET /health
   ↓
2. Performs PING on Redis
   ↓
3. Gets active connection count
   ↓
4. Calculates uptime (time.Since(startTime))
   ↓
5. Returns JSON with status
```

## Performance and Scalability

### Concurrency

- **Parallel DNS lookups**: Each blacklist is queried simultaneously
- **Response time**: O(1) relative to the number of blacklists (instead of O(n) sequential)
- **Example**: 10 blacklists queried in ~200-300ms instead of 2-3 seconds

### Caching

- **High hit rate**: Frequent IPs/domains served from cache
- **Configurable TTL**: Balance between freshness and performance
- **Complete serialization**: Cache includes all response data

### Connection Pooling

- **Redis pool**: Reuse of Redis connections
- **DNS resolver**: Custom resolver with nameserver pool

### Limits and Considerations

1. **MaxActive: 8** - Maximum 8 simultaneous Redis connections
2. **Buffered Channel**: Size = number of blacklists
3. **Global State**: Shared package-level variables (read-mostly)
4. **Memory**: Allocations monitored via `MemUsage()`

## Error Handling

### Multi-Layer Strategy

1. **Input Validation**: IP/domain validated before proceeding
2. **Redis Failures**: Non-blocking, service continues without cache
3. **DNS Errors**:
   - "no such host" ignored (expected behavior for non-blacklisted)
   - Other errors aggregated in `Errors[]` array
4. **Graceful Degradation**: Each component can fail without crashing

### Error Propagation

```
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
    ports:
      - "8080:8080"
    depends_on:
      - redis
    environment:
      - REDIS_HOST=redis
  
  redis:
    image: redis:alpine
```

**Networking**: Multicheck and Redis communicate via internal Docker network.

## Testing

### Test Coverage

`main_test.go` includes tests for:

- Health check endpoint
- Redis connectivity
- HTTP status code

### Manual Testing

Utility scripts:

- `curl.sh`: Request examples
- `health.sh`: Quick health check
- `ips.txt` / `domains.txt`: Batch testing

## Security

### Considerations

1. **Input Validation**: All input validated before processing
2. **DNS Poisoning**: Custom resolver reduces risk
3. **Rate Limiting**: Not implemented (to consider for production)
4. **Authentication**: Redis without password in basic setup (configurable)

### Production Recommendations

- Implement rate limiting
- Add API authentication
- Configure Redis password
- Use HTTPS/TLS
- Monitoring and alerting on errors

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

- [ ] Implement circuit breaker for slow blacklists
- [ ] Distributed cache (Redis Cluster)
- [ ] Prometheus metrics endpoint
- [ ] Request batching for multiple queries

### Features

- [ ] API key authentication
- [ ] Rate limiting per IP
- [ ] Webhook for notifications
- [ ] Web dashboard for visualization
- [ ] CSV/PDF report export

### Operations

- [ ] Graceful shutdown
- [ ] Configuration hot reload
- [ ] Database backup/restore
- [ ] Metrics dashboard (Grafana)
